package main

import "fmt"

const STEP_SIZE = 1000

var UNITS = [...]string{"B", "KB", "MB", "GB", "TB", "PB"} // Decimal units

// Converts a number in bytes to a human-readable representation.
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
