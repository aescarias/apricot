/* A generic text scanner. */
package bencode

import (
	"io"
	"unicode"
)

type Scanner struct {
	Contents     string
	CurrentIndex int
}

// Ended reports whether the scanner has reached the end of contents
func (s *Scanner) Ended() bool {
	return s.CurrentIndex >= len(s.Contents)
}

// Peek gets 'n' characters from the scanner without advancing.
func (s *Scanner) Peek(n int) (string, error) {
	if s.CurrentIndex+n-1 >= len(s.Contents) {
		return "", io.EOF
	}
	return s.Contents[s.CurrentIndex : s.CurrentIndex+n], nil
}

// Consume consumes and returns at most 'n' characters.
func (s *Scanner) Consume(n int) (string, error) {
	consumed, err := s.Peek(n)
	if err != nil {
		return "", err
	}
	if s.Advance(n) {
		return consumed, nil
	}
	return "", nil
}

// Advance skips 'n' characters in the scanner. Returns whether the scanner was advanced.
func (s *Scanner) Advance(n int) bool {
	if s.Ended() {
		return false
	}

	s.CurrentIndex += n
	return true
}

// AdvanceWhitespace skips through all whitespace characters.
//
// A whitespace character is defined as either of the following: horizontal tab ('\t'),
// line feed ('\n'), vertical tab ('\v'), form feed ('\f'), carriage return ('\r'),
// space (' '), next line (U+0085; NEL) and non-breaking space (U+00A0; NBSP).
func (s *Scanner) AdvanceWhitespace() {
	for {
		ch, err := s.Peek(1)
		if err == io.EOF {
			break
		}

		if !unicode.IsSpace(rune(ch[0])) {
			break
		}

		s.Advance(1)
	}
}

// ConsumeUntil scans until 'delimiter' is reached.
//
// It returns a string of the contents before the delimiter and a boolean
// indicating whether the delimiter was reached.
//
// If the delimiter is not reached, the entire contents will be consumed.
func (s *Scanner) ConsumeUntil(delimiter byte) (string, bool) {
	if s.Ended() {
		return "", false
	}

	delimiterFound := false
	var accumulated string

	for !delimiterFound {
		ch, err := s.Peek(1)
		if err == io.EOF {
			break
		}

		if ch[0] == delimiter {
			delimiterFound = true
			break
		}

		accumulated += ch
		s.Advance(1)
	}

	return accumulated, delimiterFound
}
