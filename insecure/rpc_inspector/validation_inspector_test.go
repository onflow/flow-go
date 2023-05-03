package rpc_inspector

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/rs/zerolog"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	inspectorbuilder "github.com/onflow/flow-go/network/p2p/p2pbuilder/inspector"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestValidationInspector_SafetyThreshold ensures that when RPC control message count is below the configured safety threshold the control message validation inspector
// does not return any errors and validation is skipped.
func TestValidationInspector_SafetyThreshold(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus

	// if GRAFT/PRUNE message count is lower than safety threshold the RPC validation should pass
	safetyThreshold := uint64(10)
	// create our RPC validation inspector
	inspectorConfig := inspectorbuilder.DefaultRPCValidationConfig()
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.GraftValidationCfg.SafetyThreshold = safetyThreshold
	inspectorConfig.PruneValidationCfg.SafetyThreshold = safetyThreshold

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

	signalerCtx, sporkID, cancelFunc, spammer, victimNode, distributor, validationInspector := setupTest(t, logger, role, inspectorConfig)

	messageCount := 5
	controlMessageCount := int64(2)

	defer distributor.AssertNotCalled(t, "DistributeInvalidControlMessageNotification", mockery.Anything)

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopNodesAndInspector(t, cancelFunc, nodes, validationInspector)
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

// TestValidationInspector_DiscardThreshold ensures that when RPC control message count is above the configured discard threshold an invalid control message
// notification is disseminated with the expected error.
func TestValidationInspector_DiscardThreshold(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	// if GRAFT/PRUNE message count is higher than discard threshold the RPC validation should fail and expected error should be returned
	discardThreshold := uint64(10)
	// create our RPC validation inspector
	inspectorConfig := inspectorbuilder.DefaultRPCValidationConfig()
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.GraftValidationCfg.DiscardThreshold = discardThreshold
	inspectorConfig.PruneValidationCfg.DiscardThreshold = discardThreshold

	messageCount := 50
	controlMessageCount := int64(1)
	count := atomic.NewInt64(0)
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	done := make(chan struct{})
	// ensure expected notifications are disseminated with expected error
	inspectDisseminatedNotif := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvalidControlMessageNotification)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, validation.IsErrDiscardThreshold(notification.Err))
			require.Equal(t, uint64(messageCount), notification.Count)
			switch notification.MsgType {
			case p2p.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2p.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			}
			if count.Load() == 2 {
				close(done)
			}
		}
	}

	signalerCtx, sporkID, cancelFunc, spammer, victimNode, _, validationInspector := setupTest(t, unittest.Logger(), role, inspectorConfig, withExpectedNotificationDissemination(2, inspectDisseminatedNotif))

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopNodesAndInspector(t, cancelFunc, nodes, validationInspector)

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

// TestValidationInspector_RateLimitedPeer ensures that the control message validation inspector rate limits peers per control message type as expected and
// the expected invalid control message notification is disseminated with the expected error.
func TestValidationInspector_RateLimitedPeer(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	// create our RPC validation inspector
	inspectorConfig := inspectorbuilder.DefaultRPCValidationConfig()
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
	inspectDisseminatedNotif := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvalidControlMessageNotification)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, validation.IsErrRateLimitedControlMsg(notification.Err))
			require.Equal(t, uint64(messageCount), notification.Count)
			switch notification.MsgType {
			case p2p.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2p.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			}
			if count.Load() == 4 {
				close(done)
			}
		}
	}

	signalerCtx, sporkID, cancelFunc, spammer, victimNode, _, validationInspector := setupTest(t, unittest.Logger(), role, inspectorConfig, withExpectedNotificationDissemination(4, inspectDisseminatedNotif))

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopNodesAndInspector(t, cancelFunc, nodes, validationInspector)

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
	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	require.Equal(t, uint64(2), invGraftNotifCount.Load())
	require.Equal(t, uint64(2), invPruneNotifCount.Load())
}

// TestValidationInspector_InvalidTopicID ensures that when an RPC control message contains an invalid topic ID an invalid control message
// notification is disseminated with the expected error.
// An invalid topic ID could have any of the following properties:
// - unknown topic: the topic is not a known Flow topic
// - malformed topic: topic is malformed in some way
// - invalid spork ID: spork ID prepended to topic and current spork ID do not match
func TestValidationInspector_InvalidTopicId(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	// if GRAFT/PRUNE message count is higher than discard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	inspectorConfig := inspectorbuilder.DefaultRPCValidationConfig()
	// set safety thresholds to 0 to force inspector to validate all control messages
	inspectorConfig.PruneValidationCfg.SafetyThreshold = 0
	inspectorConfig.GraftValidationCfg.SafetyThreshold = 0
	// set discard threshold to 0 so that in the case of invalid cluster ID
	// we force the inspector to return an error
	inspectorConfig.ClusterPrefixHardThreshold = 0
	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < DiscardThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := inspectorConfig.GraftValidationCfg.SafetyThreshold + 1
	controlMessageCount := int64(1)

	count := atomic.NewUint64(0)
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 6
	// ensure expected notifications are disseminated with expected error
	inspectDisseminatedNotif := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvalidControlMessageNotification)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, channels.IsErrInvalidTopic(notification.Err))
			require.Equal(t, messageCount, notification.Count)
			switch notification.MsgType {
			case p2p.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2p.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			}
			if count.Load() == uint64(expectedNumOfTotalNotif) {
				close(done)
			}
		}
	}

	signalerCtx, sporkID, cancelFunc, spammer, victimNode, _, validationInspector := setupTest(t, unittest.Logger(), role, inspectorConfig, withExpectedNotificationDissemination(expectedNumOfTotalNotif, inspectDisseminatedNotif))

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
	defer stopNodesAndInspector(t, cancelFunc, nodes, validationInspector)

	// prepare to spam - generate control messages
	graftCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), unknownTopic.String()))
	graftCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), malformedTopic.String()))
	graftCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(int(messageCount), invalidSporkIDTopic.String()))

	pruneCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), unknownTopic.String()))
	pruneCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), malformedTopic.String()))
	pruneCtlMsgsInvalidSporkIDTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(int(messageCount), invalidSporkIDTopic.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsInvalidSporkIDTopic)

	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsInvalidSporkIDTopic)

	unittest.RequireCloseBefore(t, done, 5*time.Second, "failed to inspect RPC messages on time")

	// ensure we receive the expected number of invalid control message notifications for graft and prune control message types
	// we send 3 messages with 3 diff invalid topics
	require.Equal(t, uint64(3), invGraftNotifCount.Load())
	require.Equal(t, uint64(3), invPruneNotifCount.Load())
}

// TestValidationInspector_DuplicateTopicId ensures that when an RPC control message contains a duplicate topic ID an invalid control message
// notification is disseminated with the expected error.
func TestValidationInspector_DuplicateTopicId(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	// if GRAFT/PRUNE message count is higher than discard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	inspectorConfig := inspectorbuilder.DefaultRPCValidationConfig()
	// set safety thresholds to 0 to force inspector to validate all control messages
	inspectorConfig.PruneValidationCfg.SafetyThreshold = 0
	inspectorConfig.GraftValidationCfg.SafetyThreshold = 0
	// set discard threshold to 0 so that in the case of invalid cluster ID
	// we force the inspector to return an error
	inspectorConfig.ClusterPrefixHardThreshold = 0
	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < DiscardThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := inspectorConfig.GraftValidationCfg.SafetyThreshold + 3
	controlMessageCount := int64(1)

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 2
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	inspectDisseminatedNotif := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvalidControlMessageNotification)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, validation.IsErrDuplicateTopic(notification.Err))
			require.Equal(t, messageCount, notification.Count)
			switch notification.MsgType {
			case p2p.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2p.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			}
			if count.Load() == int64(expectedNumOfTotalNotif) {
				close(done)
			}
		}
	}

	signalerCtx, sporkID, cancelFunc, spammer, victimNode, _, validationInspector := setupTest(t, unittest.Logger(), role, inspectorConfig, withExpectedNotificationDissemination(expectedNumOfTotalNotif, inspectDisseminatedNotif))

	// a topics spork ID is considered invalid if it does not match the current spork ID
	duplicateTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID))

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopNodesAndInspector(t, cancelFunc, nodes, validationInspector)

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

// TestValidationInspector_UnknownClusterId ensures that when an RPC control message contains a topic with an unknown cluster ID an invalid control message
// notification is disseminated with the expected error.
func TestValidationInspector_UnknownClusterId(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	// if GRAFT/PRUNE message count is higher than discard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	inspectorConfig := inspectorbuilder.DefaultRPCValidationConfig()
	// set safety thresholds to 0 to force inspector to validate all control messages
	inspectorConfig.PruneValidationCfg.SafetyThreshold = 0
	inspectorConfig.GraftValidationCfg.SafetyThreshold = 0
	// set discard threshold to 0 so that in the case of invalid cluster ID
	// we force the inspector to return an error
	inspectorConfig.ClusterPrefixHardThreshold = 0
	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < DiscardThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := inspectorConfig.GraftValidationCfg.SafetyThreshold + 1
	controlMessageCount := int64(1)

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 2
	invGraftNotifCount := atomic.NewUint64(0)
	invPruneNotifCount := atomic.NewUint64(0)
	inspectDisseminatedNotif := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvalidControlMessageNotification)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			require.True(t, channels.IsErrUnknownClusterID(notification.Err))
			require.Equal(t, messageCount, notification.Count)
			switch notification.MsgType {
			case p2p.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			case p2p.CtrlMsgPrune:
				invPruneNotifCount.Inc()
			}
			if count.Load() == int64(expectedNumOfTotalNotif) {
				close(done)
			}
		}
	}

	signalerCtx, sporkID, cancelFunc, spammer, victimNode, _, validationInspector := setupTest(t, unittest.Logger(), role, inspectorConfig, withExpectedNotificationDissemination(expectedNumOfTotalNotif, inspectDisseminatedNotif))

	// setup cluster prefixed topic with an invalid cluster ID
	unknownClusterID := channels.Topic(channels.SyncCluster("unknown-cluster-ID"))
	// consume cluster ID update so that active cluster IDs set
	validationInspector.OnClusterIDSUpdate(p2p.ClusterIDUpdate{"known-cluster-id"})

	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopNodesAndInspector(t, cancelFunc, nodes, validationInspector)

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

// TestValidationInspector_ActiveClusterIdsNotSet ensures that an error is returned only after the cluster prefixed topics received for a peer exceed the configured
// cluster prefix discard threshold when the active cluster IDs not set and an invalid control message notification is disseminated with the expected error.
func TestValidationInspector_ActiveClusterIdsNotSet(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	// if GRAFT/PRUNE message count is higher than discard threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	inspectorConfig := inspectorbuilder.DefaultRPCValidationConfig()
	inspectorConfig.GraftValidationCfg.SafetyThreshold = 0
	inspectorConfig.ClusterPrefixHardThreshold = 5
	inspectorConfig.NumberOfWorkers = 1
	controlMessageCount := int64(10)

	count := atomic.NewInt64(0)
	done := make(chan struct{})
	expectedNumOfTotalNotif := 5
	invGraftNotifCount := atomic.NewUint64(0)
	inspectDisseminatedNotif := func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments) {
		return func(args mockery.Arguments) {
			count.Inc()
			notification, ok := args[0].(*p2p.InvalidControlMessageNotification)
			require.True(t, ok)
			require.True(t, validation.IsErrActiveClusterIDsNotSet(notification.Err))
			require.Equal(t, spammer.SpammerNode.Host().ID(), notification.PeerID)
			switch notification.MsgType {
			case p2p.CtrlMsgGraft:
				invGraftNotifCount.Inc()
			}
			require.Equal(t, uint64(1), notification.Count)
			if count.Load() == int64(expectedNumOfTotalNotif) {
				close(done)
			}
		}
	}

	signalerCtx, sporkID, cancelFunc, spammer, victimNode, _, validationInspector := setupTest(t, unittest.Logger(), role, inspectorConfig, withExpectedNotificationDissemination(expectedNumOfTotalNotif, inspectDisseminatedNotif))

	// we deliberately avoid setting the cluster IDs so that we eventually receive errors after we have exceeded the allowed cluster
	// prefixed discard threshold
	validationInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopNodesAndInspector(t, cancelFunc, nodes, validationInspector)
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

func randomClusterPrefixedTopic() channels.Topic {
	return channels.Topic(channels.SyncCluster(flow.ChainID(fmt.Sprintf("%d", rand.Uint64()))))
}

type onNotificationDissemination func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments)
type mockDistributorOption func(*mockp2p.GossipSubInspectorNotificationDistributor, *corruptlibp2p.GossipSubRouterSpammer)

func withExpectedNotificationDissemination(expectedNumOfTotalNotif int, f onNotificationDissemination) mockDistributorOption {
	return func(distributor *mockp2p.GossipSubInspectorNotificationDistributor, spammer *corruptlibp2p.GossipSubRouterSpammer) {
		distributor.
			On("DistributeInvalidControlMessageNotification", mockery.Anything).
			Times(expectedNumOfTotalNotif).
			Run(f(spammer)).
			Return(nil)
	}
}

// setupTest sets up common components of RPC inspector test.
func setupTest(t *testing.T, logger zerolog.Logger, role flow.Role, inspectorConfig *validation.ControlMsgValidationInspectorConfig, mockDistributorOpts ...mockDistributorOption) (
	*irrecoverable.MockSignalerContext,
	flow.Identifier,
	context.CancelFunc,
	*corruptlibp2p.GossipSubRouterSpammer,
	p2p.LibP2PNode,
	*mockp2p.GossipSubInspectorNotificationDistributor,
	*validation.ControlMsgValidationInspector,
) {
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	for _, mockDistributorOpt := range mockDistributorOpts {
		mockDistributorOpt(distributor, spammer)
	}

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

	return signalerCtx, sporkID, cancel, spammer, victimNode, distributor, inspector
}
