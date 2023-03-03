package rpc_inspector_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInspect_SafetyThreshold ensures that when RPC control message count is below the configured safety threshold the control message validation inspector
// does not return any errors and validation is skipped.
// NOTE: In the future when application scoring distributor is complete this test will need to be updated to ensure the spammer node is
// also punished for this misbehavior.
func TestInspect_SafetyThreshold(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	// if GRAFT/PRUNE message count is lower than safety threshold the RPC validation should pass
	safetyThreshold := 10
	// create our RPC validation inspector
	inspectorConfig := p2pbuilder.DefaultRPCValidationConfig()
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.GraftValidationCfg.SafetyThreshold = safetyThreshold
	inspectorConfig.PruneValidationCfg.SafetyThreshold = safetyThreshold

	messageCount := 5
	controlMessageCount := int64(2)

	// expected log message logged when valid number GRAFT control messages spammed under safety threshold
	graftExpectedMessageStr := fmt.Sprintf("skipping RPC control message %s inspection validation message count %d below safety threshold", validation.ControlMsgGraft, messageCount)
	// expected log message logged when valid number PRUNE control messages spammed under safety threshold
	pruneExpectedMessageStr := fmt.Sprintf("skipping RPC control message %s inspection validation message count %d below safety threshold", validation.ControlMsgPrune, messageCount)

	graftInfoLogsReceived := atomic.NewInt64(0)
	pruneInfoLogsReceived := atomic.NewInt64(0)
	// setup logger hook, we expect info log validation is skipped
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.InfoLevel {
			if message == graftExpectedMessageStr {
				graftInfoLogsReceived.Inc()
			}

			if message == pruneExpectedMessageStr {
				pruneInfoLogsReceived.Inc()
			}
		}
	})
	logger := zerolog.New(os.Stdout).Hook(hook)

	inspector := validation.NewControlMsgValidationInspector(logger, inspectorConfig)
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
	}, time.Second, 10*time.Millisecond)
}

// TestInspect_UpperThreshold ensures that when RPC control message count is above the configured upper threshold the control message validation inspector
// returns the expected error.
// NOTE: In the future when application scoring distributor is complete this test will need to be updated to ensure the spammer node is
// also punished for this misbehavior.
func TestInspect_UpperThreshold(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	// if GRAFT/PRUNE message count is higher than upper threshold the RPC validation should fail and expected error should be returned
	upperThreshold := 10
	// create our RPC validation inspector
	inspectorConfig := p2pbuilder.DefaultRPCValidationConfig()
	inspectorConfig.NumberOfWorkers = 1
	inspectorConfig.GraftValidationCfg.UpperThreshold = upperThreshold
	inspectorConfig.PruneValidationCfg.UpperThreshold = upperThreshold

	messageCount := 50
	controlMessageCount := int64(1)

	graftValidationErrsReceived := atomic.NewInt64(0)
	pruneValidationErrsReceived := atomic.NewInt64(0)

	inspector := validation.NewControlMsgValidationInspector(unittest.Logger(), inspectorConfig)
	// we use inline inspector here so that we can check the error type when we inspect an RPC and
	// track which control message type the error involves
	inlineInspector := func(id peer.ID, rpc *corrupt.RPC) error {
		pubsubRPC := corruptlibp2p.CorruptRPCToPubSubRPC(rpc)
		err := inspector.Inspect(id, pubsubRPC)
		if err != nil {
			// we should only receive the expected error
			require.Truef(t, validation.IsErrUpperThreshold(err), fmt.Sprintf("expecting to only receive ErrUpperThreshold errors got: %s", err))
			switch {
			case len(rpc.GetControl().GetGraft()) == messageCount:
				graftValidationErrsReceived.Inc()
			case len(rpc.GetControl().GetPrune()) == messageCount:
				pruneValidationErrsReceived.Inc()
			}
			return err
		}
		return nil
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

	// after spamming a single control message for each control message type (GRAFT, PRUNE) we expect
	// to eventually encounters an error for both of these message types because the message count exceeds
	// the configured upper threshold.
	require.Eventually(t, func() bool {
		return graftValidationErrsReceived.Load() == controlMessageCount && pruneValidationErrsReceived.Load() == controlMessageCount
	}, time.Second, 10*time.Millisecond)
}

// TestInspect_RateLimitedPeer ensures that the control message validation inspector rate limits peers per control message type as expected.
// NOTE: In the future when application scoring distributor is complete this test will need to be updated to ensure the spammer node is
// also punished for this misbehavior.
func TestInspect_RateLimitedPeer(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	// create our RPC validation inspector
	inspectorConfig := p2pbuilder.DefaultRPCValidationConfig()
	inspectorConfig.NumberOfWorkers = 2

	messageCount := inspectorConfig.GraftValidationCfg.RateLimit
	controlMessageCount := int64(1)

	graftRateLimitErrsReceived := atomic.NewInt64(0)
	expectedGraftErrStr := fmt.Sprintf("rejecting RPC control messages of type %s are currently rate limited for peer", validation.ControlMsgGraft)
	pruneRateLimitErrsReceived := atomic.NewInt64(0)
	expectedPruneErrStr := fmt.Sprintf("rejecting RPC control messages of type %s are currently rate limited for peer", validation.ControlMsgPrune)
	// setup logger hook, we expect info log validation is skipped
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.ErrorLevel {
			switch {
			case strings.Contains(message, expectedGraftErrStr):
				graftRateLimitErrsReceived.Inc()
			case strings.Contains(message, expectedPruneErrStr):
				pruneRateLimitErrsReceived.Inc()
			}
		}
	})
	logger := zerolog.New(os.Stdout).Hook(hook)

	inspector := validation.NewControlMsgValidationInspector(logger, inspectorConfig)
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
	topic := fmt.Sprintf("%s/%s", channels.PushBlocks.String(), sporkID)
	// prepare to spam - generate control messages
	ctlMsgs := spammer.GenerateCtlMessages(int(controlMessageCount),
		corruptlibp2p.WithGraft(messageCount, topic),
		corruptlibp2p.WithPrune(messageCount, topic))

	// start spamming the victim peer
	// messageCount is equal to the rate limit so when we spam this ctl message 3 times
	// we expected to encounter 2 rate limit errors for each of the control message types GRAFT & PRUNE
	for i := 0; i < 3; i++ {
		spammer.SpamControlMessage(t, victimNode, ctlMsgs)
	}
	
	// eventually we should encounter 2 rate limit errors for each control message type
	require.Eventually(t, func() bool {
		return graftRateLimitErrsReceived.Load() == 2 && pruneRateLimitErrsReceived.Load() == 2
	}, time.Second, 10*time.Millisecond)
}

// TestInspect_InvalidTopicID ensures that when an RPC control message contains an invalid topic ID the expected error is logged.
// NOTE: In the future when application scoring distributor is complete this test will need to be updated to ensure the spammer node is
// also punished for this misbehavior.
func TestInspect_InvalidTopicID(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	// if GRAFT/PRUNE message count is higher than upper threshold the RPC validation should fail and expected error should be returned
	// create our RPC validation inspector
	inspectorConfig := p2pbuilder.DefaultRPCValidationConfig()
	inspectorConfig.PruneValidationCfg.SafetyThreshold = 0
	inspectorConfig.GraftValidationCfg.SafetyThreshold = 0
	inspectorConfig.NumberOfWorkers = 1

	// SafetyThreshold < messageCount < UpperThreshold ensures that the RPC message will be further inspected and topic IDs will be checked
	// restricting the message count to 1 allows us to only aggregate a single error when the error is logged in the inspector.
	messageCount := inspectorConfig.GraftValidationCfg.SafetyThreshold + 1
	controlMessageCount := int64(1)
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", corruptlibp2p.GossipSubTopicIdFixture(), sporkID))
	malformedTopic := channels.Topic("!@#$%^&**((")

	// the errors we expected to encounter for each type of control message GRAFT & PRUNE
	expectedGraftUnknownTopicErr := validation.NewUnknownTopicChannelErr(validation.ControlMsgGraft, unknownTopic)
	expectedGraftMalformedTopicErr := validation.NewMalformedTopicErr(validation.ControlMsgGraft, malformedTopic)
	graftMalformedTopicErrErrsReceived := atomic.NewInt64(0)
	graftUnknownTopicErrErrsReceived := atomic.NewInt64(0)
	expectedPruneMalformedTopicErr := validation.NewMalformedTopicErr(validation.ControlMsgPrune, malformedTopic)
	expectedPruneUnknownTopicErr := validation.NewUnknownTopicChannelErr(validation.ControlMsgPrune, unknownTopic)
	pruneMalformedTopicErrErrsReceived := atomic.NewInt64(0)
	pruneUnknownTopicErrErrsReceived := atomic.NewInt64(0)
	// setup logger hook, we expect info log validation is skipped
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.ErrorLevel {
			switch {
			case strings.Contains(message, expectedGraftUnknownTopicErr.Error()):
				graftUnknownTopicErrErrsReceived.Inc()
			case strings.Contains(message, expectedGraftMalformedTopicErr.Error()):
				graftMalformedTopicErrErrsReceived.Inc()
			case strings.Contains(message, expectedPruneUnknownTopicErr.Error()):
				pruneUnknownTopicErrErrsReceived.Inc()
			case strings.Contains(message, expectedPruneMalformedTopicErr.Error()):
				pruneMalformedTopicErrErrsReceived.Inc()
			}
		}
	})
	logger := zerolog.New(os.Stdout).Hook(hook)
	inspector := validation.NewControlMsgValidationInspector(logger, inspectorConfig)
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
	graftCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(messageCount, unknownTopic.String()))
	graftCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithGraft(messageCount, malformedTopic.String()))
	pruneCtlMsgsWithUnknownTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(messageCount, unknownTopic.String()))
	pruneCtlMsgsWithMalformedTopic := spammer.GenerateCtlMessages(int(controlMessageCount), corruptlibp2p.WithPrune(messageCount, malformedTopic.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, graftCtlMsgsWithMalformedTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithUnknownTopic)
	spammer.SpamControlMessage(t, victimNode, pruneCtlMsgsWithMalformedTopic)

	// there are 2 topic validation error types and we spam a control message for each of the 2 types GRAFT and PRUNE that violate
	// each of the topic validation rules. We expected to encounter each error type once.
	require.Eventually(t, func() bool {
		return graftUnknownTopicErrErrsReceived.Load() == 1 &&
			graftMalformedTopicErrErrsReceived.Load() == 1 &&
			pruneUnknownTopicErrErrsReceived.Load() == 1 &&
			pruneMalformedTopicErrErrsReceived.Load() == 1
	}, time.Second, 10*time.Millisecond)
}

// StartNodesAndEnsureConnected starts the victim and spammer node and ensures they are both connected.
func startNodesAndEnsureConnected(t *testing.T, ctx irrecoverable.SignalerContext, nodes []p2p.LibP2PNode, sporkID flow.Identifier) {
	p2ptest.StartNodes(t, ctx, nodes, 5*time.Second)
	// prior to the test we should ensure that spammer and victim connect.
	// this is vital as the spammer will circumvent the normal pubsub subscription mechanism and send iHAVE messages directly to the victim.
	// without a prior connection established, directly spamming pubsub messages may cause a race condition in the pubsub implementation.
	p2ptest.EnsureConnected(t, ctx, nodes)
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkID)
		return unittest.ProposalFixture(), blockTopic
	})
}

func stopNodesAndInspector(t *testing.T, cancel context.CancelFunc, nodes []p2p.LibP2PNode, inspector *validation.ControlMsgValidationInspector) {
	p2ptest.StopNodes(t, nodes, cancel, 5*time.Second)
	unittest.RequireComponentsDoneBefore(t, time.Second, inspector)
}
