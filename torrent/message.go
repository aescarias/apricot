package torrent

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

// A Message represents a peer message sent over the BitTorrent protocol.
type Message struct {
	// The message ID.
	Id MessageId

	// Whether this is a keep alive message.
	//
	// If this is true, all other fields must be ignored.
	KeepAlive bool

	// Whether this is a "generic" message, i.e., one that is not particularly handled.
	// This is here mainly for usage by extensions in case the client decides to support them.
	//
	// If this is true, the Contents field must also be specified. All other fields are ignored.
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

// A BitField represents the contents of a bitfield (5) peer message.
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

func (bf *BitField) SetPiece(index int) {
	if index >= bf.Length {
		return
	}

	offset := index % 8
	bf.Field[index/8] |= 1 << (7 - offset)
}

// A Request represents the contents of a request (6) and cancel (8) message.
type Request struct {
	Index  uint32 // The zero-based piece index.
	Begin  uint32 // The zero-based byte offset within the piece.
	Length uint32 // The byte length of the piece.
}

// A Block represents the contents of a piece (7) message.
type Block struct {
	Index uint32 // The zero-based piece index.
	Begin uint32 // The zero-based byte offset within the piece.
	Block []byte // A block of data representing a subset of the piece.
}

// A Handshake represents a peer handshake.
type Handshake struct {
	Protocol string // The handshake protocol, usually "BitTorrent protocol"
	Reserved []byte // Reserved bytes. Used by extensions, otherwise zeroed.
	InfoHash string // The 20-byte info hash
	PeerId   string // The 20-char peer ID
}

func (h *Handshake) Serialized() []byte {
	message := []byte{}
	message = append(message, byte(len(h.Protocol)))
	message = append(message, []byte(h.Protocol)...)
	message = append(message, []byte(h.Reserved)...)
	message = append(message, []byte(h.InfoHash)...)
	message = append(message, []byte(h.PeerId)...)

	return message
}
