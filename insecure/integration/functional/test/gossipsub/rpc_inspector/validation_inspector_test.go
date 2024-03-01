package rpc_inspector

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestValidationInspector_InvalidTopicId_Detection ensures that when an RPC control message contains an invalid topic ID an invalid control message
// notification is disseminated with the expected error.
// An invalid topic ID could have any of the following properties:
// - unknown topic: the topic is not a known Flow topic
// - malformed topic: topic is malformed in some way
// - invalid spork ID: spork ID prepended to topic and current spork ID do not match
func TestValidationInspector_InvalidTopicId_Detection(t *testing.T) {
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation

	messageCount := 100
	inspectorConfig.InspectionQueue.NumberOfWorkers = 1
	controlMessageCount := int64(1)

	count := atomic.NewUint64(0)
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	invIHaveNotifCount := atomic.NewUint64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 9

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	consumer := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
	consumer.On("OnInvalidControlMessageNotification", mockery.Anything).Run(func(args mockery.Arguments) {
		count.Inc()
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, notification.TopicType, p2p.CtrlMsgNonClusterTopicType, "IsClusterPrefixed is expected to be false, no RPC with cluster prefixed topic sent in this test")
		require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
		require.True(t, validation.IsInvalidTopicIDThresholdExceeded(notification.Error))
		switch notification.MsgType {
		case p2pmsg.CtrlMsgGraft:
			invGraftNotifCount.Inc()
		case p2pmsg.CtrlMsgPrune:
			invPruneNotifCount.Inc()
		case p2pmsg.CtrlMsgIHave:
			invIHaveNotifCount.Inc()
		default:
			require.Fail(t, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Error))
		}
		if count.Load() == uint64(expectedNumOfTotalNotif) {
			close(done)
		}
	}).Return().Times(expectedNumOfTotalNotif)

	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := p2ptest.NewUpdatableTopicProviderFixture()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
		InvalidControlMessageNotificationConsumer: consumer,
		TopicOracle: func() p2p.TopicProvider {
			return topicProvider
		},
	})
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(), corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)))
	idProvider.On("ByPeerID", victimNode.ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true).Maybe()

	// create unknown topic
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", p2ptest.GossipSubTopicIdFixture(), sporkID))
	// create malformed topic
	malformedTopic := channels.Topic(unittest.RandomStringFixture(t, 100))
	// a topics spork ID is considered invalid if it does not match the current spork ID
	invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture()))

	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topicProvider.UpdateTopics([]string{unknownTopic.String(), malformedTopic.String(), invalidSporkIDTopic.String()})

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, validationInspector)

	// prepare to spam - generate control messages
	graftCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithGraft(messageCount, unknownTopic.String()))
	graftCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithGraft(messageCount, malformedTopic.String()))
	graftCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithGraft(messageCount, invalidSporkIDTopic.String()))

	pruneCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithPrune(messageCount, unknownTopic.String()))
	pruneCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithPrune(messageCount, malformedTopic.String()))
	pruneCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithPrune(messageCount, invalidSporkIDTopic.String()))

	iHaveCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithIHave(messageCount, 1000, unknownTopic.String()))
	iHaveCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithIHave(messageCount, 1000, malformedTopic.String()))
	iHaveCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithIHave(messageCount, 1000, invalidSporkIDTopic.String()))

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
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation

	inspectorConfig.InspectionQueue.NumberOfWorkers = 1

	// sets the message count to the max of the duplicate topic id threshold for graft and prune control messages to ensure
	// a successful attack
	messageCount := int(math.Max(float64(inspectorConfig.IHave.DuplicateTopicIdThreshold), float64(inspectorConfig.GraftPrune.DuplicateTopicIdThreshold))) + 2
	controlMessageCount := int64(1)

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 3
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	invIHaveNotifCount := atomic.NewUint64(0)

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	consumer := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
	consumer.On("OnInvalidControlMessageNotification", mockery.Anything).Run(func(args mockery.Arguments) {
		count.Inc()
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, notification.TopicType, p2p.CtrlMsgNonClusterTopicType, "IsClusterPrefixed is expected to be false, no RPC with cluster prefixed topic sent in this test")
		require.True(t, validation.IsDuplicateTopicIDThresholdExceeded(notification.Error))
		require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
		switch notification.MsgType {
		case p2pmsg.CtrlMsgGraft:
			invGraftNotifCount.Inc()
		case p2pmsg.CtrlMsgPrune:
			invPruneNotifCount.Inc()
		case p2pmsg.CtrlMsgIHave:
			invIHaveNotifCount.Inc()
		default:
			require.Fail(t, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Error))
		}

		if count.Load() == int64(expectedNumOfTotalNotif) {
			close(done)
		}
	}).Return().Times(expectedNumOfTotalNotif)

	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := p2ptest.NewUpdatableTopicProviderFixture()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
		InvalidControlMessageNotificationConsumer: consumer,
		TopicOracle: func() p2p.TopicProvider {
			return topicProvider
		},
	})
	require.NoError(t, err)

	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(), corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)))
	idProvider.On("ByPeerID", victimNode.ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true).Maybe()

	// a topics spork ID is considered invalid if it does not match the current spork ID
	duplicateTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID))
	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topicProvider.UpdateTopics([]string{duplicateTopic.String()})

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, validationInspector)

	// prepare to spam - generate control messages
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithGraft(messageCount, duplicateTopic.String()))
	ihaveCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithIHave(messageCount, 10, duplicateTopic.String()))
	pruneCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithPrune(messageCount, duplicateTopic.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsDuplicateTopic)
	spammer.SpamControlMessage(t, victimNode, ihaveCtlMsgsDuplicateTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsDuplicateTopic)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	require.Equal(t, uint64(1), invGraftNotifCount.Load())
	require.Equal(t, uint64(1), invPruneNotifCount.Load())
	require.Equal(t, uint64(1), invIHaveNotifCount.Load())
}

// TestValidationInspector_IHaveDuplicateMessageId_Detection ensures that when an RPC iHave control message contains a duplicate message ID for a single topic
// notification is disseminated with the expected error.
func TestValidationInspector_IHaveDuplicateMessageId_Detection(t *testing.T) {
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation
	inspectorConfig.InspectionQueue.NumberOfWorkers = 1

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 1
	invIHaveNotifCount := atomic.NewUint64(0)
	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	consumer := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
	consumer.On("OnInvalidControlMessageNotification", mockery.Anything).Run(func(args mockery.Arguments) {
		count.Inc()
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, notification.TopicType, p2p.CtrlMsgNonClusterTopicType, "IsClusterPrefixed is expected to be false, no RPC with cluster prefixed topic sent in this test")
		require.True(t, validation.IsDuplicateMessageIDErr(notification.Error))
		require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
		require.True(t, notification.MsgType == p2pmsg.CtrlMsgIHave, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Error))
		invIHaveNotifCount.Inc()

		if count.Load() == int64(expectedNumOfTotalNotif) {
			close(done)
		}
	}).Return().Times(expectedNumOfTotalNotif)

	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := p2ptest.NewUpdatableTopicProviderFixture()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
		InvalidControlMessageNotificationConsumer: consumer,
		TopicOracle: func() p2p.TopicProvider {
			return topicProvider
		},
	})
	require.NoError(t, err)

	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(), corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)))
	idProvider.On("ByPeerID", victimNode.ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true).Maybe()

	// prepare to spam - generate control messages
	pushBlocks := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID))
	reqChunks := channels.Topic(fmt.Sprintf("%s/%s", channels.RequestChunks, sporkID))
	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topicProvider.UpdateTopics([]string{pushBlocks.String(), reqChunks.String()})

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	// to suppress peers provider not set
	p2ptest.RegisterPeerProviders(t, nodes)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, validationInspector)

	// generate 2 control messages with iHaves for different topics
	messageIdCount := inspectorConfig.IHave.DuplicateMessageIdThreshold + 2
	messageIds := unittest.IdentifierListFixture(1)
	for i := 0; i < messageIdCount; i++ {
		messageIds = append(messageIds, messageIds[0])
	}
	// prepares 2 control messages with iHave messages for different topics with duplicate message IDs
	ihaveCtlMsgs1 := spammer.GenerateCtlMessages(
		1,
		p2ptest.WithIHaveMessageIDs(messageIds.Strings(), pushBlocks.String()),
		p2ptest.WithIHaveMessageIDs(messageIds.Strings(), reqChunks.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ihaveCtlMsgs1)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
	// ensure we receive the expected number of invalid control message notifications
	require.Equal(t, uint64(1), invIHaveNotifCount.Load())
}

// TestValidationInspector_UnknownClusterId_Detection ensures that when an RPC control message contains a topic with an unknown cluster ID an invalid control message
// notification is disseminated with the expected error.
func TestValidationInspector_UnknownClusterId_Detection(t *testing.T) {
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation
	// set hard threshold to 0 so that in the case of invalid cluster ID
	// we force the inspector to return an error
	inspectorConfig.ClusterPrefixedMessage.HardThreshold = 0
	inspectorConfig.InspectionQueue.NumberOfWorkers = 1
	// set invalid topic id threshold to 0 so that inspector returns error early
	inspectorConfig.GraftPrune.InvalidTopicIdThreshold = 0

	// ensure we send a number of message with unknown cluster ids higher than the invalid topic ids threshold
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := 60
	controlMessageCount := int64(1)

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 2
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	consumer := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
	consumer.On("OnInvalidControlMessageNotification", mockery.Anything).Run(func(args mockery.Arguments) {
		count.Inc()
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, notification.TopicType, p2p.CtrlMsgTopicTypeClusterPrefixed)
		require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
		require.True(t, validation.IsInvalidTopicIDThresholdExceeded(notification.Error))
		switch notification.MsgType {
		case p2pmsg.CtrlMsgGraft:
			invGraftNotifCount.Inc()
		case p2pmsg.CtrlMsgPrune:
			invPruneNotifCount.Inc()
		default:
			require.Fail(t, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Error))
		}

		if count.Load() == int64(expectedNumOfTotalNotif) {
			close(done)
		}
	}).Return().Times(expectedNumOfTotalNotif)

	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := p2ptest.NewUpdatableTopicProviderFixture()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
		InvalidControlMessageNotificationConsumer: consumer,
		TopicOracle: func() p2p.TopicProvider {
			return topicProvider
		},
	})
	require.NoError(t, err)

	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(), corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)))
	idProvider.On("ByPeerID", victimNode.ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true)

	// setup cluster prefixed topic with an invalid cluster ID
	unknownClusterID := channels.Topic(channels.SyncCluster("unknown-cluster-ID"))
	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topicProvider.UpdateTopics([]string{unknownClusterID.String()})

	// consume cluster ID update so that active cluster IDs set
	validationInspector.ActiveClustersChanged(flow.ChainIDList{"known-cluster-id"})

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, validationInspector)

	// prepare to spam - generate control messages
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithGraft(messageCount, unknownClusterID.String()))
	pruneCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithPrune(messageCount, unknownClusterID.String()))

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
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation
	inspectorConfig.GraftPrune.InvalidTopicIdThreshold = 0
	inspectorConfig.ClusterPrefixedMessage.HardThreshold = 5
	inspectorConfig.InspectionQueue.NumberOfWorkers = 1

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	consumer := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
	consumer.On("OnInvalidControlMessageNotification", mockery.Anything).Run(func(args mockery.Arguments) {
		count.Inc()
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, notification.TopicType, p2p.CtrlMsgTopicTypeClusterPrefixed)
		require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
		require.True(t, validation.IsInvalidTopicIDThresholdExceeded(notification.Error))
		require.Equal(t, notification.MsgType, p2pmsg.CtrlMsgGraft)
		if count.Load() == 1 {
			close(done)
		}
	}).Return().Once()
	meshTracer := meshTracerFixture(flowConfig, idProvider)

	topicProvider := p2ptest.NewUpdatableTopicProviderFixture()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
		InvalidControlMessageNotificationConsumer: consumer,
		TopicOracle: func() p2p.TopicProvider {
			return topicProvider
		},
	})
	require.NoError(t, err)

	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(), corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)))
	idProvider.On("ByPeerID", victimNode.ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true)
	topics := randomClusterPrefixedTopics(int(inspectorConfig.ClusterPrefixedMessage.HardThreshold) + 1)
	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topicProvider.UpdateTopics(topics)

	// we deliberately avoid setting the cluster IDs so that we eventually receive errors after we have exceeded the allowed cluster
	// prefixed hard threshold
	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, validationInspector)
	// generate multiple control messages with GRAFT's for randomly generated
	// cluster prefixed channels, this ensures we do not encounter duplicate topic ID errors
	ctlMsgs := spammer.GenerateCtlMessages(1, p2ptest.WithGrafts(topics...))
	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ctlMsgs)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
}

// TestValidationInspector_ActiveClusterIdsNotSet_Prune_Detection ensures that an error is returned only after the cluster prefixed topics received for a peer exceed the configured
// cluster prefix hard threshold when the active cluster IDs not set and an invalid control message notification is disseminated with the expected error.
// This test involves Prune control messages.
func TestValidationInspector_ActiveClusterIdsNotSet_Prune_Detection(t *testing.T) {
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation
	inspectorConfig.GraftPrune.InvalidTopicIdThreshold = 0
	inspectorConfig.ClusterPrefixedMessage.HardThreshold = 5
	inspectorConfig.InspectionQueue.NumberOfWorkers = 1

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	consumer := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
	consumer.On("OnInvalidControlMessageNotification", mockery.Anything).Run(func(args mockery.Arguments) {
		count.Inc()
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, notification.TopicType, p2p.CtrlMsgTopicTypeClusterPrefixed)
		require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
		require.True(t, validation.IsInvalidTopicIDThresholdExceeded(notification.Error))
		require.Equal(t, notification.MsgType, p2pmsg.CtrlMsgPrune)
		if count.Load() == 1 {
			close(done)
		}
	}).Return().Once()
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := p2ptest.NewUpdatableTopicProviderFixture()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
		InvalidControlMessageNotificationConsumer: consumer,
		TopicOracle: func() p2p.TopicProvider {
			return topicProvider
		},
	})
	require.NoError(t, err)

	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(), corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)))
	idProvider.On("ByPeerID", victimNode.ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true)
	topics := randomClusterPrefixedTopics(int(inspectorConfig.ClusterPrefixedMessage.HardThreshold) + 1)
	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topicProvider.UpdateTopics(topics)

	// we deliberately avoid setting the cluster IDs so that we eventually receive errors after we have exceeded the allowed cluster
	// prefixed hard threshold
	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, validationInspector)
	// generate multiple control messages with prunes for randomly generated
	// cluster prefixed channels, this ensures we do not encounter duplicate topic ID errors
	ctlMsgs := spammer.GenerateCtlMessages(1, p2ptest.WithPrunes(topics...))
	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ctlMsgs)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
}

// TestValidationInspector_Unstaked_Node_Detection ensures that RPC control message inspector disseminates an invalid control message notification when an unstaked peer
// sends an RPC.
func TestValidationInspector_UnstakedNode_Detection(t *testing.T) {
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation
	inspectorConfig.InspectionQueue.NumberOfWorkers = 1
	controlMessageCount := int64(1)

	count := atomic.NewInt64(0)
	done := make(chan struct{})

	idProvider := mock.NewIdentityProvider(t)
	inspectorIDProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	unstakedPeerID := unittest.PeerIdFixture(t)
	consumer := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
	consumer.On("OnInvalidControlMessageNotification", mockery.Anything).Run(func(args mockery.Arguments) {
		count.Inc()
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, notification.TopicType, p2p.CtrlMsgNonClusterTopicType)
		require.Equal(t, unstakedPeerID, notification.PeerID)
		require.True(t, validation.IsErrUnstakedPeer(notification.Error))
		require.Equal(t, notification.MsgType, p2pmsg.CtrlMsgRPC)

		if count.Load() == 2 {
			close(done)
		}
	}).Return()

	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := p2ptest.NewUpdatableTopicProviderFixture()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		IdProvider:              inspectorIDProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
		InvalidControlMessageNotificationConsumer: consumer,
		TopicOracle: func() p2p.TopicProvider {
			return topicProvider
		},
	})
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	// we need to wait until nodes are connected before we can start returning unstaked identity.
	nodesConnected := atomic.NewBool(false)
	victimNode, victimIdentity := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(), corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
			if nodesConnected.Load() {
				// after nodes are connected invoke corrupt callback with an unstaked peer ID
				return corruptInspectorFunc(unstakedPeerID, rpc)
			}
			return corruptInspectorFunc(id, rpc)
		})))
	idProvider.On("ByPeerID", victimNode.ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true).Maybe()
	inspectorIDProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true)
	inspectorIDProvider.On("ByPeerID", unstakedPeerID).Return(nil, false)

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	nodesConnected.Store(true)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, validationInspector)

	// prepare to spam - generate control messages each of which will be immediately rejected because the sender is unstaked
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithGraft(10, ""))
	pruneCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithPrune(10, ""))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsDuplicateTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsDuplicateTopic)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
}

// TestValidationInspector_InspectIWants_CacheMissThreshold ensures that expected invalid control message notification is disseminated when the number of iWant message Ids
// without a corresponding iHave message sent with the same message ID exceeds the configured cache miss threshold.
func TestValidationInspector_InspectIWants_CacheMissThreshold(t *testing.T) {
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation
	inspectorConfig.InspectionQueue.NumberOfWorkers = 1
	inspectorConfig.IWant.CacheMissThreshold = 10
	messageCount := 10
	controlMessageCount := int64(1)
	cacheMissThresholdNotifCount := atomic.NewUint64(0)
	done := make(chan struct{})

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	consumer := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
	consumer.On("OnInvalidControlMessageNotification", mockery.Anything).Run(func(args mockery.Arguments) {
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, notification.TopicType, p2p.CtrlMsgNonClusterTopicType, "IsClusterPrefixed is expected to be false, no RPC with cluster prefixed topic sent in this test")
		require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
		require.True(t, notification.MsgType == p2pmsg.CtrlMsgIWant, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Error))
		require.True(t, validation.IsIWantCacheMissThresholdErr(notification.Error))

		cacheMissThresholdNotifCount.Inc()
		if cacheMissThresholdNotifCount.Load() == 1 {
			close(done)
		}
	}).Return().Once()

	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := p2ptest.NewUpdatableTopicProviderFixture()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
		InvalidControlMessageNotificationConsumer: consumer,
		TopicOracle: func() p2p.TopicProvider {
			return topicProvider
		},
	})
	require.NoError(t, err)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(), corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)))
	idProvider.On("ByPeerID", victimNode.ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true).Maybe()

	messageIDs := p2ptest.GossipSubMessageIdsFixture(10)

	// create control message with iWant that contains 5 message IDs that were not tracked
	ctlWithIWants := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithIWant(messageCount, messageCount))
	ctlWithIWants[0].Iwant[0].MessageIDs = messageIDs // the first 5 message ids will not have a corresponding iHave
	topic := channels.PushBlocks
	// create control message with iHave that contains only the last 4 message IDs, this will force cache misses for the other 6 message IDs
	ctlWithIhaves := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithIHave(messageCount, messageCount, topic.String()))
	ctlWithIhaves[0].Ihave[0].MessageIDs = messageIDs[6:]
	// set topic oracle
	topicProvider.UpdateTopics([]string{topic.String()})
	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	meshTracer.Start(signalerCtx)
	defer stopComponents(t, cancel, nodes, validationInspector, meshTracer)

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

// TestValidationInspector_InspectRpcPublishMessages ensures that expected invalid control message notification is disseminated when the number of errors encountered during
// RPC publish message validation exceeds the configured error threshold.
func TestValidationInspector_InspectRpcPublishMessages(t *testing.T) {
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	// create our RPC validation inspector
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation
	inspectorConfig.InspectionQueue.NumberOfWorkers = 1

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	controlMessageCount := int64(1)
	notificationCount := atomic.NewUint64(0)
	done := make(chan struct{})
	validTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.TestNetworkChannel.String(), sporkID)).String()
	// create unknown topic
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", p2ptest.GossipSubTopicIdFixture(), sporkID)).String()
	// create malformed topic
	malformedTopic := channels.Topic(unittest.RandomStringFixture(t, 100)).String()
	// a topics spork ID is considered invalid if it does not match the current spork ID
	invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture())).String()

	// unknown peer ID
	unknownPeerID := unittest.PeerIdFixture(t)

	// ejected identity
	ejectedIdentityPeerID := unittest.PeerIdFixture(t)
	ejectedIdentity := unittest.IdentityFixture()
	ejectedIdentity.EpochParticipationStatus = flow.EpochParticipationStatusEjected

	// invalid messages this should force a notification to disseminate
	invalidPublishMsgs := []*pb.Message{
		{Topic: &unknownTopic, From: []byte(spammer.SpammerNode.ID())},
		{Topic: &malformedTopic, From: []byte(spammer.SpammerNode.ID())},
		{Topic: &malformedTopic, From: []byte(spammer.SpammerNode.ID())},
		{Topic: &malformedTopic, From: []byte(spammer.SpammerNode.ID())},
		{Topic: &invalidSporkIDTopic, From: []byte(spammer.SpammerNode.ID())},
		{Topic: &validTopic, From: []byte(unknownPeerID)},
		{Topic: &validTopic, From: []byte(ejectedIdentityPeerID)},
	}
	topic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID))
	// first create 4 valid messages
	publishMsgs := unittest.GossipSubMessageFixtures(4, topic.String(), unittest.WithFrom(spammer.SpammerNode.ID()))
	publishMsgs = append(publishMsgs, invalidPublishMsgs...)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	consumer := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
	consumer.On("OnInvalidControlMessageNotification", mockery.Anything).Run(func(args mockery.Arguments) {
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, notification.TopicType, p2p.CtrlMsgNonClusterTopicType, "IsClusterPrefixed is expected to be false, no RPC with cluster prefixed topic sent in this test")
		require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
		require.True(t, notification.MsgType == p2pmsg.RpcPublishMessage, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Error))
		require.True(t, validation.IsInvalidRpcPublishMessagesErr(notification.Error))
		require.Contains(t,
			notification.Error.Error(),
			fmt.Sprintf("%d error(s) encountered", len(invalidPublishMsgs)),
			fmt.Sprintf("expected %d errors, an error for each invalid pubsub message", len(invalidPublishMsgs)))
		require.Contains(t, notification.Error.Error(), fmt.Sprintf("unstaked peer: %s", unknownPeerID))
		require.Contains(t, notification.Error.Error(), fmt.Sprintf("ejected peer: %s", ejectedIdentityPeerID))
		notificationCount.Inc()
		if notificationCount.Load() == 1 {
			close(done)
		}
	}).Return().Once()

	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := p2ptest.NewUpdatableTopicProviderFixture()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
		InvalidControlMessageNotificationConsumer: consumer,
		TopicOracle: func() p2p.TopicProvider {
			return topicProvider
		},
	})
	require.NoError(t, err)
	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topics := make([]string, len(publishMsgs))
	for i := 0; i < len(publishMsgs); i++ {
		topics[i] = publishMsgs[i].GetTopic()
	}

	topicProvider.UpdateTopics(topics)
	// after 7 errors encountered disseminate a notification
	inspectorConfig.PublishMessages.ErrorThreshold = 6
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(validationInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(), corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)))
	idProvider.On("ByPeerID", victimNode.ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true).Maybe()

	// return nil for unknown peer ID indicating unstaked peer
	idProvider.On("ByPeerID", unknownPeerID).Return(nil, false).Once()
	// return ejected identity for peer ID will force message validation failure
	idProvider.On("ByPeerID", ejectedIdentityPeerID).Return(ejectedIdentity, true).Once()

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	meshTracer.Start(signalerCtx)
	defer stopComponents(t, cancel, nodes, validationInspector, meshTracer)

	// prepare to spam - generate control messages
	ctlMsg := spammer.GenerateCtlMessages(int(controlMessageCount))
	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ctlMsg, publishMsgs...)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")
	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	require.Equal(t, uint64(1), notificationCount.Load())
}

// TestGossipSubSpamMitigationIntegration tests that the spam mitigation feature of GossipSub is working as expected.
// The test puts toghether the spam detection (through the GossipSubInspector) and the spam mitigation (through the
// scoring system) and ensures that the mitigation is triggered when the spam detection detects spam.
// The test scenario involves a spammer node that sends a large number of control messages to a victim node.
// The victim node is configured to use the GossipSubInspector to detect spam and the scoring system to mitigate spam.
// The test ensures that the victim node is disconnected from the spammer node on the GossipSub mesh after the spam detection is triggered.
func TestGossipSubSpamMitigationIntegration(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "https://github.com/dapperlabs/flow-go/issues/6949")
	t.Run("gossipsub spam mitigation invalid grafts", func(t *testing.T) {
		testGossipSubSpamMitigationIntegration(t, p2pmsg.CtrlMsgGraft)
	})
	t.Run("gossipsub spam mitigation invalid prunes", func(t *testing.T) {
		testGossipSubSpamMitigationIntegration(t, p2pmsg.CtrlMsgPrune)
	})
	t.Run("gossipsub spam mitigation invalid ihaves", func(t *testing.T) {
		testGossipSubSpamMitigationIntegration(t, p2pmsg.CtrlMsgIHave)
	})
}

// testGossipSubSpamMitigationIntegration tests that the spam mitigation feature of GossipSub is working as expected.
// The test puts together the spam detection (through the GossipSubInspector) and the spam mitigation (through the
// scoring system) and ensures that the mitigation is triggered when the spam detection detects spam.
// The test scenario involves a spammer node that sends a large number of control messages for the specified control message type to a victim node.
// The victim node is configured to use the GossipSubInspector to detect spam and the scoring system to mitigate spam.
// The test ensures that the victim node is disconnected from the spammer node on the GossipSub mesh after the spam detection is triggered.
func testGossipSubSpamMitigationIntegration(t *testing.T, msgType p2pmsg.ControlMessageType) {
	idProvider := mock.NewIdentityProvider(t)
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, flow.RoleConsensus, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// set the scoring parameters to be more aggressive to speed up the test
	cfg.NetworkConfig.GossipSub.RpcTracer.ScoreTracerInterval = 100 * time.Millisecond
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond
	victimNode, victimId := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.OverrideFlowConfig(cfg))

	ids := flow.IdentityList{&victimId, &spammer.SpammerId}
	idProvider.On("ByPeerID", mockery.Anything).Return(func(peerId peer.ID) *flow.Identity {
		switch peerId {
		case victimNode.ID():
			return &victimId
		case spammer.SpammerNode.ID():
			return &spammer.SpammerId
		default:
			return nil
		}

	}, func(peerId peer.ID) bool {
		switch peerId {
		case victimNode.ID():
			fallthrough
		case spammer.SpammerNode.ID():
			return true
		default:
			return false
		}
	})

	spamRpcCount := 50000           // total number of individual rpc messages to send
	spamCtrlMsgCount := int64(1000) // total number of control messages to send on each RPC

	// unknownTopic is an unknown topic to the victim node but shaped like a valid topic (i.e., it has the correct prefix and spork ID).
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", p2ptest.GossipSubTopicIdFixture(), sporkID))

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

	var unknownTopicSpam []pubsub_pb.ControlMessage
	var malformedTopicSpam []pubsub_pb.ControlMessage
	var invalidSporkIDTopicSpam []pubsub_pb.ControlMessage
	var duplicateTopicSpam []pubsub_pb.ControlMessage
	switch msgType {
	case p2pmsg.CtrlMsgGraft:
		unknownTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithGraft(spamRpcCount, unknownTopic.String()))
		malformedTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithGraft(spamRpcCount, malformedTopic.String()))
		invalidSporkIDTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithGraft(spamRpcCount, invalidSporkIDTopic.String()))
		duplicateTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), // sets duplicate to +2 above the threshold to ensure that the victim node will penalize the spammer node
			p2ptest.WithGraft(cfg.NetworkConfig.GossipSub.RpcInspector.Validation.GraftPrune.DuplicateTopicIdThreshold+2, duplicateTopic.String()))
	case p2pmsg.CtrlMsgPrune:
		unknownTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithPrune(spamRpcCount, unknownTopic.String()))
		malformedTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithPrune(spamRpcCount, malformedTopic.String()))
		invalidSporkIDTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithGraft(spamRpcCount, invalidSporkIDTopic.String()))
		duplicateTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), // sets duplicate to +2 above the threshold to ensure that the victim node will penalize the spammer node
			p2ptest.WithPrune(cfg.NetworkConfig.GossipSub.RpcInspector.Validation.GraftPrune.DuplicateTopicIdThreshold+2, duplicateTopic.String()))
	case p2pmsg.CtrlMsgIHave:
		unknownTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithIHave(spamRpcCount, 100, unknownTopic.String()))
		malformedTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithIHave(spamRpcCount, 100, malformedTopic.String()))
		invalidSporkIDTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithIHave(spamRpcCount, 100, invalidSporkIDTopic.String()))
		duplicateTopicSpam = spammer.GenerateCtlMessages(int(spamCtrlMsgCount), // sets duplicate to +2 above the threshold to ensure that the victim node will penalize the spammer node
			p2ptest.WithIHave(cfg.NetworkConfig.GossipSub.RpcInspector.Validation.IHave.DuplicateTopicIdThreshold+spamRpcCount, 100, duplicateTopic.String()))
	default:
		t.Fatal("invalid control message type expected graft or prune")
	}

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, unknownTopicSpam)
	spammer.SpamControlMessage(t, victimNode, malformedTopicSpam)
	spammer.SpamControlMessage(t, victimNode, invalidSporkIDTopicSpam)
	spammer.SpamControlMessage(t, victimNode, duplicateTopicSpam)
	scoreOptParameters := cfg.NetworkConfig.GossipSub.ScoringParameters.PeerScoring.Internal.Thresholds
	// wait for three GossipSub heartbeat intervals to ensure that the victim node has penalized the spammer node.
	require.Eventually(t, func() bool {
		score, ok := victimNode.PeerScoreExposer().GetScore(spammer.SpammerNode.ID())
		return ok && score < 2*scoreOptParameters.Graylist
	}, 5*time.Second, 100*time.Millisecond, "expected victim node to penalize spammer node")

	// now we expect the detection and mitigation to kick in and the victim node to disconnect from the spammer node.
	// so the spammer and victim nodes should not be able to exchange messages on the topic.
	p2ptest.EnsureNoPubsubExchangeBetweenGroups(t,
		ctx,
		[]p2p.LibP2PNode{victimNode},
		flow.IdentifierList{victimId.NodeID},
		[]p2p.LibP2PNode{spammer.SpammerNode},
		flow.IdentifierList{spammer.SpammerId.NodeID},
		blockTopic,
		1,
		func() interface{} {
			return unittest.ProposalFixture()
		})
}
