# Apricot

A toy client implementation of the [BitTorrent protocol](https://en.wikipedia.org/wiki/BitTorrent).

Currently, a simple CLI is provided for getting information from a torrent file. An implementation of the Bencode format used by BitTorrent is also available.

## CLI

The CLI provides 3 subcommands: `info`, `pieces`, and `peers`.

- `info` returns metadata about a provided torrent file.
- `pieces` returns the SHA1 piece hashes of the torrent.
- `peers` returns all peers announced by the torrent tracker.

All subcommands take a `filename` argument which is a path to a .torrent file.
