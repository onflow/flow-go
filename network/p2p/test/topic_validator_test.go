package p2ptest_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/alsp"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/network/validator"
	flowpubsub "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTopicValidator_Unstaked tests that the libP2P node topic validator rejects unauthenticated messages on non-public channels (unstaked)
func TestTopicValidator_Unstaked(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := mockmodule.NewIdentityProvider(t)
	// create a hooked logger
	logger, hook := unittest.HookedLogger()

	sporkId := unittest.IdentifierFixture()

	sn1, identity1 := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus), p2ptest.WithLogger(logger))
	sn2, identity2 := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus), p2ptest.WithLogger(logger))
	idProvider.On("ByPeerID", sn1.Host().ID()).Return(&identity1, true).Maybe()
	idProvider.On("ByPeerID", sn2.Host().ID()).Return(&identity2, true).Maybe()
	nodes := []p2p.LibP2PNode{sn1, sn2}
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	channel := channels.ConsensusCommittee
	topic := channels.TopicFromChannel(channel, sporkId)

	//NOTE: identity2 is not in the ids list simulating an un-staked node
	ids := flow.IdentityList{&identity1}
	translatorFixture, err := translator.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	// peer filter used by the topic validator to check if node is staked
	isStaked := func(pid peer.ID) error {
		fid, err := translatorFixture.GetFlowID(pid)
		if err != nil {
			return fmt.Errorf("could not translate the peer_id %s to a Flow identifier: %w", pid.String(), err)
		}

		if _, ok := ids.ByNodeID(fid); !ok {
			return fmt.Errorf("flow id not found: %x", fid)
		}

		return nil
	}

	pInfo2, err := utils.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2
	// sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(ctx, pInfo2))

	// sn1 will subscribe with is staked callback that should force the TopicValidator to drop the message received from sn2
	sub1, err := sn1.Subscribe(topic, flowpubsub.TopicValidator(logger, isStaked))
	require.NoError(t, err)

	// sn2 will subscribe with an unauthenticated callback to allow it to send the unauthenticated message
	_, err = sn2.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(ctx, 5*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	data1 := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture(), channel)

	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	// sn1 should not receive message from sn2 because sn2 is unstaked
	timedCtx, cancel1s := context.WithTimeout(ctx, time.Second)
	defer cancel1s()
	p2pfixtures.SubMustNeverReceiveAnyMessage(t, timedCtx, sub1)

	// ensure the correct error is contained in the logged error
	require.Contains(t, hook.Logs(), "filtering message from un-allowed peer")
}

// TestTopicValidator_PublicChannel tests that the libP2P node topic validator does not reject unauthenticated messages on public channels
func TestTopicValidator_PublicChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := mockmodule.NewIdentityProvider(t)
	sporkId := unittest.IdentifierFixture()
	logger := unittest.Logger()

	sn1, identity1 := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus), p2ptest.WithLogger(logger))
	sn2, identity2 := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus), p2ptest.WithLogger(logger))
	idProvider.On("ByPeerID", sn1.Host().ID()).Return(&identity1, true).Maybe()
	idProvider.On("ByPeerID", sn2.Host().ID()).Return(&identity2, true).Maybe()
	nodes := []p2p.LibP2PNode{sn1, sn2}
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	// unauthenticated messages should not be dropped on public channels
	channel := channels.PublicSyncCommittee
	topic := channels.TopicFromChannel(channel, sporkId)

	pInfo2, err := utils.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2
	// sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(ctx, pInfo2))

	// sn1 & sn2 will subscribe with unauthenticated callback to allow it to send and receive unauthenticated messages
	sub1, err := sn1.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(ctx, 5*time.Second)
	defer cancel5s()
	// create a dummy sync request to publish from our SN node
	data1 := p2pfixtures.MustEncodeEvent(t, &messages.SyncRequest{Nonce: 0, Height: 0}, channel)

	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	var wg sync.WaitGroup

	// sn1 should receive message from sn2 because the public channel is unauthenticated
	timedCtx, cancel1s := context.WithTimeout(ctx, time.Second)
	defer cancel1s()

	// sn1 gets the message
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data1, sub1)

	// sn2 also gets the message (as part of the libp2p loopback of published topic messages)
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data1, sub2)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")
}

// TestTopicValidator_TopicMismatch tests that the libP2P node topic validator rejects messages with mismatched topics
func TestTopicValidator_TopicMismatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := mockmodule.NewIdentityProvider(t)
	// create a hooked logger
	logger, hook := unittest.HookedLogger()

	sporkId := unittest.IdentifierFixture()

	sn1, identity1 := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus), p2ptest.WithLogger(logger))
	sn2, identity2 := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus), p2ptest.WithLogger(logger))
	idProvider.On("ByPeerID", sn1.Host().ID()).Return(&identity1, true).Maybe()
	idProvider.On("ByPeerID", sn2.Host().ID()).Return(&identity2, true).Maybe()
	nodes := []p2p.LibP2PNode{sn1, sn2}
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	channel := channels.ConsensusCommittee
	topic := channels.TopicFromChannel(channel, sporkId)

	pInfo2, err := utils.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2
	// sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(ctx, pInfo2))

	// sn2 will subscribe with an unauthenticated callback to allow processing of message after the authorization check
	_, err = sn1.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// sn2 will subscribe with an unauthenticated callback to allow it to send the unauthenticated message
	_, err = sn2.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(ctx, 5*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	data1 := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture(), channels.Channel("invalid-channel"))

	err = sn2.Publish(timedCtx, topic, data1)

	// publish fails because the channel validation fails
	require.Error(t, err)

	// ensure the correct error is contained in the logged error
	require.Contains(t, hook.Logs(), "channel id in message does not match pubsub topic")
}

// TestTopicValidator_InvalidTopic tests that the libP2P node topic validator rejects messages with invalid topics
func TestTopicValidator_InvalidTopic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := mockmodule.NewIdentityProvider(t)
	// create a hooked logger
	logger, hook := unittest.HookedLogger()

	sporkId := unittest.IdentifierFixture()

	sn1, identity1 := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus), p2ptest.WithLogger(logger))
	sn2, identity2 := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus), p2ptest.WithLogger(logger))
	idProvider.On("ByPeerID", sn1.Host().ID()).Return(&identity1, true).Maybe()
	idProvider.On("ByPeerID", sn2.Host().ID()).Return(&identity2, true).Maybe()
	nodes := []p2p.LibP2PNode{sn1, sn2}
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	topic := channels.Topic("invalid-topic")

	pInfo2, err := utils.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2
	// sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(ctx, pInfo2))

	// sn2 will subscribe with an unauthenticated callback to allow processing of message after the authorization check
	_, err = sn1.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// sn2 will subscribe with an unauthenticated callback to allow it to send the unauthenticated message
	_, err = sn2.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(ctx, 5*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	data1 := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture(), channels.PushBlocks)

	err = sn2.Publish(timedCtx, topic, data1)

	// publish fails because the topic conversion fails
	require.Error(t, err)

	// ensure the correct error is contained in the logged error
	require.Contains(t, hook.Logs(), "could not convert topic to channel")
}

// TestAuthorizedSenderValidator_Unauthorized tests that the authorized sender validator rejects messages from nodes that are not authorized to send the message
func TestAuthorizedSenderValidator_Unauthorized(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := mockmodule.NewIdentityProvider(t)
	logger := unittest.Logger()

	sporkId := unittest.IdentifierFixture()

	sn1, identity1 := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus))
	sn2, identity2 := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleConsensus))
	an1, identity3 := p2ptest.NodeFixture(t, sporkId, t.Name(), idProvider, p2ptest.WithRole(flow.RoleAccess))
	idProvider.On("ByPeerID", sn1.Host().ID()).Return(&identity1, true).Maybe()
	idProvider.On("ByPeerID", sn2.Host().ID()).Return(&identity2, true).Maybe()
	idProvider.On("ByPeerID", an1.Host().ID()).Return(&identity3, true).Maybe()
	nodes := []p2p.LibP2PNode{sn1, sn2, an1}
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	channel := channels.ConsensusCommittee
	topic := channels.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2, &identity3}

	translatorFixture, err := translator.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	violation := &network.Violation{
		Identity: &identity3,
		PeerID:   an1.Host().ID().String(),
		OriginID: identity3.NodeID,
		MsgType:  "*messages.BlockProposal",
		Channel:  channel,
		Protocol: message.ProtocolTypePubSub,
		Err:      message.ErrUnauthorizedRole,
	}
	violationsConsumer := mocknetwork.NewViolationsConsumer(t)
	violationsConsumer.On("OnUnAuthorizedSenderError", violation).Once().Return(nil)
	getIdentity := func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translatorFixture.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}

		return ids.ByNodeID(fid)
	}
	authorizedSenderValidator := validator.NewAuthorizedSenderValidator(logger, violationsConsumer, getIdentity)
	pubsubMessageValidator := authorizedSenderValidator.PubSubMessageValidator(channel)

	pInfo1, err := utils.PeerAddressInfo(identity1)
	require.NoError(t, err)

	pInfo2, err := utils.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2, and the an1 is connected to node1
	// an1 <-> sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(ctx, pInfo2))
	require.NoError(t, an1.AddPeer(ctx, pInfo1))

	// sn1 and sn2 subscribe to the topic with the topic validator
	sub1, err := sn1.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter(), pubsubMessageValidator))
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter(), pubsubMessageValidator))
	require.NoError(t, err)
	sub3, err := an1.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(ctx, 60*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	data1 := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture(), channel)

	// sn2 publishes the block proposal, sn1 and an1 should receive the message because
	// SN nodes are authorized to send block proposals
	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	// sn1 gets the message
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data1, sub1)

	// sn2 also gets the message (as part of the libp2p loopback of published topic messages)
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data1, sub2)

	// an1 also gets the message
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data1, sub3)

	timedCtx, cancel2s := context.WithTimeout(ctx, 2*time.Second)
	defer cancel2s()
	data2 := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture(), channel)

	// the access node now publishes the block proposal message, AN are not authorized to publish block proposals
	// the message should be rejected by the topic validator on sn1
	err = an1.Publish(timedCtx, topic, data2)
	require.NoError(t, err)

	// an1 receives its own message
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data2, sub3)

	var wg sync.WaitGroup

	// sn1 does NOT receive the message due to the topic validator
	timedCtx, cancel1s := context.WithTimeout(ctx, time.Second)
	defer cancel1s()
	p2pfixtures.SubMustNeverReceiveAnyMessage(t, timedCtx, sub1)

	// sn2 also does not receive the message via gossip from the sn1 (event after the 1 second hearbeat)
	timedCtx, cancel2s = context.WithTimeout(ctx, 2*time.Second)
	defer cancel2s()
	p2pfixtures.SubMustNeverReceiveAnyMessage(t, timedCtx, sub2)

	unittest.RequireReturnsBefore(t, wg.Wait, 5*time.Second, "could not receive message on time")
}

// TestAuthorizedSenderValidator_Authorized tests that the authorized sender validator rejects messages being sent on the wrong channel
func TestAuthorizedSenderValidator_InvalidMsg(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := mockmodule.NewIdentityProvider(t)
	// create a hooked logger
	logger, hook := unittest.HookedLogger()

	sporkId := unittest.IdentifierFixture()

	sn1, identity1 := p2ptest.NodeFixture(t, sporkId, "consensus_1", idProvider, p2ptest.WithRole(flow.RoleConsensus))
	sn2, identity2 := p2ptest.NodeFixture(t, sporkId, "consensus_2", idProvider, p2ptest.WithRole(flow.RoleConsensus))
	idProvider.On("ByPeerID", sn1.Host().ID()).Return(&identity1, true).Maybe()
	idProvider.On("ByPeerID", sn2.Host().ID()).Return(&identity2, true).Maybe()
	nodes := []p2p.LibP2PNode{sn1, sn2}
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	// try to publish BlockProposal on invalid SyncCommittee channel
	channel := channels.SyncCommittee
	topic := channels.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2}
	translatorFixture, err := translator.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	expectedMisbehaviorReport, err := alsp.NewMisbehaviorReport(identity2.NodeID, alsp.UnAuthorizedSender)
	require.NoError(t, err)
	misbehaviorReportConsumer := mocknetwork.NewMisbehaviorReportConsumer(t)
	misbehaviorReportConsumer.On("ReportMisbehaviorOnChannel", channel, expectedMisbehaviorReport).Once()
	violationsConsumer := slashing.NewSlashingViolationsConsumer(logger, metrics.NewNoopCollector(), misbehaviorReportConsumer)
	getIdentity := func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translatorFixture.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}

		return ids.ByNodeID(fid)
	}
	authorizedSenderValidator := validator.NewAuthorizedSenderValidator(logger, violationsConsumer, getIdentity)
	pubsubMessageValidator := authorizedSenderValidator.PubSubMessageValidator(channel)

	pInfo2, err := utils.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2
	// sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(ctx, pInfo2))

	// sn1 subscribe to the topic with the topic validator, while sn2 will subscribe without the topic validator to allow sn2 to publish unauthorized messages
	sub1, err := sn1.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter(), pubsubMessageValidator))
	require.NoError(t, err)
	_, err = sn2.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(ctx, 5*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	data1 := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture(), channel)

	// sn2 publishes the block proposal on the sync committee channel
	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	// sn1 should not receive message from sn2
	timedCtx, cancel1s := context.WithTimeout(ctx, time.Second)
	defer cancel1s()
	p2pfixtures.SubMustNeverReceiveAnyMessage(t, timedCtx, sub1)

	// ensure the correct error is contained in the logged error
	require.Contains(t, hook.Logs(), message.ErrUnauthorizedMessageOnChannel.Error())
}

// TestAuthorizedSenderValidator_Ejected tests that the authorized sender validator rejects messages from nodes that are ejected
func TestAuthorizedSenderValidator_Ejected(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := mockmodule.NewIdentityProvider(t)
	// create a hooked logger
	logger, hook := unittest.HookedLogger()

	sporkId := unittest.IdentifierFixture()

	sn1, identity1 := p2ptest.NodeFixture(t, sporkId, "consensus_1", idProvider, p2ptest.WithRole(flow.RoleConsensus))
	sn2, identity2 := p2ptest.NodeFixture(t, sporkId, "consensus_2", idProvider, p2ptest.WithRole(flow.RoleConsensus))
	an1, identity3 := p2ptest.NodeFixture(t, sporkId, "access_1", idProvider, p2ptest.WithRole(flow.RoleAccess))
	idProvider.On("ByPeerID", sn1.Host().ID()).Return(&identity1, true).Maybe()
	idProvider.On("ByPeerID", sn2.Host().ID()).Return(&identity2, true).Maybe()
	idProvider.On("ByPeerID", an1.Host().ID()).Return(&identity3, true).Maybe()
	nodes := []p2p.LibP2PNode{sn1, sn2, an1}
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	channel := channels.ConsensusCommittee
	topic := channels.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2, &identity3}
	translatorFixture, err := translator.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	expectedMisbehaviorReport, err := alsp.NewMisbehaviorReport(identity2.NodeID, alsp.SenderEjected)
	require.NoError(t, err)
	misbehaviorReportConsumer := mocknetwork.NewMisbehaviorReportConsumer(t)
	misbehaviorReportConsumer.On("ReportMisbehaviorOnChannel", channel, expectedMisbehaviorReport).Once()
	violationsConsumer := slashing.NewSlashingViolationsConsumer(logger, metrics.NewNoopCollector(), misbehaviorReportConsumer)
	getIdentity := func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translatorFixture.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}

		return ids.ByNodeID(fid)
	}
	authorizedSenderValidator := validator.NewAuthorizedSenderValidator(logger, violationsConsumer, getIdentity)
	pubsubMessageValidator := authorizedSenderValidator.PubSubMessageValidator(channel)

	pInfo1, err := utils.PeerAddressInfo(identity1)
	require.NoError(t, err)

	pInfo2, err := utils.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// node1 is connected to node2, and the an1 is connected to node1
	// an1 <-> sn1 <-> sn2
	require.NoError(t, sn1.AddPeer(ctx, pInfo2))
	require.NoError(t, an1.AddPeer(ctx, pInfo1))

	// sn1 subscribe to the topic with the topic validator, while sn2 will subscribe without the topic validator to allow sn2 to publish unauthorized messages
	sub1, err := sn1.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter(), pubsubMessageValidator))
	require.NoError(t, err)
	sub2, err := sn2.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
	require.NoError(t, err)
	sub3, err := an1.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(ctx, 5*time.Second)
	defer cancel5s()
	// create a dummy block proposal to publish from our SN node
	data1 := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture(), channel)

	// sn2 publishes the block proposal, sn1 and an1 should receive the message because
	// SN nodes are authorized to send block proposals
	err = sn2.Publish(timedCtx, topic, data1)
	require.NoError(t, err)

	// sn1 gets the message
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data1, sub1)

	// sn2 also gets the message (as part of the libp2p loopback of published topic messages)
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data1, sub2)

	// an1 also gets the message
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data1, sub3)

	// "eject" sn2 to ensure messages published by ejected nodes get rejected
	identity2.Ejected = true
	data3 := p2pfixtures.MustEncodeEvent(t, unittest.ProposalFixture(), channel)
	timedCtx, cancel2s := context.WithTimeout(ctx, time.Second)
	defer cancel2s()
	err = sn2.Publish(timedCtx, topic, data3)
	require.NoError(t, err)

	// sn1 should not receive rejected message from ejected sn2
	timedCtx, cancel1s := context.WithTimeout(ctx, time.Second)
	defer cancel1s()
	p2pfixtures.SubMustNeverReceiveAnyMessage(t, timedCtx, sub1)

	// ensure the correct error is contained in the logged error
	require.Contains(t, hook.Logs(), validator.ErrSenderEjected.Error())
}

// TestAuthorizedSenderValidator_ClusterChannel tests that the authorized sender validator correctly validates messages sent on cluster channels
func TestAuthorizedSenderValidator_ClusterChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := mockmodule.NewIdentityProvider(t)
	sporkId := unittest.IdentifierFixture()

	ln1, identity1 := p2ptest.NodeFixture(t, sporkId, "collection_1", idProvider, p2ptest.WithRole(flow.RoleCollection))
	ln2, identity2 := p2ptest.NodeFixture(t, sporkId, "collection_2", idProvider, p2ptest.WithRole(flow.RoleCollection))
	ln3, identity3 := p2ptest.NodeFixture(t, sporkId, "collection_3", idProvider, p2ptest.WithRole(flow.RoleCollection))
	idProvider.On("ByPeerID", ln1.Host().ID()).Return(&identity1, true).Maybe()
	idProvider.On("ByPeerID", ln2.Host().ID()).Return(&identity2, true).Maybe()
	idProvider.On("ByPeerID", ln3.Host().ID()).Return(&identity3, true).Maybe()
	nodes := []p2p.LibP2PNode{ln1, ln2, ln3}
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	channel := channels.SyncCluster(flow.Testnet)
	topic := channels.TopicFromChannel(channel, sporkId)

	ids := flow.IdentityList{&identity1, &identity2, &identity3}
	translatorFixture, err := translator.NewFixedTableIdentityTranslator(ids)
	require.NoError(t, err)

	logger := unittest.Logger()
	misbehaviorReportConsumer := mocknetwork.NewMisbehaviorReportConsumer(t)
	defer misbehaviorReportConsumer.AssertNotCalled(t, "ReportMisbehaviorOnChannel", mock.AnythingOfType("channels.Channel"), mock.AnythingOfType("*alsp.MisbehaviorReport"))
	violationsConsumer := slashing.NewSlashingViolationsConsumer(logger, metrics.NewNoopCollector(), misbehaviorReportConsumer)
	getIdentity := func(pid peer.ID) (*flow.Identity, bool) {
		fid, err := translatorFixture.GetFlowID(pid)
		if err != nil {
			return &flow.Identity{}, false
		}

		return ids.ByNodeID(fid)
	}
	authorizedSenderValidator := validator.NewAuthorizedSenderValidator(logger, violationsConsumer, getIdentity)
	pubsubMessageValidator := authorizedSenderValidator.PubSubMessageValidator(channel)

	pInfo1, err := utils.PeerAddressInfo(identity1)
	require.NoError(t, err)

	pInfo2, err := utils.PeerAddressInfo(identity2)
	require.NoError(t, err)

	// ln3 <-> sn1 <-> sn2
	require.NoError(t, ln1.AddPeer(ctx, pInfo2))
	require.NoError(t, ln3.AddPeer(ctx, pInfo1))

	sub1, err := ln1.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter(), pubsubMessageValidator))
	require.NoError(t, err)
	sub2, err := ln2.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter(), pubsubMessageValidator))
	require.NoError(t, err)
	sub3, err := ln3.Subscribe(topic, flowpubsub.TopicValidator(logger, unittest.AllowAllPeerFilter(), pubsubMessageValidator))
	require.NoError(t, err)

	// let nodes form the mesh
	time.Sleep(time.Second)

	timedCtx, cancel5s := context.WithTimeout(ctx, 5*time.Second)
	defer cancel5s()
	// create a dummy sync request to publish from our LN node
	data := p2pfixtures.MustEncodeEvent(t, &messages.RangeRequest{}, channel)

	// ln2 publishes the sync request on the cluster channel
	err = ln2.Publish(timedCtx, topic, data)
	require.NoError(t, err)

	// ln1 gets the message
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data, sub1)

	// ln2 also gets the message (as part of the libp2p loopback of published topic messages)
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data, sub2)

	// ln3 also gets the message
	p2pfixtures.SubMustReceiveMessage(t, timedCtx, data, sub3)
}
