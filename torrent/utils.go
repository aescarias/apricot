package torrent

import (
	"io"
)

// ReadN reads and returns N bytes from a reader (via the reader parameter).
//
// It returns the bytes read from 'reader' and an error if any.
//
// ReadN is simply a helper function wrapping io.ReadFull. It returns the same errors
// in the same scenarios as ReadFull, that is, it returns an error if less than N bytes
// are available for reading.
func ReadN(n int, reader io.Reader) ([]byte, error) {
	contents := make([]byte, n)

	bytesRead, err := io.ReadFull(reader, contents)
	if err != nil {
		return nil, err
	}

	return contents[:bytesRead], nil
}
