package p2p

import (
	"github.com/libp2p/go-libp2p-core/peer"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network"
)

type Filter struct {
	idProvider   id.IdentityProvider
	idTranslator IDTranslator
	myPeerID     peer.ID
}

func (f *Filter) getIdentity(pid peer.ID) *flow.Identity {
	fid, err := f.idTranslator.GetFlowID(pid)
	if err != nil {
		// translation should always succeed for staked nodes
		return nil
	}

	identities := f.idProvider.Identities(filter.HasNodeID(fid))
	if len(identities) == 0 {
		return nil
	}

	return identities[0]
}

func (f *Filter) allowedChannels(pid peer.ID) network.ChannelList {
	id := f.getIdentity(pid)

	if id == nil {
		return engine.UnstakedChannels()
	}

	return engine.ChannelsByRole(id.Role)
}

func (f *Filter) CanSubscribe(topic string) bool {
	// TODO: check if this is correct
	channel := network.Channel(topic)

	return f.allowedChannels(f.myPeerID).Contains(channel)
}

func (f *Filter) FilterIncomingSubscriptions(from peer.ID, opts []*pb.RPC_SubOpts) ([]*pb.RPC_SubOpts, error) {
	allowedChannels := f.allowedChannels(from)
	var filtered []*pb.RPC_SubOpts

	for _, opt := range opts {
		channel := network.Channel(*opt.Topicid)
		if allowedChannels.Contains(channel) {
			filtered = append(filtered, opt)
		}
	}

	return filtered, nil
}
