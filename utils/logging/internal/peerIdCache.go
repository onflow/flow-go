package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
)

var _ flow.Entity = (*peerIdCacheEntry)(nil)

type PeerIdCache struct {
	peerCache *stdmap.Backend
}

func NewPeerIdCache(size uint32) *PeerIdCache {
	return &PeerIdCache{
		peerCache: stdmap.NewBackend(
			stdmap.WithBackData(
				herocache.NewCache(
					size,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					zerolog.Nop(),
					metrics.NewNoopCollector()))),
	}
}

func (p *PeerIdCache) PeerIdString(pid peer.ID) string {
	id := flow.MakeIDFromFingerPrint([]byte(pid))
	pidEntity, ok := p.peerCache.ByID(id)
	if !ok {
		return pidEntity.(peerIdCacheEntry).Str
	}
	pidEntity = peerIdCacheEntry{
		id:  id,
		Pid: pid,
		Str: pid.String(),
	}

	p.peerCache.Add(pidEntity)

	return pidEntity.(peerIdCacheEntry).Str
}

type peerIdCacheEntry struct {
	id  flow.Identifier // cache the id for fast lookup
	Pid peer.ID         // peer id
	Str string          // base58 encoded peer id string
}

func (p peerIdCacheEntry) ID() flow.Identifier {
	return p.id
}

func (p peerIdCacheEntry) Checksum() flow.Identifier {
	return p.id
}
