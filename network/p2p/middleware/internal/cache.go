package internal

import (
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network/p2p/middleware"
)

type DisallowListCache struct {
	c *stdmap.Backend
}

var _ middleware.DisallowListCache = (*DisallowListCache)(nil)

func NewDisallowListCache() DisallowListCache {
	return DisallowListCache{
		c: stdmap.NewBackend(),
	}
}

func (d *DisallowListCache) IsDisallowed(peerID string) ([]middleware.DisallowListedCause, bool) {
	d.c.ByID()
}

func (d *DisallowListCache) DisallowFor(peerID string, cause middleware.DisallowListedCause) error {
	//TODO implement me
	panic("implement me")
}

func (d *DisallowListCache) AllowFor(peerID string, cause middleware.DisallowListedCause) error {
	//TODO implement me
	panic("implement me")
}
