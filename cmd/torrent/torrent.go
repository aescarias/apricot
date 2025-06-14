/*
Client implementation of the BitTorrent protocol.

Original spec:
	https://bittorrent.org/beps/bep_0003.html

Unofficial, "formal" spec
	https://wiki.theory.org/BitTorrentSpecification
*/

package main

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

/* PieceHashes returns a slice of all SHA1 piece hashes. */
func (i *Info) PieceHashes() []string {
	var hashes []string

	for idx := 0; idx <= len(i.Pieces)-20; idx += 20 {
		hashes = append(hashes, i.Pieces[idx:idx+20])
	}

	return hashes
}

type InfoFile struct {
	// The length of the file in bytes.
	Length int
	// A slice of path parts ending with the filename.
	Path []string
}

type TrackerResponse struct {
	Interval int
	Peers    []TrackerPeer
}

type TrackerPeer struct {
	Ip     string
	Port   int
	PeerId string
}

type ErrFailureReason struct {
	Message string
}

func (err *ErrFailureReason) Error() string {
	return err.Message
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

// Bencodable returns a Bencodable representation of the info struct.
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

// Hash returns the info hash as a byte sequence and an error if any.
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

func compactToPeerList(peers string) []TrackerPeer {
	var peerList []TrackerPeer
	peerBytes := []byte(peers)

	for idx := 0; idx < len(peerBytes); idx += 6 {
		ipBytes := peerBytes[idx : idx+4]
		portBytes := peerBytes[idx+4 : idx+6]

		portInt := binary.BigEndian.Uint16(portBytes)
		ip := fmt.Sprintf("%d.%d.%d.%d", ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3])

		peerList = append(peerList, TrackerPeer{Port: int(portInt), Ip: ip})
	}

	return peerList
}

/*
GetPeers gets the tracker peers announced by a URL in the announce list.
Returns the tracker response including the peers and an error if any.

A tracker may announce peers over TCP, UDP, or WebSockets. Only the former
is implemented.
*/
func (t *Torrent) GetPeers(peerId string) (*TrackerResponse, error) {
	announce, err := url.Parse(t.AnnounceURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse url: %w", err)
	}

	switch announce.Scheme {
	case "http", "https":
		query := announce.Query()

		infohash, err := t.Info.Hash()
		if err != nil {
			return nil, fmt.Errorf("could not get info hash: %w", err)
		}

		query.Set("info_hash", string(infohash))
		query.Set("peer_id", peerId)
		query.Set("left", fmt.Sprint(*t.Info.Length))
		query.Set("downloaded", "0")
		query.Set("uploaded", "0")
		// TODO: This would preferably be cycled (port in URL, then 6881, then 6882, and so on).
		query.Set("port", "6881")

		announce.RawQuery = query.Encode()
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", announce.Scheme)
	}

	resp, err := http.Get(announce.String())
	if err != nil {
		return nil, fmt.Errorf("request to tracker failed: %w", err)
	}
	defer resp.Body.Close()

	read, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response: %w", err)
	}

	tokens, err := DecodeBencode(string(read))
	if err != nil {
		return nil, fmt.Errorf("could not decode response: %w", err)
	}

	response, ok := tokens[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %v", response)
	}

	if failure, ok := response["failure reason"]; ok {
		return nil, &ErrFailureReason{Message: failure.(string)}
	}

	var peerList []TrackerPeer
	switch peers := response["peers"].(type) {
	case []any:
		for _, peer := range peers {
			peer, ok := peer.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("peer of unexpected type: %v", peer)
			}

			peerList = append(peerList, TrackerPeer{
				Ip:     peer["ip"].(string),
				Port:   peer["port"].(int),
				PeerId: peer["peer id"].(string),
			})
		}
	case string:
		peerList = compactToPeerList(peers)
	default:
		return nil, fmt.Errorf("unknown peer list kind: %v", peers)
	}

	return &TrackerResponse{
		Interval: response["interval"].(int),
		Peers:    peerList,
	}, nil
}
