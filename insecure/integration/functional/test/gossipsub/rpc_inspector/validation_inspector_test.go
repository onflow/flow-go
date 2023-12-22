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
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/scoring"
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
	inspectorConfig.NumberOfWorkers = 1
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

	// create unknown topic
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", p2ptest.GossipSubTopicIdFixture(), sporkID))
	// create malformed topic
	malformedTopic := channels.Topic(unittest.RandomStringFixture(t, 100))
	// a topics spork ID is considered invalid if it does not match the current spork ID
	invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture()))
	allTopics := []string{unknownTopic.String(), malformedTopic.String(), invalidSporkIDTopic.String()}
	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)
	distributor.
		On("Distribute", mockery.Anything).
		Times(expectedNumOfTotalNotif).
		Run(func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Contains(t, allTopics, notification.Errors[0].Topic())
			require.Equal(t, notification.Errors[0].CtrlMsgTopicType(), p2p.CtrlMsgNonClusterTopicType, "IsClusterPrefixed is expected to be false, no RPC with cluster prefixed topic sent in this test")
			require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
			require.True(t, channels.IsInvalidTopicErr(notification.Errors[0].Err))
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2pmsg.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			case p2pmsg.CtrlMsgIHave:
				invIHaveNotifCount.Inc()
			default:
				require.Fail(t, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Errors[0].Err))
			}
			if count.Load() == uint64(expectedNumOfTotalNotif) {
				close(done)
			}
		}).
		Return(nil)

	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := newMockUpdatableTopicProvider()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		Distributor:             distributor,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
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

	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topicProvider.UpdateTopics(allTopics)

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

	inspectorConfig.NumberOfWorkers = 1

	messageCount := 10
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

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)
	// a topics spork ID is considered invalid if it does not match the current spork ID
	duplicateTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID))
	distributor.
		On("Distribute", mockery.Anything).
		Times(expectedNumOfTotalNotif).
		Run(func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, duplicateTopic.String(), notification.Errors[0].Topic())
			require.Equal(t, notification.Errors[0].CtrlMsgTopicType(), p2p.CtrlMsgNonClusterTopicType, "IsClusterPrefixed is expected to be false, no RPC with cluster prefixed topic sent in this test")
			require.True(t, validation.IsDuplicateTopicErr(notification.Errors[0].Err))
			require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2pmsg.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			case p2pmsg.CtrlMsgIHave:
				invIHaveNotifCount.Inc()
			default:
				require.Fail(t, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Errors[0].Err))
			}

			if count.Load() == int64(expectedNumOfTotalNotif) {
				close(done)
			}
		}).
		Return(nil)

	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := newMockUpdatableTopicProvider()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		Distributor:             distributor,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
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

	inspectorConfig.NumberOfWorkers = 1

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 1
	invIHaveNotifCount := atomic.NewUint64(0)

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)

	meshTracer := meshTracerFixture(flowConfig, idProvider)

	topicProvider := newMockUpdatableTopicProvider()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		Distributor:             distributor,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
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

	// generate 2 control messages with iHaves for different topics
	ihaveCtlMsgs1 := spammer.GenerateCtlMessages(1, p2ptest.WithIHave(1, 1, pushBlocks.String()))
	ihaveCtlMsgs2 := spammer.GenerateCtlMessages(1, p2ptest.WithIHave(1, 1, reqChunks.String()))

	// duplicate message ids for a single topic is invalid and will cause an error
	ihaveCtlMsgs1[0].Ihave[0].MessageIDs = append(ihaveCtlMsgs1[0].Ihave[0].MessageIDs, ihaveCtlMsgs1[0].Ihave[0].MessageIDs[0])
	// duplicate message ids across different topics is valid
	ihaveCtlMsgs2[0].Ihave[0].MessageIDs[0] = ihaveCtlMsgs1[0].Ihave[0].MessageIDs[0]

	distributor.
		On("Distribute", mockery.Anything).
		Times(expectedNumOfTotalNotif).
		Run(func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Contains(t, ihaveCtlMsgs1[0].Ihave[0].MessageIDs, notification.Errors[0].MessageID())
			require.Equal(t, notification.Errors[0].CtrlMsgTopicType(), p2p.CtrlMsgNonClusterTopicType, "IsClusterPrefixed is expected to be false, no RPC with cluster prefixed topic sent in this test")
			require.True(t, validation.IsDuplicateMessageIDErr(notification.Errors[0].Err))
			require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
			require.True(t,
				notification.MsgType == p2pmsg.CtrlMsgIHave,
				fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Errors[0].Err))
			invIHaveNotifCount.Inc()

			if count.Load() == int64(expectedNumOfTotalNotif) {
				close(done)
			}
		}).
		Return(nil)

	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, validationInspector)

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ihaveCtlMsgs1)
	spammer.SpamControlMessage(t, victimNode, ihaveCtlMsgs2)

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
	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < HardThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := 1
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

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)
	// setup cluster prefixed topic with an invalid cluster ID
	unknownClusterID := channels.Topic(channels.SyncCluster("unknown-cluster-ID"))
	distributor.
		On("Distribute", mockery.Anything).
		Times(expectedNumOfTotalNotif).
		Run(func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, unknownClusterID.String(), notification.Errors[0].Topic())
			require.Equal(t, notification.Errors[0].CtrlMsgTopicType(), p2p.CtrlMsgTopicTypeClusterPrefixed)
			require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
			require.True(t, channels.IsUnknownClusterIDErr(notification.Errors[0].Err))
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2pmsg.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			default:
				require.Fail(t, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Errors[0].Err))
			}

			if count.Load() == int64(expectedNumOfTotalNotif) {
				close(done)
			}
		}).
		Return(nil)

	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := newMockUpdatableTopicProvider()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		Distributor:             distributor,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
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
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true).Times(4)

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
	inspectorConfig.ClusterPrefixedMessage.HardThreshold = 5
	inspectorConfig.NumberOfWorkers = 1
	controlMessageCount := int64(6)

	count := atomic.NewInt64(0)
	invGraftNotifCount := atomic.NewInt64(0)
	logHookDone := make(chan struct{})
	notificationDone := make(chan struct{})
	expectedNumOfLogs := 5
	expectedNumOfTotalNotif := 1
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			if message == "failed to validate cluster prefixed control message with cluster prefixed topic active cluster ids not set" {
				count.Inc()
			}

			if count.Load() == int64(expectedNumOfLogs) {
				close(logHookDone)
			}
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.WarnLevel).Hook(hook)

	inspectorIdProvider := mock.NewIdentityProvider(t)
	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)
	distributor.
		On("Distribute", mockery.Anything).
		Times(expectedNumOfTotalNotif).
		Run(func(args mockery.Arguments) {
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
			require.True(t, validation.IsErrActiveClusterIDsNotSet(notification.Errors[0].Err))
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			default:
				require.Fail(t, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Errors[0].Err))
			}

			if invGraftNotifCount.Load() == int64(expectedNumOfTotalNotif) {
				close(notificationDone)
			}
		}).
		Return(nil)

	meshTracer := meshTracerFixture(flowConfig, idProvider)

	topicProvider := newMockUpdatableTopicProvider()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  logger,
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		Distributor:             distributor,
		IdProvider:              inspectorIdProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
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
	// we expect controlMessageCount plus 1 extra call, this is due to messages that are exchanged when the nodes startup
	inspectorIdProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true).Times(int(controlMessageCount + 1))

	clusterPrefixedTopic := randomClusterPrefixedTopic()

	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topicProvider.UpdateTopics([]string{clusterPrefixedTopic.String()})

	// we deliberately avoid setting the cluster IDs so that we eventually receive errors after we have exceeded the allowed cluster
	// prefixed hard threshold
	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, validationInspector)
	// generate multiple control messages with GRAFT's for randomly generated
	// cluster prefixed channels, this ensures we do not encounter duplicate topic ID errors
	ctlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithGraft(1, clusterPrefixedTopic.String()))
	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ctlMsgs)

	unittest.RequireCloseBefore(t, logHookDone, 2*time.Second, "failed to observe warning logs on time")
	unittest.RequireCloseBefore(t, notificationDone, 2*time.Second, "failed to observe invalid control message notification on time")
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
	inspectorConfig.ClusterPrefixedMessage.HardThreshold = 5
	inspectorConfig.NumberOfWorkers = 1
	controlMessageCount := int64(6)

	count := atomic.NewInt64(0)
	invPruneNotifCount := atomic.NewInt64(0)
	logHookDone := make(chan struct{})
	notificationDone := make(chan struct{})
	expectedNumOfLogs := 5
	expectedNumOfTotalNotif := 1
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			if message == "failed to validate cluster prefixed control message with cluster prefixed topic active cluster ids not set" {
				count.Inc()
			}

			if count.Load() == int64(expectedNumOfLogs) {
				close(logHookDone)
			}
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.WarnLevel).Hook(hook)

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)
	distributor.
		On("Distribute", mockery.Anything).
		Times(expectedNumOfTotalNotif).
		Run(func(args mockery.Arguments) {
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
			require.True(t, validation.IsErrActiveClusterIDsNotSet(notification.Errors[0].Err))
			switch notification.MsgType {
			case p2pmsg.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			default:
				require.Fail(t, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Errors[0].Err))
			}

			if invPruneNotifCount.Load() == int64(expectedNumOfTotalNotif) {
				close(notificationDone)
			}
		}).
		Return(nil)

	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := newMockUpdatableTopicProvider()
	inspectorIdProvider := mock.NewIdentityProvider(t)
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  logger,
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		Distributor:             distributor,
		IdProvider:              inspectorIdProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
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
	// we expect controlMessageCount plus 1 extra call, this is due to messages that are exchanged when the nodes startup
	inspectorIdProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true).Times(int(controlMessageCount + 1))

	clusterPrefixedTopic := randomClusterPrefixedTopic()
	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topicProvider.UpdateTopics([]string{clusterPrefixedTopic.String()})

	// we deliberately avoid setting the cluster IDs so that we eventually receive errors after we have exceeded the allowed cluster
	// prefixed hard threshold
	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, validationInspector)
	// generate multiple control messages with GRAFT's for randomly generated
	// cluster prefixed channels, this ensures we do not encounter duplicate topic ID errors
	ctlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithPrune(1, clusterPrefixedTopic.String()))
	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ctlMsgs)

	unittest.RequireCloseBefore(t, logHookDone, 2*time.Second, "failed to observe warning logs on time")
	unittest.RequireCloseBefore(t, notificationDone, 2*time.Second, "failed to observe invalid control message notification on time")
}

// TestValidationInspector_Unstaked_Node_Detection ensures that RPC control message inspector disseminates an invalid control message notification when an unstaked peer
// sends a control message for a cluster prefixed topic.
func TestValidationInspector_UnstakedNode_Detection(t *testing.T) {
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation
	// set hard threshold to 0 so that in the case of invalid cluster ID
	// we force the inspector to return an error
	inspectorConfig.ClusterPrefixedMessage.HardThreshold = 0
	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < HardThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := 10
	controlMessageCount := int64(1)

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfLogs := 2
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			if message == "control message received from unstaked peer" {
				count.Inc()
			}
		}
		if count.Load() == int64(expectedNumOfLogs) {
			close(done)
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.WarnLevel).Hook(hook)

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)
	meshTracer := meshTracerFixture(flowConfig, idProvider)

	topicProvider := newMockUpdatableTopicProvider()
	inspectorIdProvider := mock.NewIdentityProvider(t)
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  logger,
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		Distributor:             distributor,
		IdProvider:              inspectorIdProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
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
	// we expect 2 calls from notification inspection plus 1 extra call, this is due to messages that are exchanged when the nodes startup
	inspectorIdProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(nil, false).Times(3)

	// setup cluster prefixed topic with an invalid cluster ID
	clusterID := flow.ChainID("known-cluster-id")
	clusterIDTopic := channels.Topic(channels.SyncCluster(clusterID))
	// consume cluster ID update so that active cluster IDs set
	validationInspector.ActiveClustersChanged(flow.ChainIDList{clusterID})

	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topicProvider.UpdateTopics([]string{clusterIDTopic.String()})

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, validationInspector)

	// prepare to spam - generate control messages
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithGraft(messageCount, clusterIDTopic.String()))
	pruneCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithPrune(messageCount, clusterIDTopic.String()))

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
	// force all cache miss checks
	inspectorConfig.IWant.CacheMissCheckSize = 1
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.IWant.CacheMissThreshold = .5 // set cache miss threshold to 50%
	messageCount := 1
	controlMessageCount := int64(1)
	cacheMissThresholdNotifCount := atomic.NewUint64(0)
	done := make(chan struct{})
	// ensure expected notifications are disseminated with expected error
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, notification.Errors[0].CtrlMsgTopicType(), p2p.CtrlMsgNonClusterTopicType, "IsClusterPrefixed is expected to be false, no RPC with cluster prefixed topic sent in this test")
			require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
			require.True(t, notification.MsgType == p2pmsg.CtrlMsgIWant, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Errors[0].Err))
			require.True(t, validation.IsIWantCacheMissThresholdErr(notification.Errors[0].Err))
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
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(1, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)

	topicProvider := newMockUpdatableTopicProvider()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		Distributor:             distributor,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
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
	inspectorConfig.NumberOfWorkers = 1

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
	ejectedIdentity.Ejected = true

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
	// ensure expected notifications are disseminated with expected error
	inspectDisseminatedNotifyFunc := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, notification.Errors[0].CtrlMsgTopicType(), p2p.CtrlMsgNonClusterTopicType, "IsClusterPrefixed is expected to be false, no RPC with cluster prefixed topic sent in this test")
			require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
			require.Equal(t, p2pmsg.RpcPublishMessage, notification.MsgType, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Errors[0].Err))
			require.True(t, validation.IsInvalidRpcPublishMessagesErr(notification.Errors[0].Err))
			require.Contains(t,
				notification.Errors[0].Err.Error(),
				fmt.Sprintf("%d error(s) encountered", len(invalidPublishMsgs)),
				fmt.Sprintf("expected %d errors, an error for each invalid pubsub message", len(invalidPublishMsgs)))
			require.Contains(t, notification.Errors[0].Err.Error(), fmt.Sprintf("received rpc publish message from unstaked peer: %s", unknownPeerID))
			require.Contains(t, notification.Errors[0].Err.Error(), fmt.Sprintf("received rpc publish message from ejected peer: %s", ejectedIdentityPeerID))
			notificationCount.Inc()
			if notificationCount.Load() == 1 {
				close(done)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)
	withExpectedNotificationDissemination(1, inspectDisseminatedNotifyFunc)(distributor, spammer)
	meshTracer := meshTracerFixture(flowConfig, idProvider)
	topicProvider := newMockUpdatableTopicProvider()
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		Distributor:             distributor,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
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
	inspectorConfig.MessageErrorThreshold = 6

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

	// return nil for unknown peer ID indicating unstaked peer
	idProvider.On("ByPeerID", unknownPeerID).Return(nil, false).Once()
	// return ejected identity for peer ID will force message validation failure
	idProvider.On("ByPeerID", ejectedIdentityPeerID).Return(ejectedIdentity, true).Once()

	// set topic oracle to return list with all topics to avoid hasSubscription failures and force topic validation
	topicProvider.UpdateTopics([]string{topic.String(), unknownTopic, malformedTopic, invalidSporkIDTopic})

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

// TestValidationInspector_MultiErrorNotification ensures that control message validation captures all errors for invalid control messages for a single control message type. RPC validation is performed
// on each control message type, during this validation a sample of the control messages for that type are inspected. This test ensures that the entire sample is inspected and all errors are captured in
// the p2p.InvCtrlMsgErrs field of the invalid control message notification.
func TestValidationInspector_MultiErrorNotification(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	inspectorConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs

	inspectorConfig.NumberOfWorkers = 1
	controlMessageCount := int64(1)
	invGraftCount := atomic.NewUint64(0)
	invPruneCount := atomic.NewUint64(0)
	invIHaveCount := atomic.NewUint64(0)
	invIWantCount := atomic.NewUint64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 4

	requireExpectedNotifErr := func(notification *p2p.InvCtrlMsgNotif) {
		for _, invErr := range notification.Errors {
			require.True(t, channels.IsInvalidTopicErr(invErr.Err))
		}
	}

	idProvider := mock.NewIdentityProvider(t)
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)
	topicProvider := newMockUpdatableTopicProvider()
	distributor.
		On("Distribute", mockery.Anything).
		Times(expectedNumOfTotalNotif).
		Run(func(args mockery.Arguments) {
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.ID(), notification.PeerID)
			switch notification.MsgType {
			case p2pmsg.CtrlMsgGraft:
				requireExpectedNotifErr(notification)
				require.Len(t, notification.Errors, 3)
				invGraftCount.Inc()
			case p2pmsg.CtrlMsgPrune:
				requireExpectedNotifErr(notification)
				require.Len(t, notification.Errors, 3)
				invPruneCount.Inc()
			case p2pmsg.CtrlMsgIHave:
				requireExpectedNotifErr(notification)
				require.Len(t, notification.Errors, 3)
				invIHaveCount.Inc()
			case p2pmsg.CtrlMsgIWant:
				require.Len(t, notification.Errors, 1)
				require.True(t, validation.IsIWantDuplicateMsgIDThresholdErr(notification.Errors[0].Err))
				invIWantCount.Inc()
			default:
				require.Fail(t, fmt.Sprintf("unexpected control message type %s error: %s", notification.MsgType, notification.Errors[0].Err))
			}

			if invGraftCount.Load()+invPruneCount.Load()+invIHaveCount.Load()+invIWantCount.Load() == uint64(expectedNumOfTotalNotif) {
				close(done)
			}
		}).
		Return(nil)

	meshTracer := meshTracerFixture(flowConfig, idProvider)

	inspectorMetrics := mock.NewGossipSubRpcValidationInspectorMetrics(t)
	inspectorMetrics.On("AsyncProcessingStarted")
	inspectorMetrics.On("AsyncProcessingFinished", mockery.AnythingOfType("time.Duration"))
	inspectorMetrics.On("InvalidControlMessageNotificationErrors", mockery.AnythingOfType("int")).Run(func(args mockery.Arguments) {
		count, ok := args.Get(0).(int)
		require.True(t, ok)
		// count is expected to be 3 for graft, prune and ihave messages that have been configured with 3 invalid topics.
		// 1 for iwant control message with a single duplicate message ID error.
		require.True(t, count == 3 || count == 1)
	})
	validationInspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &inspectorConfig,
		Distributor:             distributor,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        inspectorMetrics,
		RpcTracker:              meshTracer,
		NetworkingType:          network.PrivateNetwork,
		TopicOracle: func() p2p.TopicProvider {
			return topicProvider
		},
	})
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
	idProvider.On("ByPeerID", victimNode.ID()).Return(&victimIdentity, true).Maybe()
	idProvider.On("ByPeerID", spammer.SpammerNode.ID()).Return(&spammer.SpammerId, true).Maybe()

	// create unknown topic
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", p2ptest.GossipSubTopicIdFixture(), sporkID))
	// create malformed topic
	malformedTopic := channels.Topic(channels.TestNetworkChannel)
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
	grafts := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithGrafts(unknownTopic.String(), malformedTopic.String(), invalidSporkIDTopic.String()))
	prunes := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithPrunes(unknownTopic.String(), malformedTopic.String(), invalidSporkIDTopic.String()))
	ihaves := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithIHaves(100, unknownTopic.String(), malformedTopic.String(), invalidSporkIDTopic.String()))
	iwants := spammer.GenerateCtlMessages(int(controlMessageCount), p2ptest.WithIWantMessageIds("duplicate_message_id", "duplicate_message_id"))

	// spam the victim peer with invalid graft messages
	spammer.SpamControlMessage(t, victimNode, grafts)
	// spam the victim peer with invalid prune messages
	spammer.SpamControlMessage(t, victimNode, prunes)
	// spam the victim peer with invalid ihaves messages
	spammer.SpamControlMessage(t, victimNode, ihaves)
	// spam the victim peer with invalid iwants messages
	spammer.SpamControlMessage(t, victimNode, iwants)
	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")

	require.Equal(t, uint64(1), invGraftCount.Load())
	require.Equal(t, uint64(1), invPruneCount.Load())
	require.Equal(t, uint64(1), invIWantCount.Load())
	require.Equal(t, uint64(1), invIHaveCount.Load())
}

// TestGossipSubSpamMitigationIntegration tests that the spam mitigation feature of GossipSub is working as expected.
// The test puts toghether the spam detection (through the GossipSubInspector) and the spam mitigation (through the
// scoring system) and ensures that the mitigation is triggered when the spam detection detects spam.
// The test scenario involves a spammer node that sends a large number of control messages to a victim node.
// The victim node is configured to use the GossipSubInspector to detect spam and the scoring system to mitigate spam.
// The test ensures that the victim node is disconnected from the spammer node on the GossipSub mesh after the spam detection is triggered.
func TestGossipSubSpamMitigationIntegration(t *testing.T) {
	idProvider := mock.NewIdentityProvider(t)
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, flow.RoleConsensus, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// set the scoring parameters to be more aggressive to speed up the test
	cfg.NetworkConfig.GossipSub.RpcTracer.ScoreTracerInterval = 100 * time.Millisecond
	cfg.NetworkConfig.GossipSub.ScoringParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond
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

	spamRpcCount := 1000           // total number of individual rpc messages to send
	spamCtrlMsgCount := int64(500) // total number of control messages to send on each RPC

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

	// prepares spam graft and prune messages with different strategies.
	graftCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithGraft(spamRpcCount, unknownTopic.String()))
	graftCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithGraft(spamRpcCount, malformedTopic.String()))
	graftCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithGraft(spamRpcCount, invalidSporkIDTopic.String()))
	graftCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithGraft(3, duplicateTopic.String()))

	pruneCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithPrune(spamRpcCount, unknownTopic.String()))
	pruneCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithPrune(spamRpcCount, malformedTopic.String()))
	pruneCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithGraft(spamRpcCount, invalidSporkIDTopic.String()))
	pruneCtlMsgsDuplicateTopic := spammer.GenerateCtlMessages(int(spamCtrlMsgCount), p2ptest.WithPrune(3, duplicateTopic.String()))

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
	require.Eventually(t, func() bool {
		score, ok := victimNode.PeerScoreExposer().GetScore(spammer.SpammerNode.ID())
		return ok && score < 2*scoring.DefaultGraylistThreshold
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

// mockUpdatableTopicProvider is a mock implementation of the TopicProvider interface.
// TODO: there is a duplicate implementation of this in the test package, we should consolidate them.
// The duplicate exists in network/p2p/inspector/internal/mockTopicProvider.go. The reason for duplication is that
// the inspector/validation package does not have a separate test package. Hence, sharing the mock implementation
// will cause a cyclic dependency.
type mockUpdatableTopicProvider struct {
	topics        []string
	subscriptions map[string][]peer.ID
}

func newMockUpdatableTopicProvider() *mockUpdatableTopicProvider {
	return &mockUpdatableTopicProvider{
		topics:        []string{},
		subscriptions: map[string][]peer.ID{},
	}
}

func (m *mockUpdatableTopicProvider) GetTopics() []string {
	return m.topics
}

func (m *mockUpdatableTopicProvider) ListPeers(topic string) []peer.ID {
	return m.subscriptions[topic]
}

func (m *mockUpdatableTopicProvider) UpdateTopics(topics []string) {
	m.topics = topics
}

func (m *mockUpdatableTopicProvider) UpdateSubscriptions(topic string, peers []peer.ID) {
	m.subscriptions[topic] = peers
}
