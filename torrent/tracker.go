/* Torrent implementation dealing with trackers. */

package torrent

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/aescarias/apricot/torrent/bencode"
)

// A TrackerEvent represents one of a four events that can be sent in the tracker request.
type TrackerEvent string

const (
	EventStarted   TrackerEvent = "started"
	EventCompleted TrackerEvent = "completed"
	EventStopped   TrackerEvent = "stopped"
	EventEmpty     TrackerEvent = "empty"
)

// A TrackerRequest represents the request parameters sent to the tracker announce endpoint.
type TrackerRequest struct {
	// The 20-byte info hash which is the SHA1 hash of the bencoded form of the info value from the metainfo file.
	InfoHash [20]byte
	// A string of length 20 identifying the downloader.
	PeerId string
	// (optional) The IP or DNS name which this peer is at.
	Ip string
	// The port number the peer is listening on.
	Port int
	// The total amount uploaded so far.
	Uploaded int
	// The total amount downloaded so far.
	Downloaded int
	// The number of bytes this peer still has to download.
	Left int
	// (optional) An announcement to the tracker.
	//
	// 'started' announces that the download has just started. 'stopped' announces
	// that the downloader has ceased downloading. 'completed' announces that the
	// downloader has completely downloaded the file. 'empty' is a no op.
	Event TrackerEvent
	// (optional) Whether to represent the peer list in compact format. 0 means no, 1 means yes.
	//
	// This field is merely advisory. A tracker may send peers in any format desired
	// regardless of whether 'compact' is 0 or 1. A tracker may also refuse connections
	// that use 'compact=0'.
	Compact int
}

// A TrackerResponse represents the response sent by the announce endpoint.
type TrackerResponse struct {
	Interval int           // The interval in seconds to wait before re-requests.
	Peers    []TrackerPeer // A list of peers
}

// A TrackerPeer represents a peer returned in the tracker response.
type TrackerPeer struct {
	Ip     string // The IP of the peer
	Port   int    // The port of the peer
	PeerId string // The peer ID. If using a compact format, this field may be empty.
}

// An ErrFailureReason occurs when the tracker responds with a bencoded message
// including the 'failure reason' key.
type ErrFailureReason struct {
	Message string // The failure reason
}

func (p TrackerPeer) String() string {
	return net.JoinHostPort(p.Ip, fmt.Sprint(p.Port))
}

func (err *ErrFailureReason) Error() string {
	return err.Message
}

// GetPeers gets the tracker peers announced by a URL in the announce list.
// Returns the tracker response including the peers and an error if any.
//
// A tracker may announce peers over TCP, UDP, or WebSockets. Only the former
// is implemented.
func (t *Torrent) GetPeers(request TrackerRequest) (*TrackerResponse, error) {
	announce, err := url.Parse(t.AnnounceURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse url: %w", err)
	}

	switch announce.Scheme {
	case "http", "https":
		query := announce.Query()

		query.Set("info_hash", string(request.InfoHash[:]))
		query.Set("peer_id", request.PeerId)
		query.Set("left", fmt.Sprint(request.Left))
		query.Set("downloaded", fmt.Sprint(request.Downloaded))
		query.Set("uploaded", fmt.Sprint(request.Uploaded))

		if len(request.Ip) > 0 {
			query.Set("ip", request.Ip)
		}

		query.Set("port", fmt.Sprint(request.Port))
		query.Set("compact", fmt.Sprint(request.Compact))

		announce.RawQuery = query.Encode()
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", announce.Scheme)
	}

	resp, err := http.Get(announce.String())
	if err != nil {
		return nil, fmt.Errorf("request to tracker failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request to tracker returned %s", resp.Status)
	}

	read, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response: %w", err)
	}

	tokens, err := bencode.DecodeBencode(string(read))
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

// compactToPeerList decompress a peer list in compact format into a slice of tracker peers.
func compactToPeerList(format string) []TrackerPeer {
	var peerList []TrackerPeer

	for idx := 0; idx < len(format); idx += 6 {
		ipBytes := []byte(format[idx : idx+4])
		portBytes := []byte(format[idx+4 : idx+6])

		portInt := binary.BigEndian.Uint16(portBytes)
		ip := fmt.Sprintf("%d.%d.%d.%d", ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3])

		peerList = append(peerList, TrackerPeer{Port: int(portInt), Ip: ip})
	}

	return peerList
}
