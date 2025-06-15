package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aescarias/apricot/torrent"
	"github.com/aescarias/apricot/torrent/bencode"
)

const NAME = "Apricot"

var VERSION = Version{Major: 0, Minor: 1, Patch: 0}

func OpenTorrent(filename string) *torrent.Torrent {
	contents, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("The file %q does not exist.", filename)
		} else {
			log.Fatal(err)
		}
	}

	tokens, err := bencode.DecodeBencode(string(contents))
	if err != nil {
		log.Fatalf("failed to decode torrent file: %s", err)
	}

	metaInfo, ok := tokens[0].(map[string]any)
	if !ok {
		log.Fatalf("failed to read torrent file: expected meta info dictionary.")
	}

	torrentFile, err := torrent.NewTorrent(metaInfo)
	if err != nil {
		log.Fatalf("failed to read torrent file: %s", err)
	}

	return torrentFile
}

func ShowPeers(filename string) {
	torrentFile := OpenTorrent(filename)

	infoHash, err := torrentFile.Info.Hash()
	if err != nil {
		log.Fatalf("failed to generate info hash: %s", err)
	}

	resp, err := torrentFile.GetPeers(
		torrent.TrackerRequest{
			InfoHash:   string(infoHash),
			PeerId:     MakePeerId(VERSION),
			Port:       6881,
			Uploaded:   0,
			Downloaded: 0,
			Left:       *torrentFile.Info.Length,
			Compact:    1,
		},
	)

	var fr *torrent.ErrFailureReason
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
	torrentFile := OpenTorrent(filename)

	for _, piece := range torrentFile.Info.PieceHashes() {
		pieceStr := hex.EncodeToString([]byte(piece))
		fmt.Printf("%v\n", pieceStr)
	}
}

func ShowInfo(filename string) {
	torrentFile := OpenTorrent(filename)

	fmt.Println("announce url:", torrentFile.AnnounceURL)

	files := *torrentFile.Info.Files
	if len(files) > 0 {
		fmt.Println("dirname:", torrentFile.Info.Name)
	} else {
		fmt.Println("filename:", torrentFile.Info.Name)
	}

	if len(files) > 0 {
		fmt.Printf("files [%d]:\n", len(files))
		for _, file := range files {
			fmt.Printf("  %s [%s]\n", strings.Join(file.Path, "/"), HumanBytes(file.Length))
		}
	} else {
		fmt.Println("file length:", HumanBytes(*torrentFile.Info.Length))
	}

	fmt.Println("piece length:", HumanBytes(torrentFile.Info.PieceLength))

	pieceHashes := torrentFile.Info.PieceHashes()

	fmt.Printf("pieces [%d]: \n", len(pieceHashes))

	for idx := range 2 {
		pieceStr := hex.EncodeToString([]byte(pieceHashes[idx]))
		fmt.Printf("  %v\n", pieceStr)
	}

	if len(pieceHashes) > 3 {
		fmt.Println("  (...)")
	}

	infoHash, err := torrentFile.Info.Hash()
	if err != nil {
		log.Fatalf("could not get info hash: %s", err)
	}

	infoDigest := hex.EncodeToString(infoHash)
	fmt.Print("info hash: ", infoDigest)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("%s %s\n", NAME, VERSION)
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
