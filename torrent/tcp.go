// Implementation of the TCP peer protocol described in
// https://bittorrent.org/beps/bep_0003.html#peer-messages

package torrent

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
)

// A TCPClient represents a peer connection over TCP.
type TCPClient struct {
	BitField   BitField
	Choked     bool
	Connection net.Conn
	InfoHash   string
	Peer       TrackerPeer
	PeerId     string
	Pieces     int
}

// NewTCPClient creates a TCP connection with 'peer' and performs a handshake with
// the provided peer ID ('peerID') and info hash ('infoHash'). It also takes a 'pieces'
// argument for validating the bit field.
//
// Returns the created TCPClient and an error if any occurred during this process.
func NewTCPClient(infoHash string, peer TrackerPeer, peerId string, pieces int) (*TCPClient, error) {
	conn, err := net.Dial("tcp", peer.String())
	if err != nil {
		return nil, err
	}

	// Send our handshake message to the connection
	handshake := Handshake{
		Protocol: "BitTorrent protocol",
		Reserved: []byte{0, 0, 0, 0, 0, 0, 0, 0},
		InfoHash: infoHash,
		PeerId:   peerId,
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

	recvInfoHash, err := ReadN(20, conn)
	if err != nil {
		return nil, fmt.Errorf("could not read info hash: %w", err)
	}

	if !bytes.Equal(recvInfoHash, []byte(infoHash)) {
		return nil, fmt.Errorf("ending due to info hash mismatch")
	}

	recvPeerId, err := ReadN(20, conn)
	if err != nil {
		return nil, fmt.Errorf("could not read peer id: %w", err)
	}

	if len(peer.PeerId) > 0 && !bytes.Equal(recvPeerId, []byte(peer.PeerId)) {
		return nil, fmt.Errorf("ending due to tracker peer id mismatch")
	}

	return &TCPClient{
		PeerId:     peerId,
		InfoHash:   infoHash,
		Connection: conn,
		Choked:     true, // A connection starts choked and not interested by default.
		Peer:       peer,
		Pieces:     pieces,
	}, nil
}

// ReadMessage waits for a message from the peer connection and returns the
// received message or an error if any.
func (c *TCPClient) ReadMessage() (*Message, error) {
	prefixBytes, err := ReadN(4, c.Connection)
	if err != nil {
		return nil, err
	}

	lengthPrefix := binary.BigEndian.Uint32(prefixBytes)
	if lengthPrefix == 0 {
		return &Message{KeepAlive: true}, nil
	}

	messageBytes, err := ReadN(int(lengthPrefix), c.Connection)
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
		return &Message{
			Id: msgId,
			BitField: BitField{
				Field:  msgSlice,
				Length: c.Pieces,
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

// SendMessage sends a 'message' to the peer connection and returns an error if any.
func (c *TCPClient) SendMessage(message Message) error {
	if message.KeepAlive {
		// A keep alive message is simply 4 zeroes.
		_, err := c.Connection.Write([]byte{0, 0, 0, 0})
		if err != nil {
			return fmt.Errorf("could not send keep alive: %w", err)
		}

		return nil
	}

	switch message.Id {
	case MessageChoke, MessageUnchoke, MessageInterested, MessageNotInterested:
		buf := binary.BigEndian.AppendUint32([]byte{}, 1) // length prefix
		buf = append(buf, byte(message.Id))

		c.Connection.Write(buf)
	case MessageRequest:
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.BigEndian, byte(message.Id))
		binary.Write(buf, binary.BigEndian, message.Request.Index)
		binary.Write(buf, binary.BigEndian, message.Request.Begin)
		binary.Write(buf, binary.BigEndian, message.Request.Length)

		msgSent := buf.Bytes()

		lengthPrefix := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthPrefix, uint32(len(msgSent)))

		_, err := c.Connection.Write(append(lengthPrefix, msgSent...))
		if err != nil {
			return fmt.Errorf("could not send request message: %w", err)
		}
	case MessageHave:
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.BigEndian, byte(message.Id))
		binary.Write(buf, binary.BigEndian, message.PieceIndex)

		msgSent := buf.Bytes()

		lengthPrefix := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthPrefix, uint32(len(msgSent)))

		_, err := c.Connection.Write(append(lengthPrefix, msgSent...))
		if err != nil {
			return fmt.Errorf("could not send have message: %w", err)
		}
	default:
		return fmt.Errorf("no handler for message %v", message)
	}

	return nil
}
