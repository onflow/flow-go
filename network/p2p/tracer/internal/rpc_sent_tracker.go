package internal

import (
	"fmt"

	"github.com/rs/zerolog"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// RPCSentTracker tracks RPC messages that are sent.
type RPCSentTracker struct {
	cache *rpcSentCache
}

// NewRPCSentTracker returns a new *NewRPCSentTracker.
func NewRPCSentTracker(logger zerolog.Logger, sizeLimit uint32, collector module.HeroCacheMetrics) (*RPCSentTracker, error) {
	config := &RPCSentCacheConfig{
		sizeLimit: sizeLimit,
		logger:    logger,
		collector: collector,
	}
	cache, err := newRPCSentCache(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create new rpc sent cahe: %w", err)
	}
	return &RPCSentTracker{cache: cache}, nil
}

// OnIHaveRPCSent caches a unique entity message ID for each message ID included in each rpc iHave control message.
// Args:
// - *pubsub.RPC: the rpc sent.
func (t *RPCSentTracker) OnIHaveRPCSent(iHaves []*pb.ControlIHave) {
	controlMsgType := p2pmsg.CtrlMsgIHave
	for _, iHave := range iHaves {
		topicID := iHave.GetTopicID()
		for _, messageID := range iHave.GetMessageIDs() {
			entityMsgID := iHaveRPCSentEntityID(topicID, messageID)
			t.cache.init(entityMsgID, controlMsgType)
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
	entityMsgID := iHaveRPCSentEntityID(topicID, messageID)
	return t.cache.has(entityMsgID)
}

// iHaveRPCSentEntityID appends the topicId and messageId and returns the flow.Identifier hash.
// Each iHave RPC control message contains a single topicId and multiple messageIds, to ensure we
// produce a unique id for each message we append the messageId to the topicId.
func iHaveRPCSentEntityID(topicId, messageId string) flow.Identifier {
	return flow.MakeIDFromFingerPrint([]byte(fmt.Sprintf("%s%s", topicId, messageId)))
}
