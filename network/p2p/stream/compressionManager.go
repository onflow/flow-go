package p2pstream

import (
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
)

type StreamFactory interface {
	NewStream(s libp2pnet.Stream) (libp2pnet.Stream, error)
}
