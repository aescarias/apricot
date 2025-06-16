/*
Client implementation of the BitTorrent protocol.

Original spec:
	https://bittorrent.org/beps/bep_0003.html

Unofficial, "formal" spec
	https://wiki.theory.org/BitTorrentSpecification
*/

package torrent

import (
	"crypto/sha1"
	"fmt"

	"github.com/aescarias/apricot/torrent/bencode"
)

// A Torrent represents the contents of a .torrent file.
type Torrent struct {
	Info        Info   // Information describing the files of this torrent.
	AnnounceURL string // The announce URL of the torrent tracker.
}

// An Info represents the contents of the 'info' dictionary in the .torrent file.
type Info struct {
	// The suggested name of the file or directory.
	Name string
	// Number of bytes in each piece.
	PieceLength int
	// Concatenated 20-byte SHA1 hash values for each piece.
	Pieces string
	// In case of a single file torrent, the length of the file in bytes.
	Length int
	// In case of a multiple file torrent, the files included in the torrent.
	Files []InfoFile
}

// An InfoFile represents an individual file within a multiple file torrent.
type InfoFile struct {
	// The length of the file in bytes.
	Length int
	// A slice of path parts ending with the filename.
	Path []string
}

// PieceHashes returns a slice of all SHA1 piece hashes described in the torrent.
func (i *Info) PieceHashes() []string {
	var hashes []string

	for idx := 0; idx <= len(i.Pieces)-20; idx += 20 {
		hashes = append(hashes, i.Pieces[idx:idx+20])
	}

	return hashes
}

// TotalLength returns the total amount of bytes contained in this torrent.
//
// For single file torrents, this returns the same value as Length. For multiple
// file torrents, this returns the sum of the file lengths in the torrent.
func (i *Info) TotalLength() int {
	if len(i.Files) <= 0 {
		return i.Length
	}

	total := 0

	for _, file := range i.Files {
		total += file.Length
	}

	return total
}

// Bencodable returns a Bencodable representation of the info struct.
func (i *Info) Bencodable() map[string]any {
	contents := map[string]any{
		"name":         i.Name,
		"piece length": i.PieceLength,
		"pieces":       i.Pieces,
	}

	if files := i.Files; len(files) > 0 {
		var items []map[string]any
		for _, file := range files {
			items = append(items, map[string]any{
				"length": file.Length,
				"path":   file.Path,
			})
		}
		contents["files"] = items
	} else {
		contents["length"] = i.Length
	}

	return contents
}

// Hash returns the info hash as a byte sequence and an error if any.
//
// The info hash is a SHA1 hash of the bencoded info struct.
func (i *Info) Hash() ([20]byte, error) {
	bencodable := i.Bencodable()

	bencoded, err := bencode.EncodeBencode(bencodable)
	if err != nil {
		return [20]byte{}, fmt.Errorf("could not bencode data for info hash: %w", err)
	}

	return sha1.Sum([]byte(bencoded)), nil
}

// newInfoFileSlice parses a decoded 'items' list into a slice of files included
// in the torrent. Returns this slice or an error if any.
func newInfoFileSlice(items []any) ([]InfoFile, error) {
	files := make([]InfoFile, len(items))

	for nth, item := range items {
		item, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid file item: %v", item)
		}

		rawPath, ok := item["path"].([]any)
		if !ok {
			return nil, fmt.Errorf("invalid path list: %v", rawPath)
		}

		path := make([]string, len(rawPath))

		for idx, part := range rawPath {
			path[idx] = part.(string)
		}

		files[nth] = InfoFile{
			Length: item["length"].(int),
			Path:   path,
		}
	}

	return files, nil
}

// NewTorrent creates a Torrent structure from a decoded 'contents' dictionary
// representing the .torrent file. Returns the structure or an error if any.
func NewTorrent(contents map[string]any) (*Torrent, error) {
	info := contents["info"].(map[string]any)

	var files []InfoFile
	if items, ok := info["files"].([]any); ok {
		var err error

		files, err = newInfoFileSlice(items)
		if err != nil {
			return nil, fmt.Errorf("could not parse files list: %w", err)
		}
	}

	length, _ := info["length"].(int)

	return &Torrent{
		Info: Info{
			Name:        info["name"].(string),
			PieceLength: info["piece length"].(int),
			Pieces:      info["pieces"].(string),
			Length:      length,
			Files:       files,
		},
		AnnounceURL: contents["announce"].(string),
	}, nil
}
