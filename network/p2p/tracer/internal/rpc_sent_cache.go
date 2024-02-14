package internal

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// rpcCtrlMsgSentCacheConfig configuration for the rpc sent cache.
type rpcCtrlMsgSentCacheConfig struct {
	logger    zerolog.Logger
	sizeLimit uint32
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
func newRPCSentCache(config *rpcCtrlMsgSentCacheConfig) *rpcSentCache {
	backData := herocache.NewCache(config.sizeLimit,
		herocache.DefaultOversizeFactor,
		heropool.LRUEjection,
		config.logger.With().Str("mempool", "gossipsub-rpc-control-messages-sent").Logger(),
		config.collector)
	return &rpcSentCache{
		c: stdmap.NewBackend(stdmap.WithBackData(backData)),
	}
}

// add initializes the record cached for the given messageEntityID if it does not exist.
// Returns true if the record is initialized, false otherwise (i.e.: the record already exists).
// Args:
// - messageId: the message ID.
// - controlMsgType: the rpc control message type.
// Returns:
// - bool: true if the record is initialized, false otherwise (i.e.: the record already exists).
// Note that if add is called multiple times for the same messageEntityID, the record is initialized only once, and the
// subsequent calls return false and do not change the record (i.e.: the record is not re-initialized).
func (r *rpcSentCache) add(messageId string, controlMsgType p2pmsg.ControlMessageType) bool {
	return r.c.Add(newRPCSentEntity(r.rpcSentEntityID(messageId, controlMsgType), controlMsgType))
}

// has checks if the RPC message has been cached indicating it has been sent.
// Args:
// - messageId: the message ID.
// - controlMsgType: the rpc control message type.
// Returns:
// - bool: true if the RPC has been cache indicating it was sent from the local node.
func (r *rpcSentCache) has(messageId string, controlMsgType p2pmsg.ControlMessageType) bool {
	return r.c.Has(r.rpcSentEntityID(messageId, controlMsgType))
}

// size returns the number of records in the cache.
func (r *rpcSentCache) size() uint {
	return r.c.Size()
}

// rpcSentEntityID creates an entity ID from the messageID and control message type.
// Args:
// - messageId: the message ID.
// - controlMsgType: the rpc control message type.
// Returns:
// - flow.Identifier: the entity ID.
func (r *rpcSentCache) rpcSentEntityID(messageId string, controlMsgType p2pmsg.ControlMessageType) flow.Identifier {
	return flow.MakeIDFromFingerPrint([]byte(fmt.Sprintf("%s%s", messageId, controlMsgType)))
}
