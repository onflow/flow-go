package p2p_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAuthorizedSenderValidator_Unauthorized tests that the authorized sender validator rejects messages from nodes that are not authorized to send the message
func TestAuthorizedSenderValidator_Unauthorized(t *testing.T) {
	sporkId := unittest.IdentifierFixture()

	sn1, identity1 := nodeFixture(t, context.Background(), sporkId, "consensus_1", withRole(flow.RoleConsensus))
	sn2, identity2 := nodeFixture(t, context.Background(), sporkId, "consensus_2", withRole(flow.RoleConsensus))
	an1, identity3 := nodeFixture(t, context.Background(), sporkId, "access_1", withRole(flow.RoleAccess))

	channel := engine.ConsensusCommittee
	topic := engine.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2, &identity3}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	// setup hooked logger
	logger, hook := unittest.HookedLogger()

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
	sub1, err := sn1.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	sub3, err := an1.Subscribe(topic)
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

	// expecting ErrUnauthorized to be logged
	require.Regexp(t, validator.ErrUnauthorized, hook.Logs())
}

// TestAuthorizedSenderValidator_Authorized tests that the authorized sender validator rejects messages being sent on the wrong channel
func TestAuthorizedSenderValidator_InvalidMsg(t *testing.T) {
	sporkId := unittest.IdentifierFixture()

	sn1, identity1 := nodeFixture(t, context.Background(), sporkId, "consensus_1", withRole(flow.RoleConsensus))
	sn2, identity2 := nodeFixture(t, context.Background(), sporkId, "consensus_2", withRole(flow.RoleConsensus))

	// try to publish BlockProposal on invalid SyncCommittee channel
	channel := engine.SyncCommittee
	topic := engine.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	// setup hooked logger
	var (
		hook   unittest.LoggerHook
		logger zerolog.Logger
	)
	logger, hook = unittest.HookedLogger()

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
	sub1, err := sn1.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	_, err = sn2.Subscribe(topic)
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

	// expecting ErrInvalidMsgOnChannel to be logged
	require.Regexp(t, validator.ErrInvalidMsgOnChannel, hook.Logs())
}

// TestAuthorizedSenderValidator_Authorized tests that the authorized sender validator rejects messages from unstaked nodes
func TestAuthorizedSenderValidator_Unstaked(t *testing.T) {
	sporkId := unittest.IdentifierFixture()

	sn1, identity1 := nodeFixture(t, context.Background(), sporkId, "consensus_1", withRole(flow.RoleConsensus))
	sn2, _ := nodeFixture(t, context.Background(), sporkId, "consensus_2", withRole(flow.RoleConsensus))

	channel := engine.ConsensusCommittee
	topic := engine.TopicFromChannel(channel, sporkId)

	//NOTE: identity2 is not in the ids list simulating an un-staked node
	ids := flow.IdentityList{&identity1}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	// setup hooked logger
	logger, hook := unittest.HookedLogger()

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
	sub1, err := sn1.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	_, err = sn2.Subscribe(topic)
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

	// expecting ErrIdentityUnverified to be logged
	require.Regexp(t, validator.ErrIdentityUnverified, hook.Logs())
}

// TestAuthorizedSenderValidator_Ejected tests that the authorized sender validator rejects messages from nodes that are ejected
func TestAuthorizedSenderValidator_Ejected(t *testing.T) {
	sporkId := unittest.IdentifierFixture()

	sn1, identity1 := nodeFixture(t, context.Background(), sporkId, "consensus_1", withRole(flow.RoleConsensus))
	sn2, identity2 := nodeFixture(t, context.Background(), sporkId, "consensus_2", withRole(flow.RoleConsensus))
	an1, identity3 := nodeFixture(t, context.Background(), sporkId, "access_1", withRole(flow.RoleAccess))

	channel := engine.ConsensusCommittee
	topic := engine.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2, &identity3}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	// setup hooked logger
	logger, hook := unittest.HookedLogger()

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
	sub1, err := sn1.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic)
	require.NoError(t, err)
	sub3, err := an1.Subscribe(topic)
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

	// expecting ErrSenderEjected to be logged
	require.Regexp(t, validator.ErrSenderEjected, hook.Logs())
}

// TestAuthorizedSenderValidator_ReceiveOnly tests that the authorized sender validator rejects messages from nodes that should only receive on the channel
func TestAuthorizedSenderValidator_ReceiveOnly(t *testing.T) {
	sporkId := unittest.IdentifierFixture()

	sn1, identity1 := nodeFixture(t, context.Background(), sporkId, "consensus_1", withRole(flow.RoleConsensus))
	an1, identity2 := nodeFixture(t, context.Background(), sporkId, "access_1", withRole(flow.RoleAccess))

	// AN's should not be able to send on the push blocks channel
	channel := engine.PushBlocks
	topic := engine.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2}
	translator, err := p2p.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	// setup hooked logger
	var (
		hook   unittest.LoggerHook
		logger zerolog.Logger
	)
	logger, hook = unittest.HookedLogger()

	authorizedSenderValidator := validator.AuthorizedSenderValidator(logger, channel, func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translator.GetFlowID(pid)
		if err != nil {
			return nil, false
		}
		return ids.ByNodeID(fid)
	})

	require.NoError(t, an1.AddPeer(context.TODO(), *host.InfoFromHost(sn1.Host())))

	// sn1 subscribe to the topic with the topic validator, while an1 will subscribe without the topic validator to allow an1 to publish unauthorized messages
	sub1, err := sn1.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	_, err = an1.Subscribe(topic)
	require.NoError(t, err)

	// assert that the nodes are connected as expected
	require.Eventually(t, func() bool {
		return len(sn1.ListPeers(topic.String())) > 0 &&
			len(an1.ListPeers(topic.String())) > 0
	}, 3*time.Second, 100*time.Millisecond)

	var wg sync.WaitGroup
	header := unittest.BlockHeaderFixture()
	data := getMsgFixtureBz(t, &messages.BlockProposal{Header: &header})
	timedCtx, cancel2s := context.WithTimeout(context.Background(), time.Second)
	defer cancel2s()
	err = an1.Publish(timedCtx, topic, data)
	require.NoError(t, err)

	// sn1 should not receive rejected message from ejected sn2
	timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	checkReceive(timedCtx, t, nil, sub1, &wg, false)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")

	// expecting ErrReceiveOnly to be logged
	require.Regexp(t, validator.ErrReceiveOnly, hook.Logs())
}

// TestAuthorizedSenderValidator_ClusterChannel tests that the authorized sender validator correctly validates messages sent on cluster channels
func TestAuthorizedSenderValidator_ClusterChannel(t *testing.T) {
	sporkId := unittest.IdentifierFixture()

	ln1, identity1 := nodeFixture(t, context.Background(), sporkId, "collection_1", withRole(flow.RoleCollection))
	ln2, identity2 := nodeFixture(t, context.Background(), sporkId, "collection_2", withRole(flow.RoleCollection))
	ln3, identity3 := nodeFixture(t, context.Background(), sporkId, "collection_3", withRole(flow.RoleCollection))

	channel := engine.ChannelSyncCluster(flow.Testnet)
	topic := engine.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2, &identity3}
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

	sub1, err := ln1.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	sub2, err := ln2.Subscribe(topic, authorizedSenderValidator)
	require.NoError(t, err)
	sub3, err := ln3.Subscribe(topic, authorizedSenderValidator)
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
	data := getMsgFixtureBz(t, &messages.SyncRequest{})

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
	bz, err := cborcodec.NewCodec().Encode(v)
	require.NoError(t, err)

	msg := message.Message{
		Payload: bz,
	}
	data, err := msg.Marshal()
	require.NoError(t, err)

	return data
}
