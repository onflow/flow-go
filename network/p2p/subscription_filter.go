package p2p

import (
	"fmt"

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

func (f *Filter) allowedTopics(pid peer.ID) map[network.Topic]struct{} {
	id := f.getIdentity(pid)

	var channels network.ChannelList

	if id == nil {
		fmt.Println("Unstaked channels allowed by " + f.myPeerID.String() + " for " + pid.String())
		channels = engine.UnstakedChannels()
	} else {
		fmt.Println("Staked channels allowed by " + f.myPeerID.String() + " for " + pid.String())
		channels = engine.ChannelsByRole(id.Role)
	}

	topics := make(map[network.Topic]struct{})

	for _, ch := range channels {
		// TODO: we will probably have problems here with cluster channels
		// We probably need special checking for this
		// Add a unit test for cluster channels
		topics[engine.TopicFromChannel(ch, f.rootBlockID)] = struct{}{}
	}

	return topics
}

func (f *Filter) CanSubscribe(topic string) bool {
	_, allowed := f.allowedTopics(f.myPeerID)[network.Topic(topic)]
	return allowed
}

func (f *Filter) FilterIncomingSubscriptions(from peer.ID, opts []*pb.RPC_SubOpts) ([]*pb.RPC_SubOpts, error) {
	allowedTopics := f.allowedTopics(from)
	var filtered []*pb.RPC_SubOpts

	for _, opt := range opts {
		fmt.Println("Request " + *opt.Topicid + "to")
		if _, allowed := allowedTopics[network.Topic(opt.GetTopicid())]; allowed {

			filtered = append(filtered, opt)
		} else {
			fmt.Println("Blocked")
		}
	}

	return filtered, nil
}
