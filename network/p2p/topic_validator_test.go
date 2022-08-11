package p2p_test

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTopicValidator_Unstaked tests that the libP2P node topic validator rejects unauthenticated messages on non-public channels (unstaked)
func TestTopicValidator_Unstaked(t *testing.T) {
	// setup hooked logger
	var hookCalls uint64
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			atomic.AddUint64(&hookCalls, 1)
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.WarnLevel).Hook(hook)

	sporkId := unittest.IdentifierFixture()
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn1 := createNode(t, identity1.NodeID, privateKey1, sporkId, logger)

	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn2 := createNode(t, identity2.NodeID, privateKey2, sporkId, zerolog.Nop())

	channel := network.ConsensusCommittee
	topic := network.TopicFromChannel(channel, sporkId)

	//NOTE: identity2 is not in the ids list simulating an un-staked node
	ids := flow.IdentityList{identity1}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	// callback used by the topic validator to check if node is staked
	isStaked := func(pid peer.ID) bool {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return false
		}
		_, ok := ids.ByNodeID(fid)
		return ok
	}

	// node1 is connected to node2
	// sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(context.TODO(), *host.InfoFromHost(sn2.Host())))

	// sn1 will subscribe with is staked callback that should force the TopicValidator to drop the message received from sn2
	sub1, err := sn1.Subscribe(topic, unittest.NetworkCodec(), isStaked)
	require.NoError(t, err)

	// sn2 will subscribe with an unauthenticated callback to allow it to send the unauthenticated message
	_, err = sn2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter())
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(sn1.ListPeers(topic.String())) > 0 &&
			len(sn2.ListPeers(topic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	header := unittest.BlockHeaderFixture()
	data1 := getMsgFixtureBz(t, &messages.BlockProposal{Header: &header})

	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	var wg sync.WaitGroup

	// sn1 should not receive message from sn2 because sn2 is unstaked
	timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checkReceive(timedCtx, t, nil, sub1, &wg, false)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")

	// expecting 1 warn calls for each rejected message from unauthenticated node
	require.Equalf(t, uint64(1), hookCalls, "expected 1 warning to be logged")
}

// TestTopicValidator_PublicChannel tests that the libP2P node topic validator does not reject unauthenticated messages on public channels
func TestTopicValidator_PublicChannel(t *testing.T) {
	// setup hooked logger
	var hookCalls uint64
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			atomic.AddUint64(&hookCalls, 1)
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.WarnLevel).Hook(hook)

	sporkId := unittest.IdentifierFixture()
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn1 := createNode(t, identity1.NodeID, privateKey1, sporkId, logger)

	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn2 := createNode(t, identity2.NodeID, privateKey2, sporkId, zerolog.Nop())

	// unauthenticated messages should not be dropped on public channels
	channel := network.PublicSyncCommittee
	topic := network.TopicFromChannel(channel, sporkId)

	// node1 is connected to node2
	// sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(context.TODO(), *host.InfoFromHost(sn2.Host())))

	// sn1 & sn2 will subscribe with unauthenticated callback to allow it to send and receive unauthenticated messages
	sub1, err := sn1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter())
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter())
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(sn1.ListPeers(topic.String())) > 0 &&
			len(sn2.ListPeers(topic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5s()
	// create a dummy sync request to publish from our SN node
	data1 := getMsgFixtureBz(t, &messages.SyncRequest{Nonce: 0, Height: 0})

	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	var wg sync.WaitGroup

	// sn1 should receive message from sn2 because the public channel is unauthenticated
	timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// sn1 gets the message
	checkReceive(timedCtx, t, data1, sub1, nil, true)

	// sn2 also gets the message (as part of the libp2p loopback of published topic messages)
	checkReceive(timedCtx, t, data1, sub2, nil, true)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")

	// expecting no warn calls for rejected messages
	require.Equalf(t, uint64(0), hookCalls, "expected 0 warning to be logged")
}

// TestAuthorizedSenderValidator_Unauthorized tests that the authorized sender validator rejects messages from nodes that are not authorized to send the message
func TestAuthorizedSenderValidator_Unauthorized(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn1 := createNode(t, identity1.NodeID, privateKey1, sporkId, zerolog.Nop())

	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn2 := createNode(t, identity2.NodeID, privateKey2, sporkId, zerolog.Nop())

	identity3, privateKey3 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	an1 := createNode(t, identity3.NodeID, privateKey3, sporkId, zerolog.Nop())

	channel := network.ConsensusCommittee
	topic := network.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{identity1, identity2, identity3}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
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
	require.NoError(t, sn1.AddPeer(context.TODO(), *host.InfoFromHost(sn2.Host())))
	require.NoError(t, an1.AddPeer(context.TODO(), *host.InfoFromHost(sn1.Host())))

	// sn1 and sn2 subscribe to the topic with the topic validator
	sub1, err := sn1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), authorizedSenderValidator)
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), authorizedSenderValidator)
	require.NoError(t, err)
	sub3, err := an1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter())
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(sn1.ListPeers(topic.String())) > 0 &&
			len(sn2.ListPeers(topic.String())) > 0 &&
			len(an1.ListPeers(topic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
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
	sn1 := createNode(t, identity1.NodeID, privateKey1, sporkId, zerolog.Nop())

	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn2 := createNode(t, identity2.NodeID, privateKey2, sporkId, zerolog.Nop())

	// try to publish BlockProposal on invalid SyncCommittee channel
	channel := network.SyncCommittee
	topic := network.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{identity1, identity2}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
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
	require.NoError(t, sn1.AddPeer(context.TODO(), *host.InfoFromHost(sn2.Host())))

	// sn1 subscribe to the topic with the topic validator, while sn2 will subscribe without the topic validator to allow sn2 to publish unauthorized messages
	sub1, err := sn1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), authorizedSenderValidator)
	require.NoError(t, err)
	_, err = sn2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter())
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(sn1.ListPeers(topic.String())) > 0 &&
			len(sn2.ListPeers(topic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
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

// TestAuthorizedSenderValidator_Ejected tests that the authorized sender validator rejects messages from nodes that are ejected
func TestAuthorizedSenderValidator_Ejected(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn1 := createNode(t, identity1.NodeID, privateKey1, sporkId, zerolog.Nop())

	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
	sn2 := createNode(t, identity2.NodeID, privateKey2, sporkId, zerolog.Nop())

	identity3, privateKey3 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleAccess))
	an1 := createNode(t, identity3.NodeID, privateKey3, sporkId, zerolog.Nop())

	channel := network.ConsensusCommittee
	topic := network.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{identity1, identity2, identity3}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
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
	require.NoError(t, sn1.AddPeer(context.TODO(), *host.InfoFromHost(sn2.Host())))
	require.NoError(t, an1.AddPeer(context.TODO(), *host.InfoFromHost(sn1.Host())))

	// sn1 subscribe to the topic with the topic validator, while sn2 will subscribe without the topic validator to allow sn2 to publish unauthorized messages
	sub1, err := sn1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), authorizedSenderValidator)
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter())
	require.NoError(t, err)
	sub3, err := an1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter())
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(sn1.ListPeers(topic.String())) > 0 &&
			len(sn2.ListPeers(topic.String())) > 0 &&
			len(an1.ListPeers(topic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
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

// TestAuthorizedSenderValidator_ClusterChannel tests that the authorized sender validator correctly validates messages sent on cluster channels
func TestAuthorizedSenderValidator_ClusterChannel(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	identity1, privateKey1 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleCollection))
	ln1 := createNode(t, identity1.NodeID, privateKey1, sporkId, zerolog.Nop())

	identity2, privateKey2 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleCollection))
	ln2 := createNode(t, identity2.NodeID, privateKey2, sporkId, zerolog.Nop())

	identity3, privateKey3 := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleCollection))
	ln3 := createNode(t, identity3.NodeID, privateKey3, sporkId, zerolog.Nop())

	channel := network.ChannelSyncCluster(flow.Testnet)
	topic := network.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{identity1, identity2}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	authorizedSenderValidator := validator.AuthorizedSenderValidator(zerolog.Nop(), channel, func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}
		return ids.ByNodeID(fid)
	})

	// ln3 <-> sn1 <-> sn2
	require.NoError(t, ln1.AddPeer(context.TODO(), *host.InfoFromHost(ln2.Host())))
	require.NoError(t, ln3.AddPeer(context.TODO(), *host.InfoFromHost(ln1.Host())))

	sub1, err := ln1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), authorizedSenderValidator)
	require.NoError(t, err)
	sub2, err := ln2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), authorizedSenderValidator)
	require.NoError(t, err)
	sub3, err := ln3.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), authorizedSenderValidator)
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(ln1.ListPeers(topic.String())) > 0 &&
			len(ln2.ListPeers(topic.String())) > 0 &&
			len(ln3.ListPeers(topic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5s()
	// create a dummy sync request to publish from our LN node
	data := getMsgFixtureBz(t, &messages.RangeRequest{})

	// ln2 publishes the sync request on the cluster channel
	err = ln2.Publish(timedCtx, topic, data)
	require.NoError(t, err)

	// ln1 gets the message
	checkReceive(timedCtx, t, data, sub1, nil, true)

	// ln2 also gets the message (as part of the libp2p loopback of published topic messages)
	checkReceive(timedCtx, t, data, sub2, nil, true)

	// ln3 also gets the message
	checkReceive(timedCtx, t, data, sub3, nil, true)
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
			require.ErrorIs(t, err, context.DeadlineExceeded)
			wg.Done()
		}()
	}
}

func getMsgFixtureBz(t *testing.T, v interface{}) []byte {
	bz, err := unittest.NetworkCodec().Encode(v)
	require.NoError(t, err)

	msg := message.Message{
		Payload: bz,
	}
	data, err := msg.Marshal()
	require.NoError(t, err)

	return data
}
