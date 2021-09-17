package p2p

import (
	"strings"

	"github.com/libp2p/go-libp2p-core/peer"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network"
)

// RoleBasedFilter implements a subscription filter that filters subscriptions based on a node's role.
type RoleBasedFilter struct {
	idProvider  id.IdentityProvider
	myPeerID    peer.ID
	rootBlockID flow.Identifier
	chainID     flow.ChainID
}

func NewRoleBasedFilter(pid peer.ID, rootBlockID flow.Identifier, chainID flow.ChainID, idProvider id.IdentityProvider) *RoleBasedFilter {
	return &RoleBasedFilter{
		idProvider,
		pid,
		rootBlockID,
		chainID,
	}
}

func (f *RoleBasedFilter) allowedTopics(pid peer.ID) map[network.Topic]struct{} {
	id, found := f.idProvider.ByPeerID(pid)
	channels := engine.PublicChannels()

	if !found {
		// TODO: eventually we should have block proposals relayed on a separate
		// channel on the public network. For now, we need to make sure that
		// full observer nodes can subscribe to the block proposal channel.
		channels = append(channels, engine.ReceiveBlocks)
	} else {
		channels = append(channels, engine.ChannelsByRole(id.Role)...)
	}

	topics := make(map[network.Topic]struct{})

	for _, ch := range channels {
		consensusCluster := engine.ChannelConsensusCluster(f.chainID)
		syncCluster := engine.ChannelSyncCluster(f.chainID)

		if strings.HasPrefix(consensusCluster.String(), ch.String()) {
			ch = consensusCluster
		} else if strings.HasPrefix(syncCluster.String(), ch.String()) {
			ch = syncCluster
		}

		topics[engine.TopicFromChannel(ch, f.rootBlockID)] = struct{}{}
	}

	return topics
}

func (f *RoleBasedFilter) CanSubscribe(topic string) bool {
	_, allowed := f.allowedTopics(f.myPeerID)[network.Topic(topic)]
	return allowed
}

func (f *RoleBasedFilter) FilterIncomingSubscriptions(from peer.ID, opts []*pb.RPC_SubOpts) ([]*pb.RPC_SubOpts, error) {
	allowedTopics := f.allowedTopics(from)
	var filtered []*pb.RPC_SubOpts

	for _, opt := range opts {
		if _, allowed := allowedTopics[network.Topic(opt.GetTopicid())]; allowed {
			filtered = append(filtered, opt)
		}
	}

	return filtered, nil
}
