package p2p

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"sync/atomic"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/messages"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/message"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestStakedValidator tests that the staked validator prevents an unstaked node from sending messages to any staked node.
func TestStakedValidator(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	// create two staked nodes - node1 and node2
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	node1 := createNode(t, identity1.NodeID, privateKey1, sporkId)

	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	node2 := createNode(t, identity2.NodeID, privateKey2, sporkId)

	badTopic := engine.TopicFromChannel(engine.SyncCommittee, sporkId)

	ids := flow.IdentityList{identity1, identity2}
	translator, err := NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	stakedValidator := validator.StakedValidator(func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}
		return ids.ByNodeID(fid)
	})

	unstakedKey := unittest.NetworkingPrivKeyFixture()
	require.NoError(t, err)
	// create one unstaked node
	unstakedNode := createNode(t, flow.ZeroID, unstakedKey, sporkId)
	require.NoError(t, err)

	// node1 is connected to node2, and the unstaked node is connected to node1
	// unstaked Node <-> node1 <-> node2
	require.NoError(t, node1.AddPeer(context.TODO(), *host.InfoFromHost(node2.host)))
	require.NoError(t, unstakedNode.AddPeer(context.TODO(), *host.InfoFromHost(node1.host)))

	// node1 and node2 subscribe to the topic with the topic validator
	sub1, err := node1.Subscribe(badTopic, stakedValidator)
	require.NoError(t, err)
	sub2, err := node2.Subscribe(badTopic, stakedValidator)
	require.NoError(t, err)
	// the unstaked node subscribes to the topic WITHOUT the topic validator
	unstakedSub, err := unstakedNode.Subscribe(badTopic)
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(node1.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(node2.pubSub.ListPeers(badTopic.String())) > 0 &&
			len(unstakedNode.pubSub.ListPeers(badTopic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5s()

	m1 := message.Message{
		Payload: []byte("hello1"),
	}
	data1, err := m1.Marshal()
	require.NoError(t, err)

	// node2 publishes a message
	err = node2.Publish(timedCtx, badTopic, data1)
	require.NoError(t, err)

	// node1 gets the message
	checkReceive(timedCtx, t, data1, sub1, nil, true)
	// node2 also gets the message (as part of the libp2p loopback of published topic messages)
	checkReceive(timedCtx, t, data1, sub2, nil, true)
	// the unstaked node also gets the message since it subscribed to the channel without the topic validator
	checkReceive(timedCtx, t, data1, unstakedSub, nil, true)

	timedCtx, cancel2s := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2s()

	m2 := message.Message{
		Payload: []byte("hello2"),
	}
	data2, err := m2.Marshal()
	require.NoError(t, err)

	// the unstaked node now publishes a message
	err = unstakedNode.Publish(timedCtx, badTopic, data2)
	require.NoError(t, err)

	// unstaked node receives its own message
	checkReceive(timedCtx, t, data2, unstakedSub, nil, true)

	// node 1 does NOT receive the message due to the topic validator
	var wg sync.WaitGroup

	timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checkReceive(timedCtx, t, nil, sub1, &wg, false)

	// node 2 also does not receive the message via gossip from the node1 (event after the 1 second hearbeat)
	timedCtx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	checkReceive(timedCtx, t, nil, sub2, &wg, false)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")
}

// TestAuthorizedSenderValidator_Authorized tests that the authorized sender validator rejects messages from nodes that are not authorized to send the message
func TestAuthorizedSenderValidator_UnAuthorized(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn1 := createNode(t, identity1.NodeID, privateKey1, sporkId)

	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn2 := createNode(t, identity2.NodeID, privateKey2, sporkId)

	identity3, privateKey3 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	an1 := createNode(t, identity3.NodeID, privateKey3, sporkId)

	channel := engine.ConsensusCommittee
	topic := engine.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{identity1, identity2, identity3}
	translator, err := NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	// setup hooked logger
	var hookCalls uint64
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			atomic.AddUint64(&hookCalls, 1)
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.WarnLevel).Hook(hook)

	authorizedSenderValidator := validator.AuthorizedSenderValidator(logger, channel, func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}

		return ids.ByNodeID(fid)
	})

	// node1 is connected to node2, and the an1 is connected to node1
	// an1 <-> sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(context.TODO(), *host.InfoFromHost(sn2.host)))
	require.NoError(t, an1.AddPeer(context.TODO(), *host.InfoFromHost(sn1.host)))

	// sn1 and sn2 subscribe to the topic with the topic validator
	sub1, err := sn1.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	sub3, err := an1.Subscribe(topic)
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(sn1.pubSub.ListPeers(topic.String())) > 0 &&
			len(sn2.pubSub.ListPeers(topic.String())) > 0 &&
			len(an1.pubSub.ListPeers(topic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	header := unittest.BlockHeaderFixture()
	data1 := getMsgFixtureBz(t, &messages.BlockProposal{Header: &header})

	// sn2 publishes the block proposal, sn1 and an1 should receive the message because
	// SN nodes are authorized to send block proposals
	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	// sn1 gets the message
	checkReceive(timedCtx, t, data1, sub1, nil, true)

	// sn2 also gets the message (as part of the libp2p loopback of published topic messages)
	checkReceive(timedCtx, t, data1, sub2, nil, true)

	// an1 also gets the message
	checkReceive(timedCtx, t, data1, sub3, nil, true)

	timedCtx, cancel2s := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2s()
	header = unittest.BlockHeaderFixture()
	data2 := getMsgFixtureBz(t, &messages.BlockProposal{Header: &header})

	// the access node now publishes the block proposal message, AN are not authorized to publish block proposals
	// the message should be rejected by the topic validator on sn1
	err = an1.Publish(timedCtx, topic, data2)
	require.NoError(t, err)

	// an1 receives its own message
	checkReceive(timedCtx, t, data2, sub3, nil, true)

	var wg sync.WaitGroup

	// sn1 does NOT receive the message due to the topic validator
	timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checkReceive(timedCtx, t, nil, sub1, &wg, false)

	// sn2 also does not receive the message via gossip from the sn1 (event after the 1 second hearbeat)
	timedCtx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	checkReceive(timedCtx, t, nil, sub2, &wg, false)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")

	// expecting 1 warn calls for each rejected message from unauthorized node
	require.Equalf(t, uint64(1), hookCalls, "expected 1 warning to be logged")
}

// TestAuthorizedSenderValidator_Authorized tests that the authorized sender validator rejects messages being sent on the wrong channel
func TestAuthorizedSenderValidator_InvalidMsg(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn1 := createNode(t, identity1.NodeID, privateKey1, sporkId)

	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn2 := createNode(t, identity2.NodeID, privateKey2, sporkId)

	// try to publish BlockProposal on invalid SyncCommittee channel
	channel := engine.SyncCommittee
	topic := engine.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{identity1, identity2}
	translator, err := NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	// setup hooked logger
	var hookCalls uint64
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			atomic.AddUint64(&hookCalls, 1)
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.WarnLevel).Hook(hook)

	authorizedSenderValidator := validator.AuthorizedSenderValidator(logger, channel, func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}
		return ids.ByNodeID(fid)
	})

	// node1 is connected to node2
	// sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(context.TODO(), *host.InfoFromHost(sn2.host)))

	// sn1 subscribe to the topic with the topic validator, while sn2 will subscribe without the topic validator to allow sn2 to publish unauthorized messages
	sub1, err := sn1.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	_, err = sn2.Subscribe(topic)
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(sn1.pubSub.ListPeers(topic.String())) > 0 &&
			len(sn2.pubSub.ListPeers(topic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	header := unittest.BlockHeaderFixture()
	data1 := getMsgFixtureBz(t, &messages.BlockProposal{Header: &header})

	// sn2 publishes the block proposal on the sync committee channel
	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	var wg sync.WaitGroup

	// sn1 should not receive message from sn2
	timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checkReceive(timedCtx, t, nil, sub1, &wg, false)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")

	// expecting 1 warn calls for each rejected message from ejected node
	require.Equalf(t, uint64(1), hookCalls, "expected 1 warning to be logged")
}

// TestAuthorizedSenderValidator_Authorized tests that the authorized sender validator rejects messages from nodes that are ejected
func TestAuthorizedSenderValidator_Ejected(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn1 := createNode(t, identity1.NodeID, privateKey1, sporkId)

	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn2 := createNode(t, identity2.NodeID, privateKey2, sporkId)

	identity3, privateKey3 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	an1 := createNode(t, identity3.NodeID, privateKey3, sporkId)

	channel := engine.ConsensusCommittee
	topic := engine.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{identity1, identity2, identity3}
	translator, err := NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	// setup hooked logger
	var hookCalls uint64
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			atomic.AddUint64(&hookCalls, 1)
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.WarnLevel).Hook(hook)

	authorizedSenderValidator := validator.AuthorizedSenderValidator(logger, channel, func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}
		return ids.ByNodeID(fid)
	})

	// node1 is connected to node2, and the an1 is connected to node1
	// an1 <-> sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(context.TODO(), *host.InfoFromHost(sn2.host)))
	require.NoError(t, an1.AddPeer(context.TODO(), *host.InfoFromHost(sn1.host)))

	// sn1 subscribe to the topic with the topic validator, while sn2 will subscribe without the topic validator to allow sn2 to publish unauthorized messages
	sub1, err := sn1.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic)
	require.NoError(t, err)
	sub3, err := an1.Subscribe(topic)
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(sn1.pubSub.ListPeers(topic.String())) > 0 &&
			len(sn2.pubSub.ListPeers(topic.String())) > 0 &&
			len(an1.pubSub.ListPeers(topic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	header := unittest.BlockHeaderFixture()
	data1 := getMsgFixtureBz(t, &messages.BlockProposal{Header: &header})

	// sn2 publishes the block proposal, sn1 and an1 should receive the message because
	// SN nodes are authorized to send block proposals
	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	// sn1 gets the message
	checkReceive(timedCtx, t, data1, sub1, nil, true)

	// sn2 also gets the message (as part of the libp2p loopback of published topic messages)
	checkReceive(timedCtx, t, data1, sub2, nil, true)

	// an1 also gets the message
	checkReceive(timedCtx, t, data1, sub3, nil, true)

	var wg sync.WaitGroup
	// "eject" sn2 to ensure messages published by ejected nodes get rejected
	identity2.Ejected = true
	header = unittest.BlockHeaderFixture()
	data3 := getMsgFixtureBz(t, &messages.BlockProposal{Header: &header})
	timedCtx, cancel2s := context.WithTimeout(context.Background(), time.Second)
	defer cancel2s()
	err = sn2.Publish(timedCtx, topic, data3)
	require.NoError(t, err)

	// sn1 should not receive rejected message from ejected sn2
	timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checkReceive(timedCtx, t, nil, sub1, &wg, false)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")

	// expecting 1 warn calls for each rejected message from ejected node
	require.Equalf(t, uint64(1), hookCalls, "expected 1 warning to be logged")
}

// checkReceive checks that the subscription can receive the next message or not
func checkReceive(ctx context.Context, t *testing.T, expectedData []byte, sub *pubsub.Subscription, wg *sync.WaitGroup, shouldReceive bool) {
	if shouldReceive {
		// assert we can receive the next message
		msg, err := sub.Next(ctx)
		require.NoError(t, err)
		require.Equal(t, expectedData, msg.Data)
	} else {
		wg.Add(1)
		go func() {
			_, err := sub.Next(ctx)
			require.Error(t, err)
			wg.Done()
		}()
	}
}

func getMsgFixtureBz(t *testing.T, v interface{}) []byte {
	bz, err := cborcodec.NewCodec().Encode(v)
	require.NoError(t, err)

	msg := message.Message{
		Payload: bz,
	}
	data, err := msg.Marshal()
	require.NoError(t, err)

	return data
}
