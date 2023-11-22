package internal

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p/core/peer"
)

type PeerIdCache struct {
	// TODO: Note that we use lru.Cache as there is an inherent import cycle when using the HeroCache.
	//	 Moving forward we should consider moving the HeroCache to a separate repository and transition
	// 	to using it here.
	// 	This PeerIdCache is used extensively across the codebase, so any minor import cycle will cause
	// 	a lot of trouble.
	peerCache *lru.Cache
}

func NewPeerIdCache(size int) (*PeerIdCache, error) {
	c, err := lru.New(size)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer id cache: %w", err)
	}
	return &PeerIdCache{
		peerCache: c,
	}, nil
}

// PeerIdString returns the base58 encoded peer id string, it looks up the peer id in a cache to avoid
// expensive base58 encoding, and caches the result for future use in case of a cache miss.
// It is safe to call this method concurrently.
func (p *PeerIdCache) PeerIdString(pid peer.ID) string {
	pidStr, ok := p.peerCache.Get(pid)
	if ok {
		return pidStr.(string)
	}

	pidStr0 := pid.String()
	p.peerCache.Add(pid, pidStr0)
	return pidStr0
}

// Size returns the number of entries in the cache; it is mainly used for testing.
func (p *PeerIdCache) Size() int {
	return p.peerCache.Len()
}

// ByPeerId returns the base58 encoded peer id string by directly looking up the peer id in the cache. It is only
// used for testing and since this is an internal package, it is not exposed to the outside world.
func (p *PeerIdCache) ByPeerId(pid peer.ID) (string, bool) {
	pidStr, ok := p.peerCache.Get(pid)
	if ok {
		return pidStr.(string), true
	}
	return "", false
}
