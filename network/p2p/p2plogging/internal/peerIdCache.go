package internal

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p/core/peer"
)

type PeerIdCache struct {
	peerCache *lru.Cache
}

func NewPeerIdCache(size int) *PeerIdCache {
	c, err := lru.New(size)
	if err != nil {
		panic(fmt.Sprintf("failed to create lru cache for peer ids: %v", err))
	}
	return &PeerIdCache{
		peerCache: c,
	}
}

func (p *PeerIdCache) PeerIdString(pid peer.ID) string {
	pidStr, ok := p.peerCache.Get(pid)
	if ok {
		return pidStr.(string)
	}

	pidStr0 := pid.String()
	p.peerCache.Add(pid, pidStr0)
	return pidStr0
}

func (p *PeerIdCache) Size() int {
	return p.peerCache.Len()
}

func (p *PeerIdCache) ByPeerId(pid peer.ID) (string, bool) {
	pidStr, ok := p.peerCache.Get(pid)
	if ok {
		return pidStr.(string), true
	}
	return "", false
}
