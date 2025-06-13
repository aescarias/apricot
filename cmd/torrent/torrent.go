/*
Implementation of the BitTorrent protocol.

Original spec:
	https://bittorrent.org/beps/bep_0003.html

Unofficial, "formal" spec
	https://wiki.theory.org/BitTorrentSpecification
*/

package main

import (
	"crypto/sha1"
	"fmt"
)

type Torrent struct {
	Info        Info   // Information describing the files of this torrent.
	AnnounceURL string // The announce URL of the torrent tracker.
}

type Info struct {
	// The suggested name of the file or directory.
	Name string
	// Number of bytes in each piece.
	PieceLength int
	// Concatenated 20-byte SHA1 hash values for each piece.
	Pieces string
	// (in case of single file) The length of the file in bytes.
	Length *int
	// (in case of multiple files) The files included in the torrent.
	Files *[]InfoFile
}

type InfoFile struct {
	// The length of the file in bytes.
	Length int
	// A slice of path parts ending with the filename.
	Path []string
}

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
			Length:      &length,
			Files:       &files,
		},
		AnnounceURL: contents["announce"].(string),
	}, nil
}

// Returns a Bencodable representation of the info struct.
func (i *Info) Bencodable() map[string]any {
	contents := map[string]any{
		"name":         i.Name,
		"piece length": i.PieceLength,
		"pieces":       i.Pieces,
	}

	if files := *i.Files; len(files) > 0 {
		var items []map[string]any
		for _, file := range files {
			items = append(items, map[string]any{
				"length": file.Length,
				"path":   file.Path,
			})
		}
		contents["files"] = items
	} else {
		contents["length"] = *i.Length
	}

	return contents
}

// Returns the info hash as a byte sequence.
//
// The info hash is a SHA1 hash of the bencoded info struct.
func (i *Info) Hash() ([]byte, error) {
	bencodable := i.Bencodable()

	bencoded, err := EncodeBencode(bencodable)
	if err != nil {
		return nil, fmt.Errorf("could not bencode data for info hash: %w", err)
	}

	infoHash := sha1.New()
	infoHash.Write([]byte(bencoded))
	return infoHash.Sum(nil), nil
}
