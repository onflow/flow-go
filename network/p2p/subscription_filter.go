package p2p

import (
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
	myRole      *flow.Role
	rootBlockID flow.Identifier
}

func NewRoleBasedFilter(pid peer.ID, rootBlockID flow.Identifier, idProvider id.IdentityProvider) *RoleBasedFilter {
	filter := &RoleBasedFilter{
		idProvider:  idProvider,
		myPeerID:    pid,
		rootBlockID: rootBlockID,
	}
	filter.myRole = filter.getRole(pid)

	return filter
}

func (f *RoleBasedFilter) getRole(pid peer.ID) *flow.Role {
	if id, ok := f.idProvider.ByPeerID(pid); ok {
		return &id.Role
	}

	return nil
}

func (f *RoleBasedFilter) allowed(role *flow.Role, topic string) bool {
	channel, ok := engine.ChannelFromTopic(network.Topic(topic))
	if !ok {
		return false
	}

	if role == nil {
		// TODO: eventually we should have block proposals relayed on a separate
		// channel on the public network. For now, we need to make sure that
		// full observer nodes can subscribe to the block proposal channel.
		return append(engine.PublicChannels(), engine.ReceiveBlocks).Contains(channel)
	} else {
		if roles, ok := engine.RolesByChannel(channel); ok {
			return roles.Contains(*role)
		}

		return false
	}
}

func (f *RoleBasedFilter) CanSubscribe(topic string) bool {
	return f.allowed(f.myRole, topic)
}

func (f *RoleBasedFilter) FilterIncomingSubscriptions(from peer.ID, opts []*pb.RPC_SubOpts) ([]*pb.RPC_SubOpts, error) {
	role := f.getRole(from)
	var filtered []*pb.RPC_SubOpts

	for _, opt := range opts {
		if f.allowed(role, opt.GetTopicid()) {
			filtered = append(filtered, opt)
		}
	}

	return filtered, nil
}
