package main

import (
	"fmt"
	"io"
)

const STEP_SIZE = 1000                                     // Decimal unit step size
var UNITS = [...]string{"B", "KB", "MB", "GB", "TB", "PB"} // Decimal units

// HumanBytes converts a number in bytes (via the bytes parameter)
// to a human-readable size representation in decimal units.
//
// For example, HumanBytes(1000) will return "1 KB".
func HumanBytes(bytes int) string {
	number := float64(bytes)

	var unit string
	for idx := range len(UNITS) {
		unit = UNITS[idx]

		if number < STEP_SIZE {
			break
		}

		number /= STEP_SIZE
	}

	return fmt.Sprintf("%.2f %s", number, unit)
}

// ReadN reads and returns up to N bytes from a reader (via the reader parameter).
//
// It returns the bytes read from 'reader' and an error if any.
//
// ReadN may return less than 'n' bytes in case the reader has no more bytes available.
// If no more bytes can be read, returns an empty byte slice and io.EOF.
func ReadN(n int, reader io.Reader) ([]byte, error) {
	contents := make([]byte, n)

	nRead, err := reader.Read(contents)

	if err != nil {
		return nil, err
	}

	return contents[:nRead], nil
}
