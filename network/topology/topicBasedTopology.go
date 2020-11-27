package topology

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
)

// TopicBasedTopology is a deterministic topology mapping that creates a connected graph component among the nodes
// involved in each topic.
type TopicBasedTopology struct {
	me      flow.Identifier             // used to keep identifier of the node
	state   protocol.ReadOnlyState      // used to keep a read only protocol state
	subMngr network.SubscriptionManager // used to keep track topics the node subscribed to
	logger  zerolog.Logger
	seed    int64
}

// NewTopicBasedTopology returns an instance of the TopicBasedTopology.
func NewTopicBasedTopology(nodeID flow.Identifier,
	logger zerolog.Logger,
	state protocol.ReadOnlyState,
	subMngr network.SubscriptionManager) (*TopicBasedTopology, error) {
	seed, err := seedFromID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("could not generate seed from id:%w", err)
	}

	t := &TopicBasedTopology{
		me:      nodeID,
		state:   state,
		seed:    seed,
		subMngr: subMngr,
		logger:  logger.With().Str("component:", "topic-based-topology").Logger(),
	}

	return t, nil
}

// GenerateFanout receives IdentityList of entire network and constructs the fanout IdentityList
// of this instance. A node directly communicates with its fanout IdentityList on epidemic dissemination
// of the messages (i.e., publish and multicast).
// Independent invocations of GenerateFanout on different nodes collaboratively must construct a cohesive
// connected graph of nodes that enables them talking to each other.
func (t TopicBasedTopology) GenerateFanout(ids flow.IdentityList) (flow.IdentityList, error) {
	myChannelIDs := t.subMngr.GetChannelIDs()
	if len(myChannelIDs) == 0 {
		// no subscribed channel id, hence skip topology creation
		// we do not return an error at this state as invocation of MakeTopology may happen before
		// node subscribing to all its channels.
		t.logger.Warn().Msg("skips generating fanout with no subscribed channels")
		return flow.IdentityList{}, nil
	}

	// finds all interacting roles with this node
	myInteractingRoles := flow.RoleList{}
	for _, myChannel := range myChannelIDs {
		roles, ok := engine.RolesByChannelID(myChannel)
		if !ok {
			return nil, fmt.Errorf("could not extract roles for channel: %s", myChannel)
		}
		myInteractingRoles = myInteractingRoles.Union(roles)
	}

	// builds a connected component per role this node interact with,
	var myFanout flow.IdentityList
	for _, role := range myInteractingRoles {
		if role == flow.RoleCollection {
			// we do not build connected component for collection nodes based on their role
			// rather we build it based on their cluster identity in the next step.
			continue
		}
		roleFanout, err := t.subsetRole(ids, nil, flow.RoleList{role})
		if err != nil {
			return nil, fmt.Errorf("failed to derive list of peer nodes to connect for role %s: %w", role, err)
		}
		myFanout = myFanout.Union(roleFanout)
	}

	// stitches the role-based components that subscribed to the same channel id together.
	for _, myChannel := range myChannelIDs {
		shouldHave := myFanout.Copy()

		topicFanout, err := t.subsetChannel(ids, shouldHave, myChannel)
		if err != nil {
			return nil, fmt.Errorf("could not generate fanout for topic %s: %w", myChannel, err)
		}
		myFanout = myFanout.Union(topicFanout)
	}

	if len(myFanout) == 0 {
		return nil, fmt.Errorf("topology size reached zero")
	}
	t.logger.Debug().
		Int("fanout", len(myFanout)).
		Msg("fanout successfully generated")
	return myFanout, nil
}

// subsetChannel returns a random subset of the identity list that is passed. `shouldHave` represents set of
// identities that should be included in the returned subset.
// Returned identities should all subscribed to the specified `channel`.
// Note: this method should not include identity of its executor.
func (t *TopicBasedTopology) subsetChannel(ids flow.IdentityList, shouldHave flow.IdentityList,
	channel string) (flow.IdentityList, error) {
	if _, ok := engine.IsClusterChannelID(channel); ok {
		return t.clusterChannelHandler(ids, shouldHave)
	}
	return t.nonClusterChannelHandler(ids, shouldHave, channel)
}

// subsetRole returns a random subset of the identity list that is passed. `shouldHave` represents set of
// identities that should be included in the returned subset.
// Returned identities should all be of one of the specified `roles`.
// Note: this method should not include identity of its executor.
func (t TopicBasedTopology) subsetRole(ids flow.IdentityList, shouldHave flow.IdentityList, roles flow.RoleList) (flow.IdentityList, error) {
	// excludes irrelevant roles and the node itself from both should have and ids set
	shouldHave = shouldHave.Filter(filter.And(
		filter.HasRole(roles...),
		filter.Not(filter.HasNodeID(t.me)),
	))

	ids = ids.Filter(filter.And(
		filter.HasRole(roles...),
		filter.Not(filter.HasNodeID(t.me)),
	))

	sample, err := t.sampleConnectedGraph(ids, shouldHave)
	if err != nil {
		return nil, fmt.Errorf("could not sample a connected graph: %w", err)
	}

	return sample, nil
}

// sampleConnectedGraph receives two lists: all and shouldHave. It then samples a connected fanout
// for the caller that includes the shouldHave set. Independent invocations of this method over
// different nodes, should create a connected graph.
// Fanout is the set of nodes that this instance should get connected to in order to create a
// connected graph.
func (t TopicBasedTopology) sampleConnectedGraph(all flow.IdentityList, shouldHave flow.IdentityList) (flow.IdentityList, error) {

	if len(all) == 0 {
		t.logger.Debug().Msg("skips sampling connected graph with zero nodes")
		return flow.IdentityList{}, nil
	}

	if len(shouldHave) == 0 {
		// choose (n+1)/2 random nodes so that each node in the graph will have a degree >= (n+1) / 2,
		// guaranteeing a connected graph.
		size := uint(LinearFanout(len(all)))
		return all.DeterministicSample(size, t.seed), nil

	}
	// checks `shouldHave` be a subset of `all`
	nonMembers := shouldHave.Filter(filter.Not(filter.In(all)))
	if len(nonMembers) != 0 {
		return nil, fmt.Errorf("should have identities is not a subset of all: %v", nonMembers)
	}

	// total sample size
	totalSize := LinearFanout(len(all))

	if totalSize < len(shouldHave) {
		// total fanout size needed is already satisfied by shouldHave set.
		return shouldHave, nil
	}

	// subset size excluding should have ones
	subsetSize := totalSize - len(shouldHave)

	// others are all excluding should have ones
	others := all.Filter(filter.Not(filter.In(shouldHave)))
	others = others.DeterministicSample(uint(subsetSize), t.seed)

	return others.Union(shouldHave), nil

}

// clusterPeers returns the list of other nodes within the same cluster as this node.
func (t TopicBasedTopology) clusterPeers() (flow.IdentityList, error) {
	currentEpoch := t.state.Final().Epochs().Current()
	clusterList, err := currentEpoch.Clustering()
	if err != nil {
		return nil, fmt.Errorf("failed to extract cluster list %w", err)
	}

	myCluster, _, found := clusterList.ByNodeID(t.me)
	if !found {
		return nil, fmt.Errorf("failed to find the cluster for node ID %s", t.me.String())
	}

	return myCluster, nil
}

// clusterChannelHandler returns a connected graph fanout of peers in the same cluster as executor of this instance.
func (t TopicBasedTopology) clusterChannelHandler(ids, shouldHave flow.IdentityList) (flow.IdentityList, error) {
	// extracts cluster peer ids to which the node belongs to.
	clusterPeers, err := t.clusterPeers()
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster peers for node %s: %w", t.me.String(), err)
	}

	// samples a connected graph topology from the cluster peers
	return t.subsetRole(clusterPeers, shouldHave.Filter(filter.HasNodeID(clusterPeers.NodeIDs()...)), flow.RoleList{flow.RoleCollection})
}

// nonClusterChannelHandler returns a connected graph fanout of peers from `ids` that subscribed to `channel`.
// The returned sample contains `shouldHave` ones that also subscribed to `channel`.
func (t TopicBasedTopology) nonClusterChannelHandler(ids, shouldHave flow.IdentityList, channel string) (flow.IdentityList, error) {
	if _, ok := engine.IsClusterChannelID(channel); ok {
		return nil, fmt.Errorf("could not handle cluster channel: %s", channel)
	}

	// extracts flow roles subscribed to topic.
	roles, ok := engine.RolesByChannelID(channel)
	if !ok {
		return nil, fmt.Errorf("unknown topic with no subscribed roles: %s", channel)
	}

	// samples a connected graph topology
	return t.subsetRole(ids.Filter(filter.HasRole(roles...)), shouldHave, roles)
}
