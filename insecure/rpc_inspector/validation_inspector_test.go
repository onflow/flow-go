package rpc_inspector

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
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
	inspectorConfig := internal.DefaultRPCValidationConfig()
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
	mockDistributorReadyDoneAware(distributor)
	defer distributor.AssertNotCalled(t, "Distribute", mockery.Anything)
	inspector := validation.NewControlMsgValidationInspector(logger, sporkID, inspectorConfig, distributor, metrics.NewNoopCollector())
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
		corruptlibp2p.WithGraft(messageCount, channels.PushBlocks.String()),
		corruptlibp2p.WithIHave(messageCount, 1000, channels.PushBlocks.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ctlMsgs)

	// eventually we should receive 2 info logs each for GRAFT inspection and PRUNE inspection
	require.Eventually(t, func() bool {
		return graftInfoLogsReceived.Load() == controlMessageCount && pruneInfoLogsReceived.Load() == controlMessageCount
	}, 2*time.Second, 10*time.Millisecond)
}

// TestValidationInspector_HardThreshold ensures that when RPC control message count is above the configured hard threshold the control message validation inspector
// returns the expected error.
func TestValidationInspector_HardThreshold(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	// if GRAFT/PRUNE message count is higher than hard threshold the RPC validation should fail and expected error should be returned
	hardThreshold := uint64(10)
	// create our RPC validation inspector
	inspectorConfig := internal.DefaultRPCValidationConfig()
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.GraftValidationCfg.HardThreshold = hardThreshold
	inspectorConfig.PruneValidationCfg.HardThreshold = hardThreshold

	messageCount := 50
	controlMessageCount := int64(1)
	logger := unittest.Logger()
	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	count := atomic.NewInt64(0)
	done := make(chan struct{})
	distributor.On("Distribute", mockery.Anything).
		Twice().
		Run(func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.Equal(t, uint64(messageCount), notification.Count)

			require.True(t, validation.IsErrHardThreshold(notification.Err))
			require.True(t, notification.MsgType == p2p.CtrlMsgGraft || notification.MsgType == p2p.CtrlMsgPrune)
			if count.Load() == 2 {
				close(done)
			}
		}).Return(nil)
	inspector := validation.NewControlMsgValidationInspector(logger, sporkID, inspectorConfig, distributor, metrics.NewNoopCollector())
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

// TestValidationInspector_HardThresholdIHave ensures that when the ihave RPC control message count is above the configured hard threshold the control message validation inspector
// inspects a sample size of the ihave messages and returns the expected error when validation for a topic in that sample fails.
func TestValidationInspector_HardThresholdIHave(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	// create our RPC validation inspector
	inspectorConfig := internal.DefaultRPCValidationConfig()
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.IHaveValidationCfg.HardThreshold = 50
	inspectorConfig.IHaveValidationCfg.IHaveInspectionMaxSampleSize = 100
	// set the sample size divisor to 2 which will force inspection of 50% of topic IDS
	inspectorConfig.IHaveValidationCfg.IHaveSyncInspectSampleSizePercentage = .5

	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", corruptlibp2p.GossipSubTopicIdFixture(), sporkID))
	messageCount := 100
	controlMessageCount := int64(1)
	logger := unittest.Logger()
	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	count := atomic.NewInt64(0)
	done := make(chan struct{})
	distributor.On("Distribute", mockery.Anything).
		Once().
		Run(func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.Equal(t, uint64(messageCount), notification.Count)
			require.True(t, validation.IsErrInvalidTopic(notification.Err))
			// simple string check to ensure the sample size is calculated as expected
			expectedSubStr := fmt.Sprintf("invalid topic %s out of %d total topics sampled", unknownTopic.String(), int(float64(messageCount)*inspectorConfig.IHaveValidationCfg.IHaveSyncInspectSampleSizePercentage))
			require.True(t, strings.Contains(notification.Err.Error(), expectedSubStr))
			require.Equal(t, p2p.CtrlMsgIHave, notification.MsgType)
			// due to the fact that a sample of ihave messages will be inspected synchronously
			// as soon as an error is encountered the inspector will fail and distribute a notification
			// thus we can close this done channel immediately.
			close(done)
		}).Return(nil)
	inspector := validation.NewControlMsgValidationInspector(logger, sporkID, inspectorConfig, distributor, metrics.NewNoopCollector())
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

	// add an unknown topic to each of our ihave control messages, this will ensure
	// that whatever random sample of topic ids that are inspected cause validation
	// to fail and a notification to be disseminated as expected.
	ihaveCtlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIHave(messageCount, 1000, unknownTopic.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ihaveCtlMsgs)

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
	inspectorConfig := internal.DefaultRPCValidationConfig()
	inspectorConfig.NumberOfWorkers = 1

	// here we set the message count to the amount of flow channels
	// so that we can generate a valid ctl msg with all valid topics.
	flowChannels := channels.Channels()
	messageCount := flowChannels.Len()
	controlMessageCount := int64(1)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
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
	inspector := validation.NewControlMsgValidationInspector(unittest.Logger(), sporkID, inspectorConfig, distributor, metrics.NewNoopCollector())
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
		ihaves := make([]*pb.ControlIHave, messageCount)
		for i := 0; i < messageCount; i++ {
			topic := fmt.Sprintf("%s/%s", flowChannels[i].String(), sporkID)
			grafts[i] = &pb.ControlGraft{TopicID: &topic}
			prunes[i] = &pb.ControlPrune{TopicID: &topic}
			ihaves[i] = &pb.ControlIHave{TopicID: &topic, MessageIDs: corruptlibp2p.GossipSubMessageIdsFixture(messageCount)}
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
	// if GRAFT/PRUNE message count is higher than hard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	inspectorConfig := internal.DefaultRPCValidationConfig()
	inspectorConfig.PruneValidationCfg.SafetyThreshold = 0
	inspectorConfig.GraftValidationCfg.SafetyThreshold = 0

	inspectorConfig.IHaveValidationCfg.SafetyThreshold = 0
	inspectorConfig.IHaveValidationCfg.HardThreshold = 50
	inspectorConfig.IHaveValidationCfg.IHaveAsyncInspectSampleSizePercentage = .5
	inspectorConfig.IHaveValidationCfg.IHaveInspectionMaxSampleSize = 100
	ihaveMessageCount := 100

	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < HardThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := inspectorConfig.GraftValidationCfg.SafetyThreshold + 1
	controlMessageCount := int64(1)
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", corruptlibp2p.GossipSubTopicIdFixture(), sporkID))
	malformedTopic := channels.Topic("!@#$%^&**((")
	// a topics spork ID is considered invalid if it does not match the current spork ID
	invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture()))
	duplicateTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID))

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	count := atomic.NewInt64(0)
	// we expect 4 different notifications for invalid topics for 3 control message types thus 12 notifications total
	expectedCount := 12
	done := make(chan struct{})
	distributor.On("Distribute", mockery.Anything).
		Times(12).
		Run(func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			msgType := notification.MsgType
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, validation.IsErrInvalidTopic(notification.Err) || validation.IsErrDuplicateTopic(notification.Err))
			// in the case where a duplicate topic ID exists we expect notification count to be 2
			switch {
			case msgType == p2p.CtrlMsgGraft || msgType == p2p.CtrlMsgPrune:
				require.Equal(t, messageCount, notification.Count)
			case msgType == p2p.CtrlMsgIHave:
				require.Equal(t, uint64(ihaveMessageCount), notification.Count)
			default:
				t.Fatalf("unexpected control message type: %s", msgType)
			}
			if count.Load() == int64(expectedCount) {
				close(done)
			}
		}).Return(nil)
	inspector := validation.NewControlMsgValidationInspector(unittest.Logger(), sporkID, inspectorConfig, distributor, metrics.NewNoopCollector())
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
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), duplicateTopic.String()))

	pruneCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), unknownTopic.String()))
	pruneCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), malformedTopic.String()))
	pruneCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), invalidSporkIDTopic.String()))
	pruneCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), duplicateTopic.String()))

	iHaveCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIHave(ihaveMessageCount, 1000, unknownTopic.String()))
	iHaveCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIHave(ihaveMessageCount, 1000, malformedTopic.String()))
	iHaveCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIHave(ihaveMessageCount, 1000, invalidSporkIDTopic.String()))
	iHaveCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIHave(ihaveMessageCount, 1000, duplicateTopic.String()))

	// spam the victim peer with invalid graft messages
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsInvalidSporkIDTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsDuplicateTopic)

	// spam the victim peer with invalid prune messages
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsInvalidSporkIDTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsDuplicateTopic)

	// spam the victim peer with invalid ihave messages
	spammer.SpamControlMessage(t, victimNode, iHaveCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, iHaveCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, iHaveCtlMsgsInvalidSporkIDTopic)
	spammer.SpamControlMessage(t, victimNode, iHaveCtlMsgsDuplicateTopic)

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
	idProvider.On("ByPeerID", mockery.Anything).Return(
		func(peerId peer.ID) *flow.Identity {
			switch peerId {
			case victimNode.Host().ID():
				return &victimId
			case spammer.SpammerNode.Host().ID():
				return &spammer.SpammerId
			default:
				return nil
			}

		}, func(peerId peer.ID) bool {
			switch peerId {
			case victimNode.Host().ID():
				fallthrough
			case spammer.SpammerNode.Host().ID():
				return true
			default:
				return false
			}
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
	graftCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithGraft(spamRpcCount, unknownTopic.String()))
	graftCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithGraft(spamRpcCount, malformedTopic.String()))
	graftCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithGraft(spamRpcCount, invalidSporkIDTopic.String()))
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithGraft(3, duplicateTopic.String()))

	pruneCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithPrune(spamRpcCount, unknownTopic.String()))
	pruneCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithPrune(spamRpcCount, malformedTopic.String()))
	pruneCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), corruptlibp2p.WithGraft(spamRpcCount, invalidSporkIDTopic.String()))
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

	// wait for three GossipSub heartbeat intervals to ensure that the victim node has penalized the spammer node.
	time.Sleep(3 * time.Second)

	// now we expect the detection and mitigation to kick in and the victim node to disconnect from the spammer node.
	// so the spammer and victim nodes should not be able to exchange messages on the topic.
	p2ptest.EnsureNoPubsubExchangeBetweenGroups(t, ctx, []p2p.LibP2PNode{victimNode}, []p2p.LibP2PNode{spammer.SpammerNode}, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkID)
		return unittest.ProposalFixture(), blockTopic
	})
}

// mockDistributorReadyDoneAware mocks the Ready and Done methods of the distributor to return a channel that is already closed,
// so that the distributor is considered ready and done when the test needs.
func mockDistributorReadyDoneAware(d *mockp2p.GossipSubInspectorNotificationDistributor) {
	d.On("Start", mockery.Anything).Return().Maybe()
	d.On("Ready").Return(func() <-chan struct{} {
		ch := make(chan struct{})
		close(ch)
		return ch
	}()).Maybe()
	d.On("Done").Return(func() <-chan struct{} {
		ch := make(chan struct{})
		close(ch)
		return ch
	}()).Maybe()
}
