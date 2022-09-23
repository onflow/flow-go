package p2p_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/network/validator"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTopicValidator_Unstaked tests that the libP2P node topic validator rejects unauthenticated messages on non-public channels (unstaked)
func TestTopicValidator_Unstaked(t *testing.T) {
	// create a hooked logger
	logger, hook := unittest.HookedLogger()

	sporkId := unittest.IdentifierFixture()

	nodeFixtureCtx, nodeFixtureCtxCancel := context.WithCancel(context.Background())
	defer nodeFixtureCtxCancel()

	sn1, identity1 := nodeFixture(t, nodeFixtureCtx, sporkId, t.Name(), withRole(flow.RoleConsensus), withLogger(logger))
	sn2, identity2 := nodeFixture(t, nodeFixtureCtx, sporkId, t.Name(), withRole(flow.RoleConsensus), withLogger(logger))
	defer stopNodes(t, []*p2p.Node{sn1, sn2})

	channel := channels.ConsensusCommittee
	topic := channels.TopicFromChannel(channel, sporkId)

	//NOTE: identity2 is not in the ids list simulating an un-staked node
	ids := flow.IdentityList{&identity1}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	// peer filter used by the topic validator to check if node is staked
	isStaked := func(pid peer.ID) error {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return fmt.Errorf("could not translate the peer_id %s to a Flow identifier: %w", pid.Pretty(), err)
		}

		if _, ok := ids.ByNodeID(fid); !ok {
			return fmt.Errorf("flow id not found: %x", fid)
		}

		return nil
	}

	pInfo2, err := p2p.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2
	// sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(context.TODO(), pInfo2))

	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(logger, metrics.NewNoopCollector())
	// sn1 will subscribe with is staked callback that should force the TopicValidator to drop the message received from sn2
	sub1, err := sn1.Subscribe(topic, unittest.NetworkCodec(), isStaked, slashingViolationsConsumer)
	require.NoError(t, err)

	// sn2 will subscribe with an unauthenticated callback to allow it to send the unauthenticated message
	_, err = sn2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	header := unittest.BlockHeaderFixture()
	data1 := getMsgFixtureBz(t, &messages.BlockProposal{Header: header})

	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	var wg sync.WaitGroup

	// sn1 should not receive message from sn2 because sn2 is unstaked
	timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checkReceive(timedCtx, t, nil, sub1, &wg, false)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")

	// ensure the correct error is contained in the logged error
	require.Contains(t, hook.Logs(), "filtering message from un-allowed peer")
}

// TestTopicValidator_PublicChannel tests that the libP2P node topic validator does not reject unauthenticated messages on public channels
func TestTopicValidator_PublicChannel(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	logger := unittest.Logger()

	nodeFixtureCtx, nodeFixtureCtxCancel := context.WithCancel(context.Background())
	defer nodeFixtureCtxCancel()

	sn1, _ := nodeFixture(t, nodeFixtureCtx, sporkId, t.Name(), withRole(flow.RoleConsensus), withLogger(logger))
	sn2, identity2 := nodeFixture(t, nodeFixtureCtx, sporkId, t.Name(), withRole(flow.RoleConsensus), withLogger(logger))
	defer stopNodes(t, []*p2p.Node{sn1, sn2})

	// unauthenticated messages should not be dropped on public channels
	channel := channels.PublicSyncCommittee
	topic := channels.TopicFromChannel(channel, sporkId)

	pInfo2, err := p2p.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2
	// sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(context.TODO(), pInfo2))

	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(logger, metrics.NewNoopCollector())
	// sn1 & sn2 will subscribe with unauthenticated callback to allow it to send and receive unauthenticated messages
	sub1, err := sn1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

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
}

// TestAuthorizedSenderValidator_Unauthorized tests that the authorized sender validator rejects messages from nodes that are not authorized to send the message
func TestAuthorizedSenderValidator_Unauthorized(t *testing.T) {
	// create a hooked logger
	logger, hook := unittest.HookedLogger()

	sporkId := unittest.IdentifierFixture()

	nodeFixtureCtx, nodeFixtureCtxCancel := context.WithCancel(context.Background())
	defer nodeFixtureCtxCancel()

	sn1, identity1 := nodeFixture(t, nodeFixtureCtx, sporkId, t.Name(), withRole(flow.RoleConsensus))
	sn2, identity2 := nodeFixture(t, nodeFixtureCtx, sporkId, t.Name(), withRole(flow.RoleConsensus))
	an1, identity3 := nodeFixture(t, nodeFixtureCtx, sporkId, t.Name(), withRole(flow.RoleAccess))
	defer stopNodes(t, []*p2p.Node{sn1, sn2, an1})

	channel := channels.ConsensusCommittee
	topic := channels.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2, &identity3}

	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	violationsConsumer := slashing.NewSlashingViolationsConsumer(logger, metrics.NewNoopCollector())
	getIdentity := func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}

		return ids.ByNodeID(fid)
	}
	authorizedSenderValidator := validator.NewAuthorizedSenderValidator(logger, violationsConsumer, getIdentity)
	pubsubMessageValidator := authorizedSenderValidator.PubSubMessageValidator(channel)

	pInfo1, err := p2p.PeerAddressInfo(identity1)
	require.NoError(t, err)

	pInfo2, err := p2p.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2, and the an1 is connected to node1
	// an1 <-> sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(context.TODO(), pInfo2))
	require.NoError(t, an1.AddPeer(context.TODO(), pInfo1))

	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(logger, metrics.NewNoopCollector())
	// sn1 and sn2 subscribe to the topic with the topic validator
	sub1, err := sn1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer, pubsubMessageValidator)
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer, pubsubMessageValidator)
	require.NoError(t, err)
	sub3, err := an1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	header := unittest.BlockHeaderFixture()
	data1 := getMsgFixtureBz(t, &messages.BlockProposal{Header: header})

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
	data2 := getMsgFixtureBz(t, &messages.BlockProposal{Header: header})

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

	// ensure the correct error is contained in the logged error
	require.Contains(t, hook.Logs(), message.ErrUnauthorizedRole.Error())
}

// TestAuthorizedSenderValidator_Authorized tests that the authorized sender validator rejects messages being sent on the wrong channel
func TestAuthorizedSenderValidator_InvalidMsg(t *testing.T) {
	// create a hooked logger
	logger, hook := unittest.HookedLogger()

	sporkId := unittest.IdentifierFixture()

	nodeFixtureCtx, nodeFixtureCtxCancel := context.WithCancel(context.Background())
	defer nodeFixtureCtxCancel()

	sn1, identity1 := nodeFixture(t, nodeFixtureCtx, sporkId, "consensus_1", withRole(flow.RoleConsensus))
	sn2, identity2 := nodeFixture(t, nodeFixtureCtx, sporkId, "consensus_2", withRole(flow.RoleConsensus))
	defer stopNodes(t, []*p2p.Node{sn1, sn2})

	// try to publish BlockProposal on invalid SyncCommittee channel
	channel := channels.SyncCommittee
	topic := channels.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	violationsConsumer := slashing.NewSlashingViolationsConsumer(logger, metrics.NewNoopCollector())
	getIdentity := func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}

		return ids.ByNodeID(fid)
	}
	authorizedSenderValidator := validator.NewAuthorizedSenderValidator(logger, violationsConsumer, getIdentity)
	pubsubMessageValidator := authorizedSenderValidator.PubSubMessageValidator(channel)

	pInfo2, err := p2p.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2
	// sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(context.TODO(), pInfo2))

	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(logger, metrics.NewNoopCollector())
	// sn1 subscribe to the topic with the topic validator, while sn2 will subscribe without the topic validator to allow sn2 to publish unauthorized messages
	sub1, err := sn1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer, pubsubMessageValidator)
	require.NoError(t, err)
	_, err = sn2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	header := unittest.BlockHeaderFixture()
	data1 := getMsgFixtureBz(t, &messages.BlockProposal{Header: header})

	// sn2 publishes the block proposal on the sync committee channel
	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	var wg sync.WaitGroup

	// sn1 should not receive message from sn2
	timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checkReceive(timedCtx, t, nil, sub1, &wg, false)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")

	// ensure the correct error is contained in the logged error
	require.Contains(t, hook.Logs(), message.ErrUnauthorizedMessageOnChannel.Error())
}

// TestAuthorizedSenderValidator_Ejected tests that the authorized sender validator rejects messages from nodes that are ejected
func TestAuthorizedSenderValidator_Ejected(t *testing.T) {
	// create a hooked logger
	logger, hook := unittest.HookedLogger()

	sporkId := unittest.IdentifierFixture()

	nodeFixtureCtx, nodeFixtureCtxCancel := context.WithCancel(context.Background())
	defer nodeFixtureCtxCancel()

	sn1, identity1 := nodeFixture(t, nodeFixtureCtx, sporkId, "consensus_1", withRole(flow.RoleConsensus))
	sn2, identity2 := nodeFixture(t, nodeFixtureCtx, sporkId, "consensus_2", withRole(flow.RoleConsensus))
	an1, identity3 := nodeFixture(t, nodeFixtureCtx, sporkId, "access_1", withRole(flow.RoleAccess))
	defer stopNodes(t, []*p2p.Node{sn1, sn2, an1})

	channel := channels.ConsensusCommittee
	topic := channels.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2, &identity3}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	violationsConsumer := slashing.NewSlashingViolationsConsumer(logger, metrics.NewNoopCollector())
	getIdentity := func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}

		return ids.ByNodeID(fid)
	}
	authorizedSenderValidator := validator.NewAuthorizedSenderValidator(logger, violationsConsumer, getIdentity)
	pubsubMessageValidator := authorizedSenderValidator.PubSubMessageValidator(channel)

	pInfo1, err := p2p.PeerAddressInfo(identity1)
	require.NoError(t, err)

	pInfo2, err := p2p.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2, and the an1 is connected to node1
	// an1 <-> sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(context.TODO(), pInfo2))
	require.NoError(t, an1.AddPeer(context.TODO(), pInfo1))

	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(logger, metrics.NewNoopCollector())
	// sn1 subscribe to the topic with the topic validator, while sn2 will subscribe without the topic validator to allow sn2 to publish unauthorized messages
	sub1, err := sn1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer, pubsubMessageValidator)
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)
	sub3, err := an1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer)
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	header := unittest.BlockHeaderFixture()
	data1 := getMsgFixtureBz(t, &messages.BlockProposal{Header: header})

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
	data3 := getMsgFixtureBz(t, &messages.BlockProposal{Header: header})
	timedCtx, cancel2s := context.WithTimeout(context.Background(), time.Second)
	defer cancel2s()
	err = sn2.Publish(timedCtx, topic, data3)
	require.NoError(t, err)

	// sn1 should not receive rejected message from ejected sn2
	timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checkReceive(timedCtx, t, nil, sub1, &wg, false)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")

	// ensure the correct error is contained in the logged error
	require.Contains(t, hook.Logs(), validator.ErrSenderEjected.Error())
}

// TestAuthorizedSenderValidator_ClusterChannel tests that the authorized sender validator correctly validates messages sent on cluster channels
func TestAuthorizedSenderValidator_ClusterChannel(t *testing.T) {
	sporkId := unittest.IdentifierFixture()

	nodeFixtureCtx, nodeFixtureCtxCancel := context.WithCancel(context.Background())
	defer nodeFixtureCtxCancel()

	ln1, identity1 := nodeFixture(t, nodeFixtureCtx, sporkId, "collection_1", withRole(flow.RoleCollection))
	ln2, identity2 := nodeFixture(t, nodeFixtureCtx, sporkId, "collection_2", withRole(flow.RoleCollection))
	ln3, identity3 := nodeFixture(t, nodeFixtureCtx, sporkId, "collection_3", withRole(flow.RoleCollection))
	defer stopNodes(t, []*p2p.Node{ln1, ln2, ln3})

	channel := channels.SyncCluster(flow.Testnet)
	topic := channels.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2, &identity3}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	logger := unittest.Logger()
	violationsConsumer := slashing.NewSlashingViolationsConsumer(logger, metrics.NewNoopCollector())
	getIdentity := func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}

		return ids.ByNodeID(fid)
	}
	authorizedSenderValidator := validator.NewAuthorizedSenderValidator(logger, violationsConsumer, getIdentity)
	pubsubMessageValidator := authorizedSenderValidator.PubSubMessageValidator(channel)

	pInfo1, err := p2p.PeerAddressInfo(identity1)
	require.NoError(t, err)

	pInfo2, err := p2p.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// ln3 <-> sn1 <-> sn2
	require.NoError(t, ln1.AddPeer(context.TODO(), pInfo2))
	require.NoError(t, ln3.AddPeer(context.TODO(), pInfo1))

	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(logger, metrics.NewNoopCollector())
	sub1, err := ln1.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer, pubsubMessageValidator)
	require.NoError(t, err)
	sub2, err := ln2.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer, pubsubMessageValidator)
	require.NoError(t, err)
	sub3, err := ln3.Subscribe(topic, unittest.NetworkCodec(), unittest.AllowAllPeerFilter(), slashingViolationsConsumer, pubsubMessageValidator)
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

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
