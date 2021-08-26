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
	rootBlockID  flow.Identifier
}

func NewSubscriptionFilter(pid peer.ID, rootBlockID flow.Identifier, idProvider id.IdentityProvider, idTranslator IDTranslator) *Filter {
	return &Filter{
		idProvider,
		idTranslator,
		pid,
		rootBlockID,
	}
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

func (f *Filter) allowedTopics(pid peer.ID) []network.Topic {
	id := f.getIdentity(pid)

	var channels network.ChannelList

	if id == nil {
		channels = engine.UnstakedChannels()
	} else {
		channels = engine.ChannelsByRole(id.Role)
	}

	var topics []network.Topic

	for _, ch := range channels {
		topics = append(topics, engine.TopicFromChannel(ch, f.rootBlockID))
	}

	return topics
}

func (f *Filter) CanSubscribe(topic string) bool {
	for _, allowedTopic := range f.allowedTopics(f.myPeerID) {
		if topic == allowedTopic.String() {
			return true
		}
	}

	return false
}

func (f *Filter) FilterIncomingSubscriptions(from peer.ID, opts []*pb.RPC_SubOpts) ([]*pb.RPC_SubOpts, error) {
	allowedTopics := f.allowedTopics(from)
	var filtered []*pb.RPC_SubOpts

	for _, opt := range opts {
		for _, allowedTopic := range allowedTopics {
			if *opt.Topicid == allowedTopic.String() {
				filtered = append(filtered, opt)
			}
		}
	}

	return filtered, nil
}
