package p2plogging

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p/p2plogging/internal"
)

// peerIdCache is a global cache of peer ids, it is used to avoid expensive base58 encoding of peer ids.
var peerIdCache *internal.PeerIdCache

// init is called before the package is initialized. This is used to initialize
// the peer id cache before any other code is run, so that the cache is ready
// to use.
func init() {
	cache, err := internal.NewPeerIdCache(10_000)
	if err != nil {
		panic(err)
	}
	peerIdCache = cache
}

// PeerId is a logger helper that returns the base58 encoded peer id string, it looks up the peer id in a cache to avoid
// expensive base58 encoding, and caches the result for future use in case of a cache miss.
func PeerId(pid peer.ID) string {
	return peerIdCache.PeerIdString(pid)
}
