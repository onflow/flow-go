package internal

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network/p2p"
)

type entityFactory func(id flow.Identifier, controlMsgType p2p.ControlMessageType) rpcSentEntity

type RPCSentCacheConfig struct {
	sizeLimit uint32
	logger    zerolog.Logger
	collector module.HeroCacheMetrics
}

// rpcSentCache cache that stores rpcSentEntity. These entity's represent RPC control messages sent from the local node.
type rpcSentCache struct {
	// c is the underlying cache.
	c *stdmap.Backend
}

// newRPCSentCache creates a new *rpcSentCache.
// Args:
// - config: record cache config.
// Returns:
// - *rpcSentCache: the created cache.
// Note that this cache is intended to track control messages sent by the local node,
// it stores a RPCSendEntity using an Id which should uniquely identifies the message being tracked.
func newRPCSentCache(config *RPCSentCacheConfig) (*rpcSentCache, error) {
	backData := herocache.NewCache(config.sizeLimit,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection,
		config.logger.With().Str("mempool", "gossipsub=rpc-control-messages-sent").Logger(),
		config.collector)
	return &rpcSentCache{
		c: stdmap.NewBackend(stdmap.WithBackData(backData)),
	}, nil
}

// init initializes the record cached for the given messageID if it does not exist.
// Returns true if the record is initialized, false otherwise (i.e.: the record already exists).
// Args:
// - flow.Identifier: the messageID to store the rpc control message.
// - p2p.ControlMessageType: the rpc control message type.
// Returns:
// - bool: true if the record is initialized, false otherwise (i.e.: the record already exists).
// Note that if init is called multiple times for the same messageID, the record is initialized only once, and the
// subsequent calls return false and do not change the record (i.e.: the record is not re-initialized).
func (r *rpcSentCache) init(messageID flow.Identifier, controlMsgType p2p.ControlMessageType) bool {
	return r.c.Add(newRPCSentEntity(messageID, controlMsgType))
}

// has checks if the RPC message has been cached indicating it has been sent.
// Args:
// - flow.Identifier: the messageID to store the rpc control message.
// Returns:
// - bool: true if the RPC has been cache indicating it was sent from the local node.
func (r *rpcSentCache) has(messageId flow.Identifier) bool {
	return r.c.Has(messageId)
}

// ids returns the list of ids of each rpcSentEntity stored.
func (r *rpcSentCache) ids() []flow.Identifier {
	return flow.GetIDs(r.c.All())
}

// remove the record of the given messageID from the cache.
// Returns true if the record is removed, false otherwise (i.e., the record does not exist).
// Args:
// - flow.Identifier: the messageID to store the rpc control message.
// Returns:
// - true if the record is removed, false otherwise (i.e., the record does not exist).
func (r *rpcSentCache) remove(messageID flow.Identifier) bool {
	return r.c.Remove(messageID)
}

// size returns the number of records in the cache.
func (r *rpcSentCache) size() uint {
	return r.c.Size()
}
