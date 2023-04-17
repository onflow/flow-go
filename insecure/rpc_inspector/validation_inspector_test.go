package rpc_inspector

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder/inspector"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestValidationInspector_SafetyThreshold ensures that when RPC control message count is below the configured safety threshold the control message validation inspector
// does not return any errors and validation is skipped.
func TestValidationInspector_SafetyThreshold(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	// if GRAFT/PRUNE message count is lower than safety threshold the RPC validation should pass
	safetyThreshold := uint64(10)
	// create our RPC validation inspector
	inspectorConfig := inspector.DefaultRPCValidationConfig()
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.GraftValidationCfg.SafetyThreshold = safetyThreshold
	inspectorConfig.PruneValidationCfg.SafetyThreshold = safetyThreshold

	messageCount := 5
	controlMessageCount := int64(2)

	// expected log message logged when valid number GRAFT control messages spammed under safety threshold
	graftExpectedMessageStr := fmt.Sprintf("control message %s inspection passed 5 is below configured safety threshold", p2p.CtrlMsgGraft)
	// expected log message logged when valid number PRUNE control messages spammed under safety threshold
	pruneExpectedMessageStr := fmt.Sprintf("control message %s inspection passed 5 is below configured safety threshold", p2p.CtrlMsgGraft)

	graftInfoLogsReceived := atomic.NewInt64(0)
	pruneInfoLogsReceived := atomic.NewInt64(0)

	// setup logger hook, we expect info log validation is skipped
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.TraceLevel {
			if message == graftExpectedMessageStr {
				graftInfoLogsReceived.Inc()
			}

			if message == pruneExpectedMessageStr {
				pruneInfoLogsReceived.Inc()
			}
		}
	})
	logger := zerolog.New(os.Stdout).Hook(hook)
	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	defer distributor.AssertNotCalled(t, "Distribute", mockery.Anything)
	inspector := validation.NewControlMsgValidationInspector(logger, sporkID, inspectorConfig, distributor)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(inspector)
	victimNode, _ := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	inspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopNodesAndInspector(t, cancel, nodes, inspector)
	// prepare to spam - generate control messages
	ctlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount),
		corruptlibp2p.WithGraft(messageCount, channels.PushBlocks.String()),
		corruptlibp2p.WithPrune(messageCount, channels.PushBlocks.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ctlMsgs)

	// eventually we should receive 2 info logs each for GRAFT inspection and PRUNE inspection
	require.Eventually(t, func() bool {
		return graftInfoLogsReceived.Load() == controlMessageCount && pruneInfoLogsReceived.Load() == controlMessageCount
	}, 2*time.Second, 10*time.Millisecond)
}

// TestValidationInspector_DiscardThreshold ensures that when RPC control message count is above the configured discard threshold the control message validation inspector
// returns the expected error.
func TestValidationInspector_DiscardThreshold(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	// if GRAFT/PRUNE message count is higher than discard threshold the RPC validation should fail and expected error should be returned
	discardThreshold := uint64(10)
	// create our RPC validation inspector
	inspectorConfig := inspector.DefaultRPCValidationConfig()
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.GraftValidationCfg.DiscardThreshold = discardThreshold
	inspectorConfig.PruneValidationCfg.DiscardThreshold = discardThreshold

	messageCount := 50
	controlMessageCount := int64(1)
	logger := unittest.Logger()
	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	count := atomic.NewInt64(0)
	done := make(chan struct{})
	distributor.On("Distribute", mockery.Anything).
		Twice().
		Run(func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, validation.IsErrDiscardThreshold(notification.Err))
			require.Equal(t, uint64(messageCount), notification.Count)
			require.True(t, notification.MsgType == p2p.CtrlMsgGraft || notification.MsgType == p2p.CtrlMsgPrune)
			if count.Load() == 2 {
				close(done)
			}
		}).Return(nil)
	inspector := validation.NewControlMsgValidationInspector(logger, sporkID, inspectorConfig, distributor)
	// we use inline inspector here so that we can check the error type when we inspect an RPC and
	// track which control message type the error involves
	inlineInspector := func(id peer.ID, rpc *corrupt.RPC) error {
		pubsubRPC := corruptlibp2p.CorruptRPCToPubSubRPC(rpc)
		return inspector.Inspect(id, pubsubRPC)
	}
	victimNode, _ := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(inlineInspector)),
	)

	inspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopNodesAndInspector(t, cancel, nodes, inspector)

	// prepare to spam - generate control messages
	graftCtlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(messageCount, channels.PushBlocks.String()))
	pruneCtlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(messageCount, channels.PushBlocks.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgs)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgs)

	unittest.RequireCloseBefore(t, done, 2*time.Second, "failed to inspect RPC messages on time")
}

// TestValidationInspector_RateLimitedPeer ensures that the control message validation inspector rate limits peers per control message type as expected.
func TestValidationInspector_RateLimitedPeer(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	// create our RPC validation inspector
	inspectorConfig := inspector.DefaultRPCValidationConfig()
	inspectorConfig.NumberOfWorkers = 1

	// here we set the message count to the amount of flow channels
	// so that we can generate a valid ctl msg with all valid topics.
	flowChannels := channels.Channels()
	messageCount := flowChannels.Len()
	controlMessageCount := int64(1)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	count := atomic.NewInt64(0)
	done := make(chan struct{})
	distributor.On("Distribute", mockery.Anything).
		Times(4).
		Run(func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, validation.IsErrRateLimitedControlMsg(notification.Err))
			require.Equal(t, uint64(messageCount), notification.Count)
			require.True(t, notification.MsgType == p2p.CtrlMsgGraft || notification.MsgType == p2p.CtrlMsgPrune)
			if count.Load() == 4 {
				close(done)
			}
		}).Return(nil)
	inspector := validation.NewControlMsgValidationInspector(unittest.Logger(), sporkID, inspectorConfig, distributor)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(inspector)
	victimNode, _ := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)

	inspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopNodesAndInspector(t, cancel, nodes, inspector)

	// the first time we spam this message it will be processed completely so we need to ensure
	// all topics are valid and no duplicates exists.
	validCtlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount), func(message *pb.ControlMessage) {
		grafts := make([]*pb.ControlGraft, messageCount)
		prunes := make([]*pb.ControlPrune, messageCount)
		for i := 0; i < messageCount; i++ {
			topic := fmt.Sprintf("%s/%s", flowChannels[i].String(), sporkID)
			grafts[i] = &pb.ControlGraft{TopicID: &topic}
			prunes[i] = &pb.ControlPrune{TopicID: &topic}
		}
		message.Graft = grafts
		message.Prune = prunes
	})

	// start spamming the victim peer
	for i := 0; i < 3; i++ {
		spammer.SpamControlMessage(t, victimNode, validCtlMsgs)
	}

	unittest.RequireCloseBefore(t, done, 2*time.Second, "failed to inspect RPC messages on time")
}

// TestValidationInspector_InvalidTopicID ensures that when an RPC control message contains an invalid topic ID the expected error is logged.
func TestValidationInspector_InvalidTopicID(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	// if GRAFT/PRUNE message count is higher than discard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	inspectorConfig := inspector.DefaultRPCValidationConfig()
	inspectorConfig.PruneValidationCfg.SafetyThreshold = 0
	inspectorConfig.GraftValidationCfg.SafetyThreshold = 0
	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < DiscardThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := inspectorConfig.GraftValidationCfg.SafetyThreshold + 1
	controlMessageCount := int64(1)
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", corruptlibp2p.GossipSubTopicIdFixture(), sporkID))
	malformedTopic := channels.Topic("!@#$%^&**((")
	// a topics spork ID is considered invalid if it does not match the current spork ID
	invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture()))
	duplicateTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID))

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	count := atomic.NewInt64(0)
	done := make(chan struct{})
	distributor.On("Distribute", mockery.Anything).
		Times(8).
		Run(func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, validation.IsErrInvalidTopic(notification.Err) || validation.IsErrDuplicateTopic(notification.Err))
			require.True(t, messageCount == notification.Count || notification.Count == 3)
			require.True(t, notification.MsgType == p2p.CtrlMsgGraft || notification.MsgType == p2p.CtrlMsgPrune)
			if count.Load() == 8 {
				close(done)
			}
		}).Return(nil)
	inspector := validation.NewControlMsgValidationInspector(unittest.Logger(), sporkID, inspectorConfig, distributor)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(inspector)
	victimNode, _ := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)

	inspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopNodesAndInspector(t, cancel, nodes, inspector)

	// prepare to spam - generate control messages
	graftCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), unknownTopic.String()))
	graftCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), malformedTopic.String()))
	graftCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), invalidSporkIDTopic.String()))
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(3, duplicateTopic.String()))

	pruneCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), unknownTopic.String()))
	pruneCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), malformedTopic.String()))
	pruneCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), invalidSporkIDTopic.String()))
	pruneCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(3, duplicateTopic.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsInvalidSporkIDTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsDuplicateTopic)

	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsInvalidSporkIDTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsDuplicateTopic)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
}

// TestGossipSubSpamMitigationIntegration tests that the spam mitigation feature of GossipSub is working as expected.
// The test puts toghether the spam detection (through the GossipSubInspector) and the spam mitigation (through the
// scoring system) and ensures that the mitigation is triggered when the spam detection detects spam.
// The test scenario involves a spammer node that sends a large number of control messages to a victim node.
// The victim node is configured to use the GossipSubInspector to detect spam and the scoring system to mitigate spam.
// The test ensures that the victim node is disconnected from the spammer node on the GossipSub mesh after the spam detection is triggered.
func TestGossipSubSpamMitigationIntegration(t *testing.T) {
	t.Parallel()
	idProvider := mock.NewIdentityProvider(t)
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, flow.RoleConsensus)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	victimNode, victimId := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.WithPeerScoringEnabled(idProvider),
	)

	ids := flow.IdentityList{&victimId, &spammer.SpammerId}
	provider := id.NewFixedIdentityProvider(ids)
	idProvider.On("ByPeerID", mockery.Anything).Return(
		func(peerId peer.ID) *flow.Identity {
			identity, _ := provider.ByPeerID(peerId)
			return identity
		}, func(peerId peer.ID) bool {
			_, ok := provider.ByPeerID(peerId)
			return ok
		})

	spamRpcCount := 10            // total number of individual rpc messages to send
	spamCtrlMsgCount := int64(10) // total number of control messages to send on each RPC

	// unknownTopic is an unknown topic to the victim node but shaped like a valid topic (i.e., it has the correct prefix and spork ID).
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", corruptlibp2p.GossipSubTopicIdFixture(), sporkID))

	// malformedTopic is a topic that is not shaped like a valid topic (i.e., it does not have the correct prefix and spork ID).
	malformedTopic := channels.Topic("!@#$%^&**((")

	// invalidSporkIDTopic is a topic that has a valid prefix but an invalid spork ID (i.e., not the current spork ID).
	invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture()))

	// duplicateTopic is a valid topic that is used to send duplicate spam messages.
	duplicateTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID))

	// starting the nodes.
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 2*time.Second)
	spammer.Start(t)

	// wait for the nodes to discover each other
	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// as nodes started fresh and no spamming has happened yet, the nodes should be able to exchange messages on the topic.
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkID)
		return unittest.ProposalFixture(), blockTopic
	})

	// prepares spam graft and prune messages with different strategies.
	graftCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithGraft(int(spamRpcCount), unknownTopic.String()))
	graftCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithGraft(int(spamRpcCount), malformedTopic.String()))
	graftCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithGraft(int(spamRpcCount), invalidSporkIDTopic.String()))
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithGraft(3, duplicateTopic.String()))

	pruneCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithPrune(int(spamRpcCount), unknownTopic.String()))
	pruneCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithPrune(int(spamRpcCount), malformedTopic.String()))
	pruneCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithGraft(int(spamRpcCount), invalidSporkIDTopic.String()))
	pruneCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithPrune(3, duplicateTopic.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsInvalidSporkIDTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsDuplicateTopic)

	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsInvalidSporkIDTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsDuplicateTopic)

	// now we expect the detection and mitigation to kick in and the victim node to disconnect from the spammer node.
	// so the spammer and victim nodes should not be able to exchange messages on the topic.
	p2ptest.EnsureNoPubsubExchangeBetweenGroups(t, ctx, []p2p.LibP2PNode{victimNode}, []p2p.LibP2PNode{spammer.SpammerNode}, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkID)
		return unittest.ProposalFixture(), blockTopic
	})
}
