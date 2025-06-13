# gostream

A toy implementation of the BitTorrent protocol.

Currently, a simple CLI is provided for getting information from a torrent file. An implementation of the Bencode format is also available.

## CLI

The CLI provides 2 subcommands: `info` and `pieces`.

- `info` returns metadata about a provided torrent file.
- `pieces` returns the SHA1 piece hashes of the torrent.

Both subcommands take a `filename` argument.
