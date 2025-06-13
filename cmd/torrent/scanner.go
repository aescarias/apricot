/* A generic text scanner. */

package main

import (
	"io"
	"unicode"
)

type Scanner struct {
	contents     string
	currentIndex int
}

// Returns whether the scanner has reached the end of contents
func (s *Scanner) ended() bool {
	return s.currentIndex >= len(s.contents)
}

// Gets 'n' characters from the scanner without advancing.
func (s *Scanner) peek(n int) (string, error) {
	if s.currentIndex+n-1 >= len(s.contents) {
		return "", io.EOF
	}
	return s.contents[s.currentIndex : s.currentIndex+n], nil
}

// Consumes and returns at most 'n' characters.
func (s *Scanner) consume(n int) (string, error) {
	consumed, err := s.peek(n)
	if err != nil {
		return "", err
	}
	if s.advance(n) {
		return consumed, nil
	}
	return "", nil
}

// Advances 'n' characters in the scanner. Returns whether the scanner was advanced.
func (s *Scanner) advance(n int) bool {
	if s.ended() {
		return false
	}

	s.currentIndex += n
	return true
}

// Advances through all whitespace characters.
//
// A whitespace character is defined as either of the following: horizontal tab ('\t'),
// line feed ('\n'), vertical tab ('\v'), form feed ('\f'), carriage return ('\r'),
// space (' '), next line (U+0085; NEL) and non-breaking space (U+00A0; NBSP).
func (s *Scanner) advanceWhitespace() {
	for {
		ch, err := s.peek(1)
		if err == io.EOF {
			break
		}

		if !unicode.IsSpace(rune(ch[0])) {
			break
		}

		s.advance(1)
	}
}

// Scans until 'delimiter' is reached.
//
// The return values are a string of the contents before the delimiter and
// a boolean indicating whether the delimiter was reached.
//
// If the delimiter is not reached, the entire contents will be consumed.
func (s *Scanner) consumeUntil(delimiter byte) (string, bool) {
	if s.ended() {
		return "", false
	}

	delimiterFound := false
	var accumulated string

	for !delimiterFound {
		ch, err := s.peek(1)
		if err == io.EOF {
			break
		}

		if ch[0] == delimiter {
			delimiterFound = true
			break
		}

		accumulated += ch
		s.advance(1)
	}

	return accumulated, delimiterFound
}
