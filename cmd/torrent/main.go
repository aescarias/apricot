package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
)

// While the v1 BitTorrent spec describes 32 KiB as a block size,
// in reality, most trackers use and even force 16 KiB block sizes.

const BLOCK_SIZE = 16 * 1024
const VERSION = "0.1.0"

func makePeerId() string {
	// A peer ID is 20 bytes long. There are a few conventions in use for peer
	// IDs. The one used here (Azureus-style) includes a client and version
	// identifier alongside 12 random numbers.
	min, max := 100_000_000_000, 999_999_999_999
	randVal := rand.Intn(max+1-min) + min

	return fmt.Sprint("-GX0010-", randVal)
}

func OpenTorrent(filename string) *Torrent {
	contents, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("The file %q does not exist.", filename)
		} else {
			log.Fatal(err)
		}
	}

	tokens, err := DecodeBencode(string(contents))
	if err != nil {
		log.Fatalf("could not decode file: %s", err)
	}

	metainfo, ok := tokens[0].(map[string]any)
	if !ok {
		log.Fatalf("failed to read torrent: expected metainfo dictionary.")
	}

	torrent, err := NewTorrent(metainfo)
	if err != nil {
		log.Fatalf("failed to read torrent: %s", err)
	}

	return torrent
}

func ShowPeers(filename string) {
	torrent := OpenTorrent(filename)

	infoHash, err := torrent.Info.Hash()
	if err != nil {
		log.Fatalf("failed to generate info hash: %s", err)
	}

	resp, err := torrent.GetPeers(
		TrackerRequest{
			InfoHash:   string(infoHash),
			PeerId:     makePeerId(),
			Port:       6881,
			Uploaded:   0,
			Downloaded: 0,
			Left:       *torrent.Info.Length,
			Compact:    1,
		},
	)
	var fr *ErrFailureReason
	if errors.As(err, &fr) {
		log.Fatalf("tracker returned error: %s", fr.Message)
	}

	if err != nil {
		log.Fatalf("could not get peers: %v\n", err)
	}

	fmt.Printf("request interval: %d seconds\n", resp.Interval)

	if len(resp.Peers) <= 0 {
		fmt.Printf("no peers")
		return
	}

	for idx, peer := range resp.Peers {
		fmt.Println("peer", idx+1)
		fmt.Println("  ip:     ", peer.Ip)
		fmt.Println("  port:   ", peer.Port)
		if len(peer.PeerId) > 0 {
			fmt.Printf("  peer id: %x\n", peer.PeerId)
		}
	}
}

func ShowPieces(filename string) {
	torrent := OpenTorrent(filename)

	for _, piece := range torrent.Info.PieceHashes() {
		pieceStr := hex.EncodeToString([]byte(piece))
		fmt.Printf("%v\n", pieceStr)
	}
}

func ShowInfo(filename string) {
	torrent := OpenTorrent(filename)

	fmt.Println("announce url:", torrent.AnnounceURL)

	files := *torrent.Info.Files
	if len(files) > 0 {
		fmt.Println("dirname:", torrent.Info.Name)
	} else {
		fmt.Println("filename:", torrent.Info.Name)
	}

	if len(files) > 0 {
		fmt.Printf("files [%d]:\n", len(files))
		for _, file := range files {
			fmt.Printf("  %s [%s]\n", strings.Join(file.Path, "/"), HumanBytes(file.Length))
		}
	} else {
		fmt.Println("file length:", HumanBytes(*torrent.Info.Length))
	}

	fmt.Println("piece length:", HumanBytes(torrent.Info.PieceLength))

	pieceHashes := torrent.Info.PieceHashes()

	fmt.Printf("pieces [%d]: \n", len(pieceHashes))

	for idx := range 2 {
		pieceStr := hex.EncodeToString([]byte(pieceHashes[idx]))
		fmt.Printf("  %v\n", pieceStr)
	}

	if len(pieceHashes) > 3 {
		fmt.Println("  (...)")
	}

	infoHash, err := torrent.Info.Hash()
	if err != nil {
		log.Fatalf("could not get info hash: %s", err)
	}

	infoDigest := hex.EncodeToString(infoHash)
	fmt.Print("info hash: ", infoDigest)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("gostream %s\n", VERSION)
		fmt.Printf("usage: %s {info,peers,pieces} <options>\n", os.Args[0])
		os.Exit(1)
	}

	progArgs := os.Args[1:]

	switch progArgs[0] {
	case "info":
		if len(progArgs) < 2 {
			log.Fatalf("usage: %s info <filename>\n", os.Args[0])
		}
		ShowInfo(progArgs[1])
	case "pieces":
		if len(progArgs) < 2 {
			log.Fatalf("usage: %s pieces <filename>\n", os.Args[0])
		}

		ShowPieces(progArgs[1])
	case "peers":
		if len(progArgs) < 2 {
			log.Fatalf("usage: %s peers <filename>\n", os.Args[0])
		}

		ShowPeers(progArgs[1])
	default:
		fmt.Printf("invalid subcommand %q\n", progArgs[0])
		fmt.Printf("subcommands: info, peers, pieces\n")
		os.Exit(1)
	}
}
