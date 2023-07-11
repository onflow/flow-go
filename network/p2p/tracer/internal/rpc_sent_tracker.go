package internal

import (
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// RPCSentTracker tracks RPC messages that are sent.
type RPCSentTracker struct {
	cache *rpcSentCache
}

// NewRPCSentTracker returns a new *NewRPCSentTracker.
func NewRPCSentTracker(logger zerolog.Logger, sizeLimit uint32, collector module.HeroCacheMetrics) *RPCSentTracker {
	config := &rpcCtrlMsgSentCacheConfig{
		sizeLimit: sizeLimit,
		logger:    logger,
		collector: collector,
	}
	return &RPCSentTracker{cache: newRPCSentCache(config)}
}

// OnIHaveRPCSent caches a unique entity message ID for each message ID included in each rpc iHave control message.
// Args:
// - *pubsub.RPC: the rpc sent.
func (t *RPCSentTracker) OnIHaveRPCSent(iHaves []*pb.ControlIHave) {
	controlMsgType := p2pmsg.CtrlMsgIHave
	for _, iHave := range iHaves {
		topicID := iHave.GetTopicID()
		for _, messageID := range iHave.GetMessageIDs() {
			t.cache.add(topicID, messageID, controlMsgType)
		}
	}
}

// WasIHaveRPCSent checks if an iHave control message with the provided message ID was sent.
// Args:
// - string: the topic ID of the iHave RPC.
// - string: the message ID of the iHave RPC.
// Returns:
// - bool: true if the iHave rpc with the provided message ID was sent.
func (t *RPCSentTracker) WasIHaveRPCSent(topicID, messageID string) bool {
	return t.cache.has(topicID, messageID, p2pmsg.CtrlMsgIHave)
}
