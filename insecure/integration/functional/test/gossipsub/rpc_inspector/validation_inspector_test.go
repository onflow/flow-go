package rpc_inspector

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
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
	// if GRAFT/PRUNE message count is lower than safety threshold the RPC validation should pass
	safetyThreshold := uint64(10)
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.GraftLimits.SafetyThreshold = safetyThreshold
	inspectorConfig.PruneLimits.SafetyThreshold = safetyThreshold

	// expected log message logged when valid number GRAFT control messages spammed under safety threshold
	graftExpectedMessageStr := fmt.Sprintf("control message %s inspection passed 5 is below configured safety threshold", p2pmsg.CtrlMsgGraft)
	// expected log message logged when valid number PRUNE control messages spammed under safety threshold
	pruneExpectedMessageStr := fmt.Sprintf("control message %s inspection passed 5 is below configured safety threshold", p2pmsg.CtrlMsgGraft)
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

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)

	messageCount := 5
	controlMessageCount := int64(2)

	meshTracer := meshTracerFixture(flowConfig, idProvider)

	validationInspector, err := validation.NewControlMsgValidationInspector(
		logger,
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Maybe()

	defer distributor.AssertNotCalled(t, "Distribute", mockery.Anything)

	validationInspector.Start(signalerCtx)

	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopTestComponents(t, cancel, nodes, validationInspector)
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

// TestValidationInspector_HardThreshold_Detection ensures that when RPC control message count is above the configured hard threshold an invalid control message
// notification is disseminated with the expected error.
func TestValidationInspector_HardThreshold_Detection(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// if GRAFT/PRUNE message count is higher than hard threshold the RPC validation should fail and expected error should be returned
	hardThreshold := uint64(10)
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.GraftLimits.HardThreshold = hardThreshold
	inspectorConfig.PruneLimits.HardThreshold = hardThreshold

	messageCount := 50
	controlMessageCount := int64(1)
	count := atomic.NewInt64(0)
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	done := make(chan struct{})
	// ensure expected notifications are disseminated with expected error
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.Equal(t, uint64(messageCount), notification.Count)
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2pmsg.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			default:
				require.Fail(t, "unexpected control message type")
			}
			if count.Load() == 2 {
				close(done)
			}
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(2, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Maybe()

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopTestComponents(t, cancel, nodes, validationInspector)

	// prepare to spam - generate control messages
	graftCtlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(messageCount, channels.PushBlocks.String()))
	pruneCtlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(messageCount, channels.PushBlocks.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgs)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgs)

	unittest.RequireCloseBefore(t, done, 2*time.Second, "failed to inspect RPC messages on time")
	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	require.Equal(t, uint64(1), invGraftNotifCount.Load())
	require.Equal(t, uint64(1), invPruneNotifCount.Load())
}

// TestValidationInspector_HardThresholdIHave_Detection ensures that when the ihave RPC control message count is above the configured hard threshold the control message validation inspector
// inspects a sample size of the ihave messages and returns the expected error when validation for a topic in that sample fails.
func TestValidationInspector_HardThresholdIHave_Detection(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.IHaveLimits.HardThreshold = 50
	inspectorConfig.IHaveInspectionMaxSampleSize = 100
	// set the sample size divisor to 2 which will force inspection of 50% of topic IDS
	inspectorConfig.IHaveSyncInspectSampleSizePercentage = .5

	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", corruptlibp2p.GossipSubTopicIdFixture(), sporkID))
	messageCount := 100
	controlMessageCount := int64(1)
	count := atomic.NewInt64(0)
	done := make(chan struct{})

	invIhaveNotifCount := atomic.NewUint64(0)
	// ensure expected notifications are disseminated with expected error
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.Equal(t, uint64(messageCount), notification.Count)
			require.True(t, channels.IsInvalidTopicErr(notification.Err))
			switch notification.MsgType {
			case p2pmsg.CtrlMsgIHave:
				invIhaveNotifCount.Inc()
			default:
				require.Fail(t, "unexpected control message type")
			}
			if count.Load() == 1 {
				close(done)
			}
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(1, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Maybe()

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopTestComponents(t, cancel, nodes, validationInspector)

	// add an unknown topic to each of our ihave control messages, this will ensure
	// that whatever random sample of topic ids that are inspected cause validation
	// to fail and a notification to be disseminated as expected.
	ihaveCtlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIHave(messageCount, 1000, unknownTopic.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ihaveCtlMsgs)

	unittest.RequireCloseBefore(t, done, 2*time.Second, "failed to inspect RPC messages on time")
	require.Equal(t, uint64(1), invIhaveNotifCount.Load())
}

// TestValidationInspector_RateLimitedPeer_Detection ensures that the control message validation inspector rate limits peers per control message type as expected and
// the expected invalid control message notification is disseminated with the expected error.
func TestValidationInspector_RateLimitedPeer_Detection(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	inspectorConfig.NumberOfWorkers = 1

	// here we set the message count to the amount of flow channels
	// so that we can generate a valid ctl msg with all valid topics.
	flowChannels := channels.Channels()
	messageCount := flowChannels.Len()
	controlMessageCount := int64(1)

	count := atomic.NewInt64(0)
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	done := make(chan struct{})
	// ensure expected notifications are disseminated with expected error
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, validation.IsErrRateLimitedControlMsg(notification.Err))
			require.Equal(t, uint64(messageCount), notification.Count)
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2pmsg.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			default:
				require.Fail(t, "unexpected control message type")
			}
			if count.Load() == 4 {
				close(done)
			}
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(4, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Maybe()

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopTestComponents(t, cancel, nodes, validationInspector)

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
	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	require.Equal(t, uint64(2), invGraftNotifCount.Load())
	require.Equal(t, uint64(2), invPruneNotifCount.Load())
}

// TestValidationInspector_InvalidTopicId_Detection ensures that when an RPC control message contains an invalid topic ID an invalid control message
// notification is disseminated with the expected error.
// An invalid topic ID could have any of the following properties:
// - unknown topic: the topic is not a known Flow topic
// - malformed topic: topic is malformed in some way
// - invalid spork ID: spork ID prepended to topic and current spork ID do not match
func TestValidationInspector_InvalidTopicId_Detection(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// if GRAFT/PRUNE message count is higher than hard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	// set safety thresholds to 0 to force inspector to validate all control messages
	inspectorConfig.PruneLimits.SafetyThreshold = 0
	inspectorConfig.GraftLimits.SafetyThreshold = 0
	// set hard threshold to 0 so that in the case of invalid cluster ID
	// we force the inspector to return an error
	inspectorConfig.ClusterPrefixHardThreshold = 0
	inspectorConfig.IHaveLimits.SafetyThreshold = 0
	inspectorConfig.IHaveLimits.HardThreshold = 50
	inspectorConfig.IHaveAsyncInspectSampleSizePercentage = .5
	inspectorConfig.IHaveInspectionMaxSampleSize = 100
	ihaveMessageCount := 100
	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < HardThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := inspectorConfig.GraftLimits.SafetyThreshold + 1
	controlMessageCount := int64(1)

	count := atomic.NewUint64(0)
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	invIHaveNotifCount := atomic.NewUint64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 9
	// ensure expected notifications are disseminated with expected error
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, channels.IsInvalidTopicErr(notification.Err))
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				invGraftNotifCount.Inc()
				require.Equal(t, messageCount, notification.Count)
			case p2pmsg.CtrlMsgPrune:
				invPruneNotifCount.Inc()
				require.Equal(t, messageCount, notification.Count)
			case p2pmsg.CtrlMsgIHave:
				require.Equal(t, uint64(ihaveMessageCount), notification.Count)
				invIHaveNotifCount.Inc()
			default:
				require.Fail(t, "unexpected control message type")
			}
			if count.Load() == uint64(expectedNumOfTotalNotif) {
				close(done)
			}
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(expectedNumOfTotalNotif, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Maybe()

	// create unknown topic
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", corruptlibp2p.GossipSubTopicIdFixture(), sporkID))
	// create malformed topic
	malformedTopic := channels.Topic("!@#$%^&**((")
	// a topics spork ID is considered invalid if it does not match the current spork ID
	invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture()))

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopTestComponents(t, cancel, nodes, validationInspector)

	// prepare to spam - generate control messages
	graftCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), unknownTopic.String()))
	graftCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), malformedTopic.String()))
	graftCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), invalidSporkIDTopic.String()))

	pruneCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), unknownTopic.String()))
	pruneCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), malformedTopic.String()))
	pruneCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), invalidSporkIDTopic.String()))

	iHaveCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIHave(ihaveMessageCount, 1000, unknownTopic.String()))
	iHaveCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIHave(ihaveMessageCount, 1000, malformedTopic.String()))
	iHaveCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIHave(ihaveMessageCount, 1000, invalidSporkIDTopic.String()))

	// spam the victim peer with invalid graft messages
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsInvalidSporkIDTopic)

	// spam the victim peer with invalid prune messages
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsInvalidSporkIDTopic)

	// spam the victim peer with invalid ihave messages
	spammer.SpamControlMessage(t, victimNode, iHaveCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, iHaveCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, iHaveCtlMsgsInvalidSporkIDTopic)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")

	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	// we send 3 messages with 3 diff invalid topics
	require.Equal(t, uint64(3), invGraftNotifCount.Load())
	require.Equal(t, uint64(3), invPruneNotifCount.Load())
	require.Equal(t, uint64(3), invIHaveNotifCount.Load())
}

// TestValidationInspector_DuplicateTopicId_Detection ensures that when an RPC control message contains a duplicate topic ID an invalid control message
// notification is disseminated with the expected error.
func TestValidationInspector_DuplicateTopicId_Detection(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// if GRAFT/PRUNE message count is higher than hard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	// set safety thresholds to 0 to force inspector to validate all control messages
	inspectorConfig.PruneLimits.SafetyThreshold = 0
	inspectorConfig.GraftLimits.SafetyThreshold = 0
	// set hard threshold to 0 so that in the case of invalid cluster ID
	// we force the inspector to return an error
	inspectorConfig.ClusterPrefixHardThreshold = 0
	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < HardThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := inspectorConfig.GraftLimits.SafetyThreshold + 3
	controlMessageCount := int64(1)

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 2
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, validation.IsDuplicateFoundErr(notification.Err))
			require.Equal(t, messageCount, notification.Count)
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2pmsg.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			default:
				require.Fail(t, "unexpected control message type")
			}
			if count.Load() == int64(expectedNumOfTotalNotif) {
				close(done)
			}
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(expectedNumOfTotalNotif, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Maybe()

	// a topics spork ID is considered invalid if it does not match the current spork ID
	duplicateTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID))

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopTestComponents(t, cancel, nodes, validationInspector)

	// prepare to spam - generate control messages
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), duplicateTopic.String()))

	pruneCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), duplicateTopic.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsDuplicateTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsDuplicateTopic)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	require.Equal(t, uint64(1), invGraftNotifCount.Load())
	require.Equal(t, uint64(1), invPruneNotifCount.Load())
}

// TestValidationInspector_UnknownClusterId_Detection ensures that when an RPC control message contains a topic with an unknown cluster ID an invalid control message
// notification is disseminated with the expected error.
func TestValidationInspector_UnknownClusterId_Detection(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// if GRAFT/PRUNE message count is higher than hard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	// set safety thresholds to 0 to force inspector to validate all control messages
	inspectorConfig.PruneLimits.SafetyThreshold = 0
	inspectorConfig.GraftLimits.SafetyThreshold = 0
	// set hard threshold to 0 so that in the case of invalid cluster ID
	// we force the inspector to return an error
	inspectorConfig.ClusterPrefixHardThreshold = 0
	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < HardThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := inspectorConfig.GraftLimits.SafetyThreshold + 1
	controlMessageCount := int64(1)

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 2
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, channels.IsUnknownClusterIDErr(notification.Err))
			require.Equal(t, messageCount, notification.Count)
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2pmsg.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			default:
				require.Fail(t, "unexpected control message type")
			}
			if count.Load() == int64(expectedNumOfTotalNotif) {
				close(done)
			}
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(expectedNumOfTotalNotif, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Times(3)

	// setup cluster prefixed topic with an invalid cluster ID
	unknownClusterID := channels.Topic(channels.SyncCluster("unknown-cluster-ID"))
	// consume cluster ID update so that active cluster IDs set
	validationInspector.ActiveClustersChanged(flow.ChainIDList{"known-cluster-id"})

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopTestComponents(t, cancel, nodes, validationInspector)

	// prepare to spam - generate control messages
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), unknownClusterID.String()))
	pruneCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), unknownClusterID.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsDuplicateTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsDuplicateTopic)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	require.Equal(t, uint64(1), invGraftNotifCount.Load())
	require.Equal(t, uint64(1), invPruneNotifCount.Load())
}

// TestValidationInspector_ActiveClusterIdsNotSet_Graft_Detection ensures that an error is returned only after the cluster prefixed topics received for a peer exceed the configured
// cluster prefix hard threshold when the active cluster IDs not set and an invalid control message notification is disseminated with the expected error.
// This test involves Graft control messages.
func TestValidationInspector_ActiveClusterIdsNotSet_Graft_Detection(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// if GRAFT/PRUNE message count is higher than hard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	inspectorConfig.GraftLimits.SafetyThreshold = 0
	inspectorConfig.ClusterPrefixHardThreshold = 5
	inspectorConfig.NumberOfWorkers = 1
	controlMessageCount := int64(10)

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 5
	invGraftNotifCount := atomic.NewUint64(0)
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.True(t, validation.IsErrActiveClusterIDsNotSet(notification.Err))
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			default:
				require.Fail(t, "unexpected control message type")
			}
			require.Equal(t, uint64(1), notification.Count)
			if count.Load() == int64(expectedNumOfTotalNotif) {
				close(done)
			}
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(expectedNumOfTotalNotif, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Times(int(controlMessageCount + 1))

	// we deliberately avoid setting the cluster IDs so that we eventually receive errors after we have exceeded the allowed cluster
	// prefixed hard threshold
	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopTestComponents(t, cancel, nodes, validationInspector)
	// generate multiple control messages with GRAFT's for randomly generated
	// cluster prefixed channels, this ensures we do not encounter duplicate topic ID errors
	ctlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount),
		corruptlibp2p.WithGraft(1, randomClusterPrefixedTopic().String()),
	)
	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ctlMsgs)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	require.Equal(t, uint64(5), invGraftNotifCount.Load())
}

// TestValidationInspector_ActiveClusterIdsNotSet_Prune_Detection ensures that an error is returned only after the cluster prefixed topics received for a peer exceed the configured
// cluster prefix hard threshold when the active cluster IDs not set and an invalid control message notification is disseminated with the expected error.
// This test involves Prune control messages.
func TestValidationInspector_ActiveClusterIdsNotSet_Prune_Detection(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// if GRAFT/PRUNE message count is higher than hard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	inspectorConfig.PruneLimits.SafetyThreshold = 0
	inspectorConfig.ClusterPrefixHardThreshold = 5
	inspectorConfig.NumberOfWorkers = 1
	controlMessageCount := int64(10)

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 5
	invPruneNotifCount := atomic.NewUint64(0)
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.True(t, validation.IsErrActiveClusterIDsNotSet(notification.Err))
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			switch notification.MsgType {
			case p2pmsg.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			default:
				require.Fail(t, "unexpected control message type")
			}
			require.Equal(t, uint64(1), notification.Count)
			if count.Load() == int64(expectedNumOfTotalNotif) {
				close(done)
			}
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(expectedNumOfTotalNotif, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Times(int(controlMessageCount + 1))

	// we deliberately avoid setting the cluster IDs so that we eventually receive errors after we have exceeded the allowed cluster
	// prefixed hard threshold
	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopTestComponents(t, cancel, nodes, validationInspector)
	// generate multiple control messages with GRAFT's for randomly generated
	// cluster prefixed channels, this ensures we do not encounter duplicate topic ID errors
	ctlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount),
		corruptlibp2p.WithPrune(1, randomClusterPrefixedTopic().String()),
	)
	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ctlMsgs)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	require.Equal(t, uint64(5), invPruneNotifCount.Load())
}

// TestValidationInspector_UnstakedNode_Detection ensures that RPC control message inspector disseminates an invalid control message notification when an unstaked peer
// sends a control message for a cluster prefixed topic.
func TestValidationInspector_UnstakedNode_Detection(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// if GRAFT/PRUNE message count is higher than hard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	// set safety thresholds to 0 to force inspector to validate all control messages
	inspectorConfig.PruneLimits.SafetyThreshold = 0
	inspectorConfig.GraftLimits.SafetyThreshold = 0
	// set hard threshold to 0 so that in the case of invalid cluster ID
	// we force the inspector to return an error
	inspectorConfig.ClusterPrefixHardThreshold = 0
	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < HardThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := inspectorConfig.GraftLimits.SafetyThreshold + 1
	controlMessageCount := int64(1)

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 2
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, validation.IsErrUnstakedPeer(notification.Err))
			require.Equal(t, messageCount, notification.Count)
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2pmsg.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			default:
				require.Fail(t, "unexpected control message type")
			}
			if count.Load() == int64(expectedNumOfTotalNotif) {
				close(done)
			}
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(expectedNumOfTotalNotif, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(nil, false).Times(3)

	// setup cluster prefixed topic with an invalid cluster ID
	clusterID := flow.ChainID("known-cluster-id")
	clusterIDTopic := channels.Topic(channels.SyncCluster(clusterID))
	// consume cluster ID update so that active cluster IDs set
	validationInspector.ActiveClustersChanged(flow.ChainIDList{clusterID})

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopTestComponents(t, cancel, nodes, validationInspector)

	// prepare to spam - generate control messages
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), clusterIDTopic.String()))
	pruneCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), clusterIDTopic.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsDuplicateTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsDuplicateTopic)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	require.Equal(t, uint64(1), invGraftNotifCount.Load())
	require.Equal(t, uint64(1), invPruneNotifCount.Load())
}

// TestValidationInspector_InspectIWants_DuplicateMessageIDs ensures that expected invalid control message notification is disseminated when iWant control messages containing
// duplicate messages Ids are encountered during validation.
func TestValidationInspector_InspectIWants_DuplicateMessageIDs(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	inspectorConfig.NumberOfWorkers = 1

	messageCount := 2
	controlMessageCount := int64(1)
	duplicateMessageIDNotifCount := atomic.NewUint64(0)
	done := make(chan struct{})
	// ensure expected notifications are disseminated with expected error
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.Equal(t, uint64(messageCount), notification.Count)
			require.True(t, validation.IsDuplicateFoundErr(notification.Err))
			duplicateMessageIDNotifCount.Inc()
			if duplicateMessageIDNotifCount.Load() == 2 {
				close(done)
			}
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(2, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := mockp2p.NewPubSubTracer(t)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Maybe()

	duplicateMessageIDs := corruptlibp2p.GossipSubMessageIdsFixture(10)
	// create iWants control messages where the first 2 iwants contain a duplicate message ID "duplicate"
	iWantFactory := func() []pb.ControlMessage {
		ctlWithiWantDuplicateMsgIDs := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIWant(messageCount, messageCount))
		// add some non duplicates to our duplicate message IDs
		ctlWithiWantDuplicateMsgIDs[0].Iwant[0].MessageIDs = append(duplicateMessageIDs, corruptlibp2p.GossipSubMessageIdsFixture(5)...)
		ctlWithiWantDuplicateMsgIDs[0].Iwant[1].MessageIDs = append(duplicateMessageIDs, corruptlibp2p.GossipSubMessageIdsFixture(5)...)
		return ctlWithiWantDuplicateMsgIDs
	}

	// avoid cache miss threshold errors
	meshTracer.On("WasIHaveRPCSent", mockery.AnythingOfType("string")).Return(true).Maybe()
	meshTracer.On("LastHighestIHaveRPCSize").Return(int64(100))

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopTestComponents(t, cancel, nodes, validationInspector)

	// spam the victim with 2 iWant control messages that contain duplicate message IDs, we expect to observe 2 invalid control message notifications disseminated
	spammer.SpamControlMessage(t, victimNode, iWantFactory())
	spammer.SpamControlMessage(t, victimNode, iWantFactory())

	unittest.RequireCloseBefore(t, done, 2*time.Second, "failed to inspect RPC messages on time")
}

// TestValidationInspector_InspectIWants_CacheMissThreshold ensures that expected invalid control message notification is disseminated when the number of iWant message Ids
// without a corresponding iHave message sent with the same message ID exceeds the configured cache miss threshold.
func TestValidationInspector_InspectIWants_CacheMissThreshold(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.IWantRPCInspectionConfig.CacheMissThreshold = .5 // set cache miss threshold to 50%
	messageCount := 1
	controlMessageCount := int64(1)
	cacheMissThresholdNotifCount := atomic.NewUint64(0)
	done := make(chan struct{})
	// ensure expected notifications are disseminated with expected error
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.Equal(t, uint64(messageCount), notification.Count)
			require.True(t, validation.IsIWantCacheMissThresholdErr(notification.Err))
			cacheMissThresholdNotifCount.Inc()
			if cacheMissThresholdNotifCount.Load() == 1 {
				close(done)
			}
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(1, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Maybe()

	messageIDs := corruptlibp2p.GossipSubMessageIdsFixture(10)

	// create control message with iWant that contains 5 message IDs that were not tracked
	ctlWithIWants := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIWant(messageCount, messageCount))
	ctlWithIWants[0].Iwant[0].MessageIDs = messageIDs // the first 5 message ids will not have a corresponding iHave

	// create control message with iHave that contains only the last 4 message IDs, this will force cache misses for the other 6 message IDs
	ctlWithIhaves := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIHave(messageCount, messageCount, channels.PushBlocks.String()))
	ctlWithIhaves[0].Ihave[0].MessageIDs = messageIDs[6:]

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	meshTracer.Start(signalerCtx)
	defer stopTestComponents(t, cancel, nodes, validationInspector, meshTracer)

	// simulate tracking some message IDs
	meshTracer.SendRPC(&pubsub.RPC{
		RPC: pb.RPC{
			Control: &ctlWithIhaves[0],
		},
	}, "")

	// spam the victim with iWant message that contains message IDs that do not have a corresponding iHave
	spammer.SpamControlMessage(t, victimNode, ctlWithIWants)

	unittest.RequireCloseBefore(t, done, 2*time.Second, "failed to inspect RPC messages on time")
}

// TestValidationInspector_InspectIWants_DuplicateMsgIDThreshold ensures that expected invalid control message notification is disseminated when the number
// of duplicate message Ids in a single iWant control message exceeds the configured duplicate message ID threshold.
func TestValidationInspector_InspectIWants_DuplicateMsgIDThreshold(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.IWantRPCInspectionConfig.DuplicateMsgIDThreshold = .5 // set duplicate message id threshold to 50%
	inspectorConfig.IWantRPCInspectionConfig.CacheMissThreshold = 2       // avoid throwing cache miss threshold error
	messageCount := 1
	controlMessageCount := int64(1)
	done := make(chan struct{})
	// ensure expected notifications are disseminated with expected error
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			defer close(done)
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.Equal(t, uint64(messageCount), notification.Count)
			require.True(t, validation.IsIWantDuplicateMsgIDThresholdErr(notification.Err))
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	mockDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(1, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	validationInspector, err := validation.NewControlMsgValidationInspector(
		unittest.Logger(),
		sporkID,
		&inspectorConfig,
		distributor,
		metrics.NewNoopCollector(),
		metrics.NewNoopCollector(),
		idProvider,
		metrics.NewNoopCollector(),
		meshTracer)
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		p2ptest.WithGossipSubTracer(meshTracer),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.On("ByPeerID", victimNode.Host().ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.Host().ID()).Return(&spammer.SpammerId, true).Maybe()

	messageIDs := corruptlibp2p.GossipSubMessageIdsFixture(10)

	// create control message with iWant that contains 5 message IDs that were not tracked
	ctlMsg := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithIWant(messageCount, messageCount))
	ctlMsg[0].Iwant[0].MessageIDs = messageIDs
	// set first 6 message ids to duplicate id > 50% duplicate message ID threshold
	for i := 0; i <= 6; i++ {
		ctlMsg[0].Iwant[0].MessageIDs[i] = messageIDs[0]
	}

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	meshTracer.Start(signalerCtx)
	defer stopTestComponents(t, cancel, nodes, validationInspector, meshTracer)

	// spam the victim with iWant message that contains 60% duplicate message ids more than the configured 50% allowed threshold
	spammer.SpamControlMessage(t, victimNode, ctlMsg)

	unittest.RequireCloseBefore(t, done, 2*time.Second, "failed to inspect RPC messages on time")
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
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, flow.RoleConsensus, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	victimNode, victimId := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.EnablePeerScoringWithOverride(p2p.PeerScoringConfigNoOverride),
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
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)
	spammer.Start(t)

	// wait for the nodes to discover each other
	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// as nodes started fresh and no spamming has happened yet, the nodes should be able to exchange messages on the topic.
	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkID)
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
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
	p2ptest.EnsureNoPubsubExchangeBetweenGroups(t, ctx, []p2p.LibP2PNode{victimNode}, []p2p.LibP2PNode{spammer.SpammerNode}, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})
}
