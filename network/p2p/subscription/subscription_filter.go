package subscription

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module"
	p2putils "github.com/onflow/flow-go/network/p2p/utils"

	"github.com/onflow/flow-go/model/flow"
)

// RoleBasedFilter implements a subscription filter that filters subscriptions based on a node's role.
type RoleBasedFilter struct {
	idProvider module.IdentityProvider
	myRole     flow.Role
}

const UnstakedRole = flow.Role(0)

var _ pubsub.SubscriptionFilter = (*RoleBasedFilter)(nil)

func NewRoleBasedFilter(role flow.Role, idProvider module.IdentityProvider) *RoleBasedFilter {
	filter := &RoleBasedFilter{
		idProvider: idProvider,
		myRole:     role,
	}

	return filter
}

func (f *RoleBasedFilter) getRole(pid peer.ID) flow.Role {
	if flowId, ok := f.idProvider.ByPeerID(pid); ok {
		return flowId.Role
	}

	return UnstakedRole
}

func (f *RoleBasedFilter) CanSubscribe(topic string) bool {
	return p2putils.AllowedSubscription(f.myRole, topic)
}

func (f *RoleBasedFilter) FilterIncomingSubscriptions(from peer.ID, opts []*pb.RPC_SubOpts) ([]*pb.RPC_SubOpts, error) {
	role := f.getRole(from)
	var filtered []*pb.RPC_SubOpts

	for _, opt := range opts {
		if p2putils.AllowedSubscription(role, opt.GetTopicid()) {
			filtered = append(filtered, opt)
		}
	}

	return filtered, nil
}
