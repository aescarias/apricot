package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strings"
)

const VERSION = "0.1.0"

func OpenTorrent(filename string) *Torrent {
	contents, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("The filename %q does not exist.\n", filename)
			os.Exit(1)
		} else {
			panic(err)
		}
	}

	tokens, err := DecodeBencode(string(contents))
	if err != nil {
		panic(err)
	}

	metainfo, ok := tokens[0].(map[string]any)
	if !ok {
		panic("data error: expected metainfo dictionary.")
	}

	torrent, err := NewTorrent(metainfo)
	if err != nil {
		panic(err)
	}

	return torrent
}

func makePeerId() string {
	min, max := 100_000_000_000, 999_999_999_999
	randVal := rand.Intn(max+1-min) + min

	return fmt.Sprint("-ST0010-", randVal)
}

func ShowPeers(filename string) {
	torrent := OpenTorrent(filename)

	resp, err := torrent.GetPeers(makePeerId())
	var fr *ErrFailureReason
	if errors.As(err, &fr) {
		log.Fatalf("tracker returned error: %s", fr.Message)
	}

	if err != nil {
		log.Fatalf("could not get peers: %v\n", err)
	}

	fmt.Printf("request interval: %d seconds\n", resp.Interval)

	if len(resp.Peers) <= 0 {
		return
	}

	for idx, peer := range resp.Peers {
		fmt.Println("peer", idx+1)
		fmt.Println("  ip:     ", peer.Ip)
		fmt.Println("  port:   ", peer.Port)
		fmt.Printf("  peer id: %x\n", peer.PeerId)
	}
}

func ShowPieces(filename string) {
	torrent := OpenTorrent(filename)

	for idx := 0; idx <= len(torrent.Info.Pieces)-20; idx += 20 {
		pieceStr := hex.EncodeToString([]byte(torrent.Info.Pieces[idx : idx+20]))
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
		for idx, file := range files {
			fmt.Println("file", idx+1)
			fmt.Println("  filepath:", strings.Join(file.Path, "/"))
			fmt.Println("  length:", HumanBytes(file.Length))
		}
	} else {
		fmt.Println("file length:", HumanBytes(*torrent.Info.Length))
	}

	fmt.Println("piece length:", HumanBytes(torrent.Info.PieceLength))

	pieceCount := int64(math.Ceil(float64(len(torrent.Info.Pieces)) / 20))
	fmt.Printf("pieces [%d]: \n", pieceCount)

	for idx := 0; idx <= 2*20; idx += 20 {
		pieceStr := hex.EncodeToString([]byte(torrent.Info.Pieces[idx : idx+20]))
		fmt.Printf("  %v\n", pieceStr)
	}

	if pieceCount > 3 {
		fmt.Println("  (...)")
	}

	infoHash, err := torrent.Info.Hash()
	if err != nil {
		panic(err)
	}

	infoDigest := hex.EncodeToString(infoHash)
	fmt.Print("info hash: ", infoDigest)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("gostream %s\n", VERSION)
		fmt.Printf("usage: %s {info,pieces,peers} <options>\n", os.Args[0])
		os.Exit(1)
	}

	progArgs := os.Args[1:]

	switch progArgs[0] {
	case "info":
		if len(progArgs) < 2 {
			fmt.Printf("usage: %s info <filename>\n", os.Args[0])
			os.Exit(1)
		}
		ShowInfo(progArgs[1])

	case "pieces":
		if len(progArgs) < 2 {
			fmt.Printf("usage: %s pieces <filename>\n", os.Args[0])
			os.Exit(1)
		}

		ShowPieces(progArgs[1])
	case "peers":
		if len(progArgs) < 2 {
			fmt.Printf("usage: %s peers <filename>\n", os.Args[0])
			os.Exit(1)
		}

		ShowPeers(progArgs[1])
	default:
		fmt.Printf("invalid subcommand %q\n", progArgs[0])
		fmt.Printf("subcommands: info, pieces, peers\n")
		os.Exit(1)
	}
}
