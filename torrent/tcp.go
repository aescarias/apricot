// Implementation of the TCP peer protocol described in
// https://bittorrent.org/beps/bep_0003.html#peer-messages

package torrent

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
)

type MessageId int

const (
	MessageChoke MessageId = iota
	MessageUnchoke
	MessageInterested
	MessageNotInterested
	MessageHave
	MessageBitfield
	MessageRequest
	MessagePiece
	MessageCancel
)

type TCPClient struct {
	PeerId      string
	Connections map[TrackerPeer]net.Conn
	MetaInfo    Torrent
}

func NewTCPClient(peerId string, metaInfo Torrent) TCPClient {
	return TCPClient{PeerId: peerId, MetaInfo: metaInfo, Connections: map[TrackerPeer]net.Conn{}}
}

type Message struct {
	// The message ID.
	Id MessageId

	// Whether this is a keep alive message.
	//
	// If this is true, all other fields must be ignored.
	KeepAlive bool

	// Whether this is a "generic" message, i.e., one that is not particularly handled.
	//
	// If this is true, the Contents field must be specified. All other fields are ignored.
	Generic bool
	// If the Generic field is true, the contents of the message.
	Contents []byte

	// If message ID is have (4), the index of the piece the tracker has.
	PieceIndex uint32
	// If message ID is bitfield (5), the returned bitfield representing each piece index.
	//
	// For each bit up to N pieces, 1 means that the tracker has the piece and 0 means
	// otherwise. All bits after the N pieces must be zero.
	BitField BitField
	// If message ID is request (6) or cancel (7), the request details.
	Request Request
	// If message ID is piece (7), the contents of the piece.
	Block Block
}

type BitField struct {
	Field  []byte
	Length int
}

// HasPiece reports whether the piece at 'index' is contained in the bit field.
func (bf *BitField) HasPiece(index int) bool {
	if index >= bf.Length {
		return false
	}

	pieceByte := int(bf.Field[index/8])
	offset := index % 8
	return pieceByte&(1<<7-offset) != 0
}

type Request struct {
	Index  uint32 // The zero-based piece index.
	Begin  uint32 // The zero-based byte offset within the piece.
	Length uint32 // The byte length of the piece.
}

type Block struct {
	Index uint32 // The zero-based piece index.
	Begin uint32 // The zero-based byte offset within the piece.
	Block []byte // A block of data representing a subset of the piece.
}

type Handshake struct {
	Protocol string  // The handshake protocol, usually "BitTorrent protocol"
	Reserved [8]byte // Reserved bytes. Used by extensions, otherwise zeroed.
	InfoHash string  // The 20-byte info hash
	PeerId   string  // The 20-char peer ID
}

func (h *Handshake) Serialized() []byte {
	message := []byte{}
	message = append(message, byte(len(h.Protocol)))
	message = append(message, []byte(h.Protocol)...)
	message = append(message, []byte(h.InfoHash)...)
	message = append(message, []byte(h.PeerId)...)

	return message
}

/*
Handshake starts a connection with 'peer' by sending a handshake message.

It returns a net.Conn instance and an error if any. The connection returned
must be closed by the caller creating the TCP client.
*/
func (c *TCPClient) Handshake(peer TrackerPeer) (net.Conn, error) {
	address := net.JoinHostPort(peer.Ip, fmt.Sprint(peer.Port))
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	infoHash, err := c.MetaInfo.Info.Hash()
	if err != nil {
		return nil, err
	}

	// Send our handshake message to the connection
	handshake := Handshake{
		Protocol: "BitTorrent protocol",
		Reserved: [8]byte{0, 0, 0, 0, 0, 0, 0, 0},
		InfoHash: string(infoHash),
		PeerId:   c.PeerId,
	}

	_, err = conn.Write(handshake.Serialized())
	if err != nil {
		return nil, fmt.Errorf("could not send handshake message: %w", err)
	}

	// Process and validate the handshake sent by the tracker.
	pStrLen, err := ReadN(1, conn)
	if err != nil {
		return nil, fmt.Errorf("could not read peer handshake: %w", err)
	}

	if _, err := ReadN(int(pStrLen[0]), conn); err != nil {
		return nil, fmt.Errorf("could not read peer handshake protocol: %w", err)
	}

	if _, err := ReadN(8, conn); err != nil {
		return nil, fmt.Errorf("could not read reserved bytes: %w", err)
	}

	sentInfoHash, err := ReadN(20, conn)
	if err != nil {
		return nil, fmt.Errorf("could not read info hash: %w", err)
	}

	if !bytes.Equal(sentInfoHash, infoHash) {
		return nil, fmt.Errorf("ending due to info hash mismatch")
	}

	peerId, err := ReadN(20, conn)
	if err != nil {
		return nil, fmt.Errorf("could not read peer id: %w", err)
	}

	if len(peer.PeerId) > 0 && !bytes.Equal(peerId, []byte(peer.PeerId)) {
		return nil, fmt.Errorf("ending due to tracker peer id mismatch")
	}

	c.Connections[peer] = conn
	return conn, nil
}

/*
ReadMessage waits for a message from the 'peer' connection and returns the
received message and the first error if any.
*/
func (c *TCPClient) ReadMessage(peer TrackerPeer) (*Message, error) {
	conn, ok := c.Connections[peer]
	if !ok {
		return nil, fmt.Errorf("peer does not have an active connection")
	}

	prefixBytes, err := ReadN(4, conn)
	if err != nil {
		return nil, err
	}

	lengthPrefix := binary.BigEndian.Uint32(prefixBytes)
	if lengthPrefix == 0 {
		return &Message{KeepAlive: true}, nil
	}

	messageBytes, err := ReadN(int(lengthPrefix), conn)
	if err != nil {
		return nil, fmt.Errorf("could not read message: %w", err)
	}

	msgId := MessageId(messageBytes[0])
	msgSlice := messageBytes[1:]

	switch msgId {
	case MessageChoke, MessageUnchoke, MessageInterested, MessageNotInterested:
		return &Message{Id: msgId}, nil
	case MessageHave:
		return &Message{Id: msgId, PieceIndex: binary.BigEndian.Uint32(msgSlice)}, nil
	case MessageBitfield:
		pieces := c.MetaInfo.Info.PieceHashes()

		return &Message{
			Id: msgId,
			BitField: BitField{
				Field:  msgSlice,
				Length: len(pieces),
			},
		}, nil
	case MessageRequest, MessageCancel:
		index := binary.BigEndian.Uint32(msgSlice[0:4])
		begin := binary.BigEndian.Uint32(msgSlice[4:8])
		length := binary.BigEndian.Uint32(msgSlice[8:12])

		return &Message{
			Id:      msgId,
			Request: Request{Index: index, Begin: begin, Length: length},
		}, nil
	case MessagePiece:
		index := binary.BigEndian.Uint32(msgSlice[0:4])
		begin := binary.BigEndian.Uint32(msgSlice[4:8])
		block := msgSlice[8:]

		return &Message{
			Id:    msgId,
			Block: Block{Index: index, Begin: begin, Block: block},
		}, nil
	default:
		return &Message{Generic: true, Contents: msgSlice, Id: msgId}, nil
	}
}

/* SendMessage sends a 'message' to the 'peer' connection and returns an error if any. */
func (c *TCPClient) SendMessage(peer TrackerPeer, message Message) error {
	conn, ok := c.Connections[peer]
	if !ok {
		return fmt.Errorf("peer does not have an active connection")
	}

	if message.KeepAlive {
		// A keep alive message is simply 4 zeroes.
		_, err := conn.Write([]byte{0, 0, 0, 0})
		if err != nil {
			return fmt.Errorf("could not send keep alive: %w", err)
		}

		return nil
	}

	switch message.Id {
	case MessageChoke, MessageUnchoke, MessageInterested, MessageNotInterested:
		buf := binary.BigEndian.AppendUint32([]byte{}, 1) // length prefix
		buf = append(buf, byte(message.Id))

		conn.Write(buf)
	case MessageRequest:
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.BigEndian, byte(message.Id))
		binary.Write(buf, binary.BigEndian, message.Request.Index)
		binary.Write(buf, binary.BigEndian, message.Request.Begin)
		binary.Write(buf, binary.BigEndian, message.Request.Length)

		msgSent := buf.Bytes()

		lengthPrefix := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthPrefix, uint32(len(msgSent)))

		_, err := conn.Write(append(lengthPrefix, msgSent...))
		if err != nil {
			return fmt.Errorf("could not send request message: %w", err)
		}
	default:
		return fmt.Errorf("no handler for message %v", message)
	}

	return nil
}
