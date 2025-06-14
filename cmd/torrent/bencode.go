/*
	Encoder and decoder for the Bencode data serialization format.

	See https://bittorrent.org/beps/bep_0003.html#bencoding
*/

package main

import (
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"unicode"
)

// Parses a Bencode string of the form 'length:string'.
//
// Strings are length-prefixed base ten followed by a colon and the string.
//
// For example 4:spam corresponds to 'spam'.
func ParseBencodeString(scanner *Scanner) (string, error) {
	start := scanner.CurrentIndex
	digitStr, found := scanner.ConsumeUntil(':')

	if !found {
		scanner.CurrentIndex = start
		return "", fmt.Errorf("expected length specification")
	}

	strLen, err := strconv.Atoi(digitStr)
	if err != nil {
		return "", fmt.Errorf("length conversion errored: %w", err)
	}

	scanner.Advance(1) // past the ":"

	strVal, err := scanner.Consume(strLen)
	if err != nil {
		return "", err
	}

	return strVal, nil
}

// Parses a Bencode integer of the form 'i...e'.
//
// Integers are represented by an 'i' followed by the number in base 10 and ended
// by an 'e'. For example i3e corresponds to 3 and i-3e corresponds to -3.
//
// Integers have no size limitation. i-0e is invalid. All encodings with a
// leading zero, such as i03e, are invalid, other than i0e, which is just zero.
func ParseBencodeInteger(scanner *Scanner) (int, error) {
	scanner.Advance(1) // past the 'i'
	digitStr, found := scanner.ConsumeUntil('e')

	if !found {
		return 0, fmt.Errorf("expected end of integer")
	}

	number, err := strconv.Atoi(digitStr)
	if err != nil {
		return 0, fmt.Errorf("integer conversion errored: %s", err)
	}

	scanner.Advance(1)
	return number, nil
}

// Parses a Bencode list of the form l...e
//
// Lists are encoded as an 'l' followed by Bencode elements and ended by an 'e'.
// For example l4:spam4:eggse corresponds to ['spam', 'eggs'].
func ParseBencodeList(scanner *Scanner) ([]any, error) {
	var tokens []any

	scanner.Advance(1) // past the "l"

	for !scanner.Ended() {
		scanner.AdvanceWhitespace()

		ch, err := scanner.Peek(1)
		if err == io.EOF {
			return nil, err
		}

		if ch[0] == 'e' {
			scanner.Advance(1) // advance past the 'e'
			break
		}

		token, err := ParseBencodeToken(scanner)
		if err != nil {
			return nil, err
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

// Parses a Bencode dictionary of the form 'd...e'
//
// Dictionaries are encoded as a 'd' followed by a list of alternating keys
// and their corresponding values followed by an 'e'.
//
// For example, d3:cow3:moo4:spam4:eggse corresponds to {'cow': 'moo', 'spam': 'eggs'}
// and d4:spaml1:a1:bee corresponds to {'spam': ['a', 'b']}.
//
// Keys must be strings and appear in sorted order (sorted as raw strings, not alphanumerics).
func ParseBencodeDictionary(scanner *Scanner) (map[string]any, error) {
	dictionary := make(map[string]any)

	scanner.Advance(1)
	for !scanner.Ended() {
		scanner.AdvanceWhitespace()
		ch, err := scanner.Peek(1)
		if err != nil {
			return nil, err
		}

		if ch[0] == 'e' {
			scanner.Advance(1)
			break
		}

		key, err := ParseBencodeToken(scanner)
		if err != nil {
			return nil, err
		}

		scanner.AdvanceWhitespace()
		value, err := ParseBencodeToken(scanner)
		if err != nil {
			return nil, err
		}

		dictionary[key.(string)] = value
	}

	return dictionary, nil
}

// Parses any valid Bencode token. The 4 data types supported by Bencode are
// Integers, Strings, Lists and Dictionaries.
func ParseBencodeToken(scanner *Scanner) (any, error) {
	ch, err := scanner.Peek(1)
	if err == io.EOF {
		return nil, err
	}

	if unicode.IsDigit(rune(ch[0])) {
		return ParseBencodeString(scanner)
	} else if ch[0] == 'i' {
		return ParseBencodeInteger(scanner)
	} else if ch[0] == 'l' {
		return ParseBencodeList(scanner)
	} else if ch[0] == 'd' {
		return ParseBencodeDictionary(scanner)
	}

	return nil, fmt.Errorf("unexpected character %q", ch)
}

// Decodes a Bencoded string into a Go object.
func DecodeBencode(contents string) ([]any, error) {
	scanner := Scanner{Contents: contents, CurrentIndex: 0}

	var tokens []any

	for !scanner.Ended() {
		scanner.AdvanceWhitespace()

		token, err := ParseBencodeToken(&scanner)
		if err != nil {
			return nil, err
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

// Encodes a Go object `contents` into a Bencode string provided that the object
// is serializable (i.e. either an integer, string, map or list).
func EncodeBencode(contents any) (string, error) {
	switch token := reflect.ValueOf(contents); token.Kind() {
	case reflect.String:
		str := token.String()
		return fmt.Sprintf("%d:%s", len(str), str), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("i%de", token.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("i%de", token.Uint()), nil
	case reflect.Slice, reflect.Array:
		var bencoded string
		for idx := range token.Len() {
			itemCoded, err := EncodeBencode(token.Index(idx).Interface())
			if err != nil {
				return "", fmt.Errorf("error while encoding list item: %w", err)
			}
			bencoded += itemCoded
		}
		return "l" + bencoded + "e", nil
	case reflect.Map:
		var bencoded string

		orderedKeys := []string{}
		for _, value := range token.MapKeys() {
			orderedKeys = append(orderedKeys, value.String())
		}
		slices.Sort(orderedKeys)

		for _, key := range orderedKeys {
			keyCoded, err := EncodeBencode(key)
			if err != nil {
				return "", fmt.Errorf("error while encoding dict key: %w", err)
			}

			value := token.MapIndex(reflect.ValueOf(key))

			valueCoded, err := EncodeBencode(value.Interface())
			if err != nil {
				return "", fmt.Errorf("error while encoding dict value: %w", err)
			}

			bencoded += keyCoded + valueCoded
		}
		return "d" + bencoded + "e", nil
	default:
		return "", fmt.Errorf("cannot serialize value %v", contents)
	}
}
