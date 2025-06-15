package main

import (
	"fmt"
	"math/rand"
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

// RandIntString creates a string of length N with random integers.
func RandIntString(n int) string {
	var contents string

	for _ = range n {
		contents += fmt.Sprint(rand.Intn(2))
	}

	return contents
}

// MakePeerId returns a peer ID string that is 20 bytes long appropriate for use
// with the BitTorrent protocol.
func MakePeerId(version Version) string {
	// There are a few conventions in use for peer IDs. The one used here
	// (Azureus-style) includes a client and version identifier alongside
	// random numbers.
	ident := fmt.Sprintf("-PI%d%02d%d-", version.Major, version.Minor, version.Patch)
	return fmt.Sprint(ident, RandIntString(20-len(ident)))
}

type Version struct {
	Major int
	Minor int
	Patch int
}

// String returns a version string in the form MAJOR.MINOR.PATCH
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
