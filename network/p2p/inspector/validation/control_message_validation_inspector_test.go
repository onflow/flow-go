package validation_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestNewControlMsgValidationInspector(t *testing.T) {
	t.Run("should create validation inspector without error", func(t *testing.T) {
		sporkID := unittest.IdentifierFixture()
		flowConfig, err := config.DefaultConfig()
		require.NoError(t, err, "failed to get default flow config")
		consumer := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
		idProvider := mockmodule.NewIdentityProvider(t)
		topicProvider := p2ptest.NewUpdatableTopicProviderFixture()
		inspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
			Logger:                  unittest.Logger(),
			SporkID:                 sporkID,
			Config:                  &flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation,
			IdProvider:              idProvider,
			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
			InspectorMetrics:        metrics.NewNoopCollector(),
			RpcTracker:              mockp2p.NewRpcControlTracking(t),
			NetworkingType:          network.PublicNetwork,
			InvalidControlMessageNotificationConsumer: consumer,
			TopicOracle: func() p2p.TopicProvider {
				return topicProvider
			},
		})
		require.NoError(t, err)
		require.NotNil(t, inspector)
	})
	t.Run("should return error if any of the params are nil", func(t *testing.T) {
		inspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
			Logger:                  unittest.Logger(),
			SporkID:                 unittest.IdentifierFixture(),
			Config:                  nil,
			IdProvider:              nil,
			HeroCacheMetricsFactory: nil,
			InspectorMetrics:        nil,
			RpcTracker:              nil,
			TopicOracle:             nil,
			InvalidControlMessageNotificationConsumer: nil,
		})
		require.Nil(t, inspector)
		require.Error(t, err)
		s := err.Error()
		require.Contains(t, s, "validation for 'Config' failed on the 'required'")
		require.Contains(t, s, "validation for 'InvalidControlMessageNotificationConsumer' failed on the 'required'")
		require.Contains(t, s, "validation for 'IdProvider' failed on the 'required'")
		require.Contains(t, s, "validation for 'HeroCacheMetricsFactory' failed on the 'required'")
		require.Contains(t, s, "validation for 'InspectorMetrics' failed on the 'required'")
		require.Contains(t, s, "validation for 'RpcTracker' failed on the 'required'")
		require.Contains(t, s, "validation for 'NetworkingType' failed on the 'required'")
		require.Contains(t, s, "validation for 'TopicOracle' failed on the 'required'")
	})
}

// TestControlMessageValidationInspector_TruncateRPC verifies the expected truncation behavior of RPC control messages.
// Message truncation for each control message type occurs when the count of control
// messages exceeds the configured maximum sample size for that control message type.
func TestControlMessageValidationInspector_truncateRPC(t *testing.T) {
	t.Run("graft truncation", func(t *testing.T) {
		graftPruneMessageMaxSampleSize := 1000
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.GraftPrune.MessageCountThreshold = graftPruneMessageMaxSampleSize
		})
		// topic validation is ignored set any topic oracle
		consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		// topic validation not performed so we can use random strings
		graftsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixtures(unittest.IdentifierListFixture(2000).Strings()...)...))
		require.Greater(t, len(graftsGreaterThanMaxSampleSize.GetControl().GetGraft()), graftPruneMessageMaxSampleSize)
		graftsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixtures(unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(graftsLessThanMaxSampleSize.GetControl().GetGraft()), graftPruneMessageMaxSampleSize)

		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Twice()
		require.NoError(t, inspector.Inspect(from, graftsGreaterThanMaxSampleSize))
		require.NoError(t, inspector.Inspect(from, graftsLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			// rpc with grafts greater than configured max sample size should be truncated to GraftPruneMessageMaxSampleSize
			shouldBeTruncated := len(graftsGreaterThanMaxSampleSize.GetControl().GetGraft()) == graftPruneMessageMaxSampleSize
			// rpc with grafts less than GraftPruneMessageMaxSampleSize should not be truncated
			shouldNotBeTruncated := len(graftsLessThanMaxSampleSize.GetControl().GetGraft()) == 50
			return shouldBeTruncated && shouldNotBeTruncated
		}, time.Second, 500*time.Millisecond)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("prune truncation", func(t *testing.T) {
		graftPruneMessageMaxSampleSize := 1000
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.GraftPrune.MessageCountThreshold = graftPruneMessageMaxSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()

		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		// topic validation not performed, so we can use random strings
		prunesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(2000).Strings()...)...))
		require.Greater(t, len(prunesGreaterThanMaxSampleSize.GetControl().GetPrune()), graftPruneMessageMaxSampleSize)
		prunesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(prunesLessThanMaxSampleSize.GetControl().GetPrune()), graftPruneMessageMaxSampleSize)
		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Twice()
		require.NoError(t, inspector.Inspect(from, prunesGreaterThanMaxSampleSize))
		require.NoError(t, inspector.Inspect(from, prunesLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			// rpc with prunes greater than configured max sample size should be truncated to GraftPruneMessageMaxSampleSize
			shouldBeTruncated := len(prunesGreaterThanMaxSampleSize.GetControl().GetPrune()) == graftPruneMessageMaxSampleSize
			// rpc with prunes less than GraftPruneMessageMaxSampleSize should not be truncated
			shouldNotBeTruncated := len(prunesLessThanMaxSampleSize.GetControl().GetPrune()) == 50
			return shouldBeTruncated && shouldNotBeTruncated
		}, time.Second, 500*time.Millisecond)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("ihave message id truncation", func(t *testing.T) {
		maxSampleSize := 1000
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IHave.MessageCountThreshold = maxSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()

		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		// topic validation not performed so we can use random strings
		iHavesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(2000, unittest.IdentifierListFixture(2000).Strings()...)...))
		require.Greater(t, len(iHavesGreaterThanMaxSampleSize.GetControl().GetIhave()), maxSampleSize)
		iHavesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(200, unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(iHavesLessThanMaxSampleSize.GetControl().GetIhave()), maxSampleSize)

		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Twice()
		require.NoError(t, inspector.Inspect(from, iHavesGreaterThanMaxSampleSize))
		require.NoError(t, inspector.Inspect(from, iHavesLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			// rpc with iHaves greater than configured max sample size should be truncated to MessageCountThreshold
			shouldBeTruncated := len(iHavesGreaterThanMaxSampleSize.GetControl().GetIhave()) == maxSampleSize
			// rpc with iHaves less than MessageCountThreshold should not be truncated
			shouldNotBeTruncated := len(iHavesLessThanMaxSampleSize.GetControl().GetIhave()) == 50
			return shouldBeTruncated && shouldNotBeTruncated
		}, time.Second, 500*time.Millisecond)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("ihave message ids truncation", func(t *testing.T) {
		maxMessageIDSampleSize := 1000
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IHave.MessageIdCountThreshold = maxMessageIDSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		// topic validation not performed so we can use random strings
		iHavesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(2000, unittest.IdentifierListFixture(10).Strings()...)...))
		iHavesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(50, unittest.IdentifierListFixture(10).Strings()...)...))

		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Twice()
		require.NoError(t, inspector.Inspect(from, iHavesGreaterThanMaxSampleSize))
		require.NoError(t, inspector.Inspect(from, iHavesLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			for _, iHave := range iHavesGreaterThanMaxSampleSize.GetControl().GetIhave() {
				// rpc with iHaves message ids greater than configured max sample size should be truncated to MessageCountThreshold
				if len(iHave.GetMessageIDs()) != maxMessageIDSampleSize {
					return false
				}
			}
			for _, iHave := range iHavesLessThanMaxSampleSize.GetControl().GetIhave() {
				// rpc with iHaves message ids less than MessageCountThreshold should not be truncated
				if len(iHave.GetMessageIDs()) != 50 {
					return false
				}
			}
			return true
		}, time.Second, 500*time.Millisecond)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("iwant message truncation", func(t *testing.T) {
		maxSampleSize := uint(100)
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IWant.MessageCountThreshold = maxSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		iWantsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(200, 200)...))
		require.Greater(t, uint(len(iWantsGreaterThanMaxSampleSize.GetControl().GetIwant())), maxSampleSize)
		iWantsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(50, 200)...))
		require.Less(t, uint(len(iWantsLessThanMaxSampleSize.GetControl().GetIwant())), maxSampleSize)

		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Twice()
		require.NoError(t, inspector.Inspect(from, iWantsGreaterThanMaxSampleSize))
		require.NoError(t, inspector.Inspect(from, iWantsLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			// rpc with iWants greater than configured max sample size should be truncated to MessageCountThreshold
			shouldBeTruncated := len(iWantsGreaterThanMaxSampleSize.GetControl().GetIwant()) == int(maxSampleSize)
			// rpc with iWants less than MessageCountThreshold should not be truncated
			shouldNotBeTruncated := len(iWantsLessThanMaxSampleSize.GetControl().GetIwant()) == 50
			return shouldBeTruncated && shouldNotBeTruncated
		}, time.Second, 500*time.Millisecond)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("iwant message id truncation", func(t *testing.T) {
		maxMessageIDSampleSize := 1000
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IWant.MessageIdCountThreshold = maxMessageIDSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		iWantsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 2000)...))
		iWantsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 50)...))

		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Twice()
		require.NoError(t, inspector.Inspect(from, iWantsGreaterThanMaxSampleSize))
		require.NoError(t, inspector.Inspect(from, iWantsLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			for _, iWant := range iWantsGreaterThanMaxSampleSize.GetControl().GetIwant() {
				// rpc with iWants message ids greater than configured max sample size should be truncated to MessageCountThreshold
				if len(iWant.GetMessageIDs()) != maxMessageIDSampleSize {
					return false
				}
			}
			for _, iWant := range iWantsLessThanMaxSampleSize.GetControl().GetIwant() {
				// rpc with iWants less than MessageCountThreshold should not be truncated
				if len(iWant.GetMessageIDs()) != 50 {
					return false
				}
			}
			return true
		}, time.Second, 500*time.Millisecond)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})
}

// TestControlMessageInspection_ValidRpc ensures inspector does not disseminate invalid control message notifications for a valid RPC.
func TestControlMessageInspection_ValidRpc(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	defer consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification")

	topics := []string{
		fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID),
		fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID),
		fmt.Sprintf("%s/%s", channels.SyncCommittee, sporkID),
		fmt.Sprintf("%s/%s", channels.RequestChunks, sporkID),
	}
	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics(topics)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	grafts := unittest.P2PRPCGraftFixtures(topics...)
	prunes := unittest.P2PRPCPruneFixtures(topics...)
	ihaves := unittest.P2PRPCIHaveFixtures(50, topics...)
	iwants := unittest.P2PRPCIWantFixtures(2, 50)
	pubsubMsgs := unittest.GossipSubMessageFixtures(10, topics[0])

	rpc := unittest.P2PRPCFixture(
		unittest.WithGrafts(grafts...),
		unittest.WithPrunes(prunes...),
		unittest.WithIHaves(ihaves...),
		unittest.WithIWants(iwants...),
		unittest.WithPubsubMessages(pubsubMsgs...))
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Run(func(args mock.Arguments) {
		id, ok := args[0].(string)
		require.True(t, ok)
		for _, iwant := range iwants {
			for _, messageID := range iwant.GetMessageIDs() {
				if id == messageID {
					return
				}
			}
		}
		require.Fail(t, "message id not found in iwant messages")
	})

	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	require.NoError(t, inspector.Inspect(from, rpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestGraftInspection_InvalidTopic_BelowThreshold ensures inspector does not disseminate an invalid control message notification for
// graft messages when the invalid topic id count does not exceed the configured threshold.
func TestGraftInspection_InvalidTopic_BelowThreshold(t *testing.T) {
	c, err := config.DefaultConfig()
	require.NoError(t, err)
	cfg := &c.NetworkConfig.GossipSub.RpcInspector.Validation
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})

	var unknownTopicGrafts []*pubsub_pb.ControlGraft
	var malformedTopicGrafts []*pubsub_pb.ControlGraft
	var invalidSporkIDTopicGrafts []*pubsub_pb.ControlGraft
	var allTopics []string
	for i := 0; i < cfg.GraftPrune.InvalidTopicIdThreshold; i++ {
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		allTopics = append(allTopics, unknownTopic, malformedTopic, invalidSporkIDTopic)
		unknownTopicGrafts = append(unknownTopicGrafts, unittest.P2PRPCGraftFixture(&unknownTopic))
		malformedTopicGrafts = append(malformedTopicGrafts, unittest.P2PRPCGraftFixture(&malformedTopic))
		invalidSporkIDTopicGrafts = append(invalidSporkIDTopicGrafts, unittest.P2PRPCGraftFixture(&invalidSporkIDTopic))
	}
	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics(allTopics)
	unknownTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(unknownTopicGrafts...))
	malformedTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(malformedTopicGrafts...))
	invalidSporkIDTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(invalidSporkIDTopicGrafts...))

	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Times(3)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif"))
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, unknownTopicReq))
	require.NoError(t, inspector.Inspect(from, malformedTopicReq))
	require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicReq))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 3
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestGraftInspection_InvalidTopic_AboveThreshold ensures inspector disseminates an invalid control message notification for
// graft messages when the invalid topic id count exceeds the configured threshold.
func TestGraftInspection_InvalidTopic_AboveThreshold(t *testing.T) {
	c, err := config.DefaultConfig()
	require.NoError(t, err)
	cfg := &c.NetworkConfig.GossipSub.RpcInspector.Validation
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Config = cfg
		params.Logger = logger
	})

	var unknownTopicGrafts []*pubsub_pb.ControlGraft
	var malformedTopicGrafts []*pubsub_pb.ControlGraft
	var invalidSporkIDTopicGrafts []*pubsub_pb.ControlGraft
	var allTopics []string
	for i := 0; i < cfg.GraftPrune.InvalidTopicIdThreshold+1; i++ {
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		allTopics = append(allTopics, unknownTopic, malformedTopic, invalidSporkIDTopic)
		unknownTopicGrafts = append(unknownTopicGrafts, unittest.P2PRPCGraftFixture(&unknownTopic))
		malformedTopicGrafts = append(malformedTopicGrafts, unittest.P2PRPCGraftFixture(&malformedTopic))
		invalidSporkIDTopicGrafts = append(invalidSporkIDTopicGrafts, unittest.P2PRPCGraftFixture(&invalidSporkIDTopic))
	}

	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics(allTopics)
	unknownTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(unknownTopicGrafts...))
	malformedTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(malformedTopicGrafts...))
	invalidSporkIDTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(invalidSporkIDTopicGrafts...))

	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Times(3)
	checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgGraft, validation.IsInvalidTopicIDThresholdExceeded, p2p.CtrlMsgNonClusterTopicType)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, unknownTopicReq))
	require.NoError(t, inspector.Inspect(from, malformedTopicReq))
	require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicReq))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 3
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestGraftInspection_DuplicateTopicIds_BelowThreshold ensures inspector does not disseminate invalid control message notifications
// for a valid RPC with duplicate graft topic ids below the threshold.
func TestGraftInspection_DuplicateTopicIds_BelowThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	duplicateTopic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics([]string{duplicateTopic})
	var grafts []*pubsub_pb.ControlGraft
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.GraftPrune.DuplicateTopicIdThreshold; i++ {
		grafts = append(grafts, unittest.P2PRPCGraftFixture(&duplicateTopic))
	}
	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	rpc := unittest.P2PRPCFixture(unittest.WithGrafts(grafts...))
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	// no notification should be disseminated for valid messages as long as the number of duplicates is below the threshold
	consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif"))

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 100*time.Millisecond, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

func TestGraftInspection_DuplicateTopicIds_AboveThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	duplicateTopic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics([]string{duplicateTopic})
	var grafts []*pubsub_pb.ControlGraft
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.GraftPrune.DuplicateTopicIdThreshold+2; i++ {
		grafts = append(grafts, unittest.P2PRPCGraftFixture(&duplicateTopic))
	}
	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	rpc := unittest.P2PRPCFixture(unittest.WithGrafts(grafts...))
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()

	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(func(args mock.Arguments) {
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, notification.TopicType, p2p.CtrlMsgNonClusterTopicType, "expected p2p.CtrlMsgNonClusterTopicType notification type, no RPC with cluster prefixed topic sent in this test")
		require.Equal(t, from, notification.PeerID)
		require.Equal(t, p2pmsg.CtrlMsgGraft, notification.MsgType)
		require.True(t, validation.IsDuplicateTopicIDThresholdExceeded(notification.Error))
	})

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 100*time.Millisecond, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestPruneInspection_InvalidTopic_BelowThreshold ensures inspector does not disseminate an invalid control message notification for
// prune messages when the invalid topic id count does not exceed the configured threshold.
func TestPruneInspection_InvalidTopic_BelowThreshold(t *testing.T) {
	c, err := config.DefaultConfig()
	require.NoError(t, err)
	cfg := &c.NetworkConfig.GossipSub.RpcInspector.Validation
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Config = cfg
	})

	var unknownTopicPrunes []*pubsub_pb.ControlPrune
	var malformedTopicPrunes []*pubsub_pb.ControlPrune
	var invalidSporkIDTopicPrunes []*pubsub_pb.ControlPrune
	var allTopics []string
	for i := 0; i < cfg.GraftPrune.InvalidTopicIdThreshold; i++ {
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		allTopics = append(allTopics, unknownTopic, malformedTopic, invalidSporkIDTopic)
		unknownTopicPrunes = append(unknownTopicPrunes, unittest.P2PRPCPruneFixture(&unknownTopic))
		malformedTopicPrunes = append(malformedTopicPrunes, unittest.P2PRPCPruneFixture(&malformedTopic))
		invalidSporkIDTopicPrunes = append(invalidSporkIDTopicPrunes, unittest.P2PRPCPruneFixture(&invalidSporkIDTopic))
	}

	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics(allTopics)
	unknownTopicReq := unittest.P2PRPCFixture(unittest.WithPrunes(unknownTopicPrunes...))
	malformedTopicReq := unittest.P2PRPCFixture(unittest.WithPrunes(malformedTopicPrunes...))
	invalidSporkIDTopicReq := unittest.P2PRPCFixture(unittest.WithPrunes(invalidSporkIDTopicPrunes...))

	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Times(3)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	// no notification should be disseminated for valid messages as long as the number of invalid topic ids is below the threshold
	consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif"))
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, unknownTopicReq))
	require.NoError(t, inspector.Inspect(from, malformedTopicReq))
	require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicReq))

	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(2 * time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestPruneInspection_InvalidTopic_AboveThreshold ensures inspector disseminates an invalid control message notification for
// prune messages when the invalid topic id count exceeds the configured threshold.
func TestPruneInspection_InvalidTopic_AboveThreshold(t *testing.T) {
	c, err := config.DefaultConfig()
	require.NoError(t, err)
	cfg := &c.NetworkConfig.GossipSub.RpcInspector.Validation
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Config = cfg
	})

	var unknownTopicPrunes []*pubsub_pb.ControlPrune
	var malformedTopicPrunes []*pubsub_pb.ControlPrune
	var invalidSporkIDTopicPrunes []*pubsub_pb.ControlPrune
	var allTopics []string
	for i := 0; i < cfg.GraftPrune.InvalidTopicIdThreshold+1; i++ {
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		allTopics = append(allTopics, unknownTopic, malformedTopic, invalidSporkIDTopic)
		unknownTopicPrunes = append(unknownTopicPrunes, unittest.P2PRPCPruneFixture(&unknownTopic))
		malformedTopicPrunes = append(malformedTopicPrunes, unittest.P2PRPCPruneFixture(&malformedTopic))
		invalidSporkIDTopicPrunes = append(invalidSporkIDTopicPrunes, unittest.P2PRPCPruneFixture(&invalidSporkIDTopic))
	}

	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics(allTopics)
	unknownTopicReq := unittest.P2PRPCFixture(unittest.WithPrunes(unknownTopicPrunes...))
	malformedTopicReq := unittest.P2PRPCFixture(unittest.WithPrunes(malformedTopicPrunes...))
	invalidSporkIDTopicReq := unittest.P2PRPCFixture(unittest.WithPrunes(invalidSporkIDTopicPrunes...))

	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Times(3)
	checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgPrune, validation.IsInvalidTopicIDThresholdExceeded, p2p.CtrlMsgNonClusterTopicType)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, unknownTopicReq))
	require.NoError(t, inspector.Inspect(from, malformedTopicReq))
	require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicReq))

	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestPruneInspection_DuplicateTopicIds_AboveThreshold ensures inspector disseminates an invalid control message notification for
// prune messages when the number of duplicate topic ids is above the threshold.
func TestPruneInspection_DuplicateTopicIds_AboveThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	duplicateTopic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics([]string{duplicateTopic})
	var prunes []*pubsub_pb.ControlPrune
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// we need threshold + 1 to trigger the invalid control message notification; as the first duplicate topic id is not counted
	for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.GraftPrune.DuplicateTopicIdThreshold+2; i++ {
		prunes = append(prunes, unittest.P2PRPCPruneFixture(&duplicateTopic))
	}
	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	rpc := unittest.P2PRPCFixture(unittest.WithPrunes(prunes...))
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()

	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(func(args mock.Arguments) {
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, notification.TopicType, p2p.CtrlMsgNonClusterTopicType, "expected p2p.CtrlMsgNonClusterTopicType notification type, no RPC with cluster prefixed topic sent in this test")
		require.Equal(t, from, notification.PeerID)
		require.Equal(t, p2pmsg.CtrlMsgPrune, notification.MsgType)
		require.True(t, validation.IsDuplicateTopicIDThresholdExceeded(notification.Error))
	})

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 100*time.Millisecond, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestPruneInspection_DuplicateTopicIds_BelowThreshold ensures inspector does not disseminate invalid control message notifications
// for a valid RPC with duplicate prune topic ids below the threshold.
func TestPrueInspection_DuplicateTopicIds_BelowThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	duplicateTopic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics([]string{duplicateTopic})
	var prunes []*pubsub_pb.ControlPrune
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.GraftPrune.DuplicateTopicIdThreshold; i++ {
		prunes = append(prunes, unittest.P2PRPCPruneFixture(&duplicateTopic))
	}
	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	rpc := unittest.P2PRPCFixture(unittest.WithPrunes(prunes...))
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()

	// no notification should be disseminated for valid messages as long as the number of duplicates is below the threshold
	consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif"))

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 100*time.Millisecond, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestIHaveInspection_InvalidTopic_AboveThreshold ensures inspector disseminates an invalid control message notification for
// ihave messages when the invalid topic id count exceeds the configured threshold.
func TestIHaveInspection_InvalidTopic_AboveThreshold(t *testing.T) {
	c, err := config.DefaultConfig()
	require.NoError(t, err)
	cfg := &c.NetworkConfig.GossipSub.RpcInspector.Validation
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Config = cfg
		params.Logger = logger
	})

	var unknownTopicIHaves []*pubsub_pb.ControlIHave
	var malformedTopicIHaves []*pubsub_pb.ControlIHave
	var invalidSporkIDTopicIHaves []*pubsub_pb.ControlIHave
	var allTopics []string
	for i := 0; i < cfg.GraftPrune.InvalidTopicIdThreshold+1; i++ {
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		allTopics = append(allTopics, unknownTopic, malformedTopic, invalidSporkIDTopic)
		unknownTopicIHaves = append(unknownTopicIHaves, unittest.P2PRPCIHaveFixture(&unknownTopic, unittest.IdentifierListFixture(5).Strings()...))
		malformedTopicIHaves = append(malformedTopicIHaves, unittest.P2PRPCIHaveFixture(&malformedTopic, unittest.IdentifierListFixture(5).Strings()...))
		invalidSporkIDTopicIHaves = append(invalidSporkIDTopicIHaves, unittest.P2PRPCIHaveFixture(&invalidSporkIDTopic, unittest.IdentifierListFixture(5).Strings()...))
	}

	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics(allTopics)
	unknownTopicReq := unittest.P2PRPCFixture(unittest.WithIHaves(unknownTopicIHaves...))
	malformedTopicReq := unittest.P2PRPCFixture(unittest.WithIHaves(malformedTopicIHaves...))
	invalidSporkIDTopicReq := unittest.P2PRPCFixture(unittest.WithIHaves(invalidSporkIDTopicIHaves...))

	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Times(3)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIHave, validation.IsInvalidTopicIDThresholdExceeded, p2p.CtrlMsgNonClusterTopicType)
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, unknownTopicReq))
	require.NoError(t, inspector.Inspect(from, malformedTopicReq))
	require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicReq))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 3
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestIHaveInspection_InvalidTopic_BelowThreshold ensures inspector does not disseminate an invalid control message notification for
// ihave messages when the invalid topic id count does not exceed the configured threshold.
func TestIHaveInspection_InvalidTopic_BelowThreshold(t *testing.T) {
	c, err := config.DefaultConfig()
	require.NoError(t, err)
	cfg := &c.NetworkConfig.GossipSub.RpcInspector.Validation
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Config = cfg
		params.Logger = logger
	})

	var unknownTopicIHaves []*pubsub_pb.ControlIHave
	var malformedTopicIHaves []*pubsub_pb.ControlIHave
	var invalidSporkIDTopicIHaves []*pubsub_pb.ControlIHave
	var allTopics []string
	for i := 0; i < cfg.GraftPrune.InvalidTopicIdThreshold; i++ {
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		allTopics = append(allTopics, unknownTopic, malformedTopic, invalidSporkIDTopic)
		unknownTopicIHaves = append(unknownTopicIHaves, unittest.P2PRPCIHaveFixture(&unknownTopic, unittest.IdentifierListFixture(5).Strings()...))
		malformedTopicIHaves = append(malformedTopicIHaves, unittest.P2PRPCIHaveFixture(&malformedTopic, unittest.IdentifierListFixture(5).Strings()...))
		invalidSporkIDTopicIHaves = append(invalidSporkIDTopicIHaves, unittest.P2PRPCIHaveFixture(&invalidSporkIDTopic, unittest.IdentifierListFixture(5).Strings()...))
	}

	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics(allTopics)
	unknownTopicReq := unittest.P2PRPCFixture(unittest.WithIHaves(unknownTopicIHaves...))
	malformedTopicReq := unittest.P2PRPCFixture(unittest.WithIHaves(malformedTopicIHaves...))
	invalidSporkIDTopicReq := unittest.P2PRPCFixture(unittest.WithIHaves(invalidSporkIDTopicIHaves...))

	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Times(3)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	// no notification should be disseminated for valid messages as long as the number of invalid topic ids is below the threshold
	consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif"))
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, unknownTopicReq))
	require.NoError(t, inspector.Inspect(from, malformedTopicReq))
	require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicReq))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 3
	}, time.Second, 500*time.Millisecond)

	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestIHaveInspection_DuplicateTopicIds_BelowThreshold ensures inspector does not disseminate an invalid control message notification for
// iHave messages when duplicate topic ids are below allowed threshold.
func TestIHaveInspection_DuplicateTopicIds_BelowThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	validTopic := fmt.Sprintf("%s/%s", channels.PushBlocks.String(), sporkID)
	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics([]string{validTopic})

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	validTopicIHave := unittest.P2PRPCIHaveFixture(&validTopic, unittest.IdentifierListFixture(5).Strings()...)
	ihaves := []*pubsub_pb.ControlIHave{validTopicIHave}
	// duplicate the valid topic id on other iHave messages but with different message ids
	for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.IHave.DuplicateTopicIdThreshold-1; i++ {
		ihaves = append(ihaves, unittest.P2PRPCIHaveFixture(&validTopic, unittest.IdentifierListFixture(5).Strings()...))
	}
	// creates an RPC with duplicate topic ids but different message ids
	duplicateMsgIDRpc := unittest.P2PRPCFixture(unittest.WithIHaves(ihaves...))
	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()

	// no notification should be disseminated for valid messages as long as the number of duplicates is below the threshold
	consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif"))
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, duplicateMsgIDRpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestIHaveInspection_DuplicateTopicIds_AboveThreshold ensures inspector disseminate an invalid control message notification for
// iHave messages when duplicate topic ids are above allowed threshold.
func TestIHaveInspection_DuplicateTopicIds_AboveThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	validTopic := fmt.Sprintf("%s/%s", channels.PushBlocks.String(), sporkID)
	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics([]string{validTopic})

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	validTopicIHave := unittest.P2PRPCIHaveFixture(&validTopic, unittest.IdentifierListFixture(5).Strings()...)
	ihaves := []*pubsub_pb.ControlIHave{validTopicIHave}
	// duplicate the valid topic id on other iHave messages but with different message ids up to the threshold
	for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.IHave.DuplicateTopicIdThreshold+2; i++ {
		ihaves = append(ihaves, unittest.P2PRPCIHaveFixture(&validTopic, unittest.IdentifierListFixture(5).Strings()...))
	}
	// creates an RPC with duplicate topic ids but different message ids
	duplicateMsgIDRpc := unittest.P2PRPCFixture(unittest.WithIHaves(ihaves...))
	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()

	// one notification should be disseminated for invalid messages when the number of duplicates exceeds the threshold
	checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIHave, validation.IsDuplicateTopicIDThresholdExceeded, p2p.CtrlMsgNonClusterTopicType)
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, duplicateMsgIDRpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestIHaveInspection_DuplicateMessageIds_BelowThreshold ensures inspector does not disseminate an invalid control message notification for
// iHave messages when duplicate message ids are below allowed threshold.
func TestIHaveInspection_DuplicateMessageIds_BelowThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	validTopic := fmt.Sprintf("%s/%s", channels.PushBlocks.String(), sporkID)
	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics([]string{validTopic})
	duplicateMsgID := unittest.IdentifierFixture()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	msgIds := flow.IdentifierList{}
	// includes as many duplicates as allowed by the threshold
	for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.IHave.DuplicateMessageIdThreshold; i++ {
		msgIds = append(msgIds, duplicateMsgID)
	}
	duplicateMsgIDIHave := unittest.P2PRPCIHaveFixture(&validTopic, append(msgIds, unittest.IdentifierListFixture(5)...).Strings()...)
	duplicateMsgIDRpc := unittest.P2PRPCFixture(unittest.WithIHaves(duplicateMsgIDIHave))
	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()

	// no notification should be disseminated for valid messages as long as the number of duplicates is below the threshold
	consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif"))
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, duplicateMsgIDRpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestIHaveInspection_DuplicateMessageIds_AboveThreshold ensures inspector disseminates an invalid control message notification for
// iHave messages when duplicate message ids are above allowed threshold.
func TestIHaveInspection_DuplicateMessageIds_AboveThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	validTopic := fmt.Sprintf("%s/%s", channels.PushBlocks.String(), sporkID)
	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics([]string{validTopic})
	duplicateMsgID := unittest.IdentifierFixture()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	msgIds := flow.IdentifierList{}
	// includes as many duplicates as beyond the threshold
	for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.IHave.DuplicateMessageIdThreshold+2; i++ {
		msgIds = append(msgIds, duplicateMsgID)
	}
	duplicateMsgIDIHave := unittest.P2PRPCIHaveFixture(&validTopic, append(msgIds, unittest.IdentifierListFixture(5)...).Strings()...)
	duplicateMsgIDRpc := unittest.P2PRPCFixture(unittest.WithIHaves(duplicateMsgIDIHave))
	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()

	// one notification should be disseminated for invalid messages when the number of duplicates exceeds the threshold
	checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIHave, validation.IsDuplicateMessageIDErr, p2p.CtrlMsgNonClusterTopicType)
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, duplicateMsgIDRpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestIWantInspection_DuplicateMessageIds_BelowThreshold ensures inspector does not disseminate an invalid control message notification for
// iWant messages when duplicate message ids are below allowed threshold.
func TestIWantInspection_DuplicateMessageIds_BelowThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	// oracle must be set even though iWant messages do not have topic IDs
	duplicateMsgID := unittest.IdentifierFixture()
	duplicates := flow.IdentifierList{}
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// includes as many duplicates as allowed by the threshold
	for i := 0; i < int(cfg.NetworkConfig.GossipSub.RpcInspector.Validation.IWant.DuplicateMsgIdThreshold)-2; i++ {
		duplicates = append(duplicates, duplicateMsgID)
	}
	msgIds := append(duplicates, unittest.IdentifierListFixture(5)...).Strings()
	duplicateMsgIDIWant := unittest.P2PRPCIWantFixture(msgIds...)

	duplicateMsgIDRpc := unittest.P2PRPCFixture(unittest.WithIWants(duplicateMsgIDIWant))

	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	// no notification should be disseminated for valid messages as long as the number of duplicates is below the threshold
	consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif"))
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Run(func(args mock.Arguments) {
		id, ok := args[0].(string)
		require.True(t, ok)
		require.Contains(t, msgIds, id)
	}).Maybe() // if iwant message ids count are not bigger than cache miss check size, this method is not called, anyway in this test we do not care about this method.

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, duplicateMsgIDRpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestIWantInspection_DuplicateMessageIds_AboveThreshold ensures inspector disseminates invalid control message notifications for iWant messages when duplicate message ids exceeds allowed threshold.
func TestIWantInspection_DuplicateMessageIds_AboveThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	// oracle must be set even though iWant messages do not have topic IDs
	duplicateMsgID := unittest.IdentifierFixture()
	duplicates := flow.IdentifierList{}
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// includes as many duplicates as allowed by the threshold
	for i := 0; i < int(cfg.NetworkConfig.GossipSub.RpcInspector.Validation.IWant.DuplicateMsgIdThreshold)+2; i++ {
		duplicates = append(duplicates, duplicateMsgID)
	}
	msgIds := append(duplicates, unittest.IdentifierListFixture(5)...).Strings()
	duplicateMsgIDIWant := unittest.P2PRPCIWantFixture(msgIds...)

	duplicateMsgIDRpc := unittest.P2PRPCFixture(unittest.WithIWants(duplicateMsgIDIWant))

	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIWant, validation.IsIWantDuplicateMsgIDThresholdErr, p2p.CtrlMsgNonClusterTopicType)
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Run(func(args mock.Arguments) {
		id, ok := args[0].(string)
		require.True(t, ok)
		require.Contains(t, msgIds, id)
	}).Maybe() // if iwant message ids count are not bigger than cache miss check size, this method is not called, anyway in this test we do not care about this method.

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, duplicateMsgIDRpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestIWantInspection_CacheMiss_AboveThreshold ensures inspector disseminates invalid control message notifications for iWant messages when cache misses exceeds allowed threshold.
func TestIWantInspection_CacheMiss_AboveThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
		// set high cache miss threshold to ensure we only disseminate notification when it is exceeded
		params.Config.IWant.CacheMissThreshold = 900
		params.Logger = logger
	})
	// 10 iwant messages, each with 100 message ids; total of 1000 message ids, which when imitated as cache misses should trigger notification dissemination.
	inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 100)...))

	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIWant, validation.IsIWantCacheMissThresholdErr, p2p.CtrlMsgNonClusterTopicType)
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	// return false each time to eventually force a notification to be disseminated when the cache miss count finally exceeds the 90% threshold
	allIwantsChecked := sync.WaitGroup{}
	allIwantsChecked.Add(901) // 901 message ids
	rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(false).Run(func(args mock.Arguments) {
		defer allIwantsChecked.Done()

		id, ok := args[0].(string)
		require.True(t, ok)
		found := false
		for _, iwant := range inspectMsgRpc.GetControl().GetIwant() {
			for _, messageID := range iwant.GetMessageIDs() {
				if id == messageID {
					found = true
				}
			}
		}
		require.True(t, found)
	})

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
	unittest.RequireReturnsBefore(t, allIwantsChecked.Wait, 1*time.Second, "all iwant messages should be checked for cache misses")

	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

func TestIWantInspection_CacheMiss_BelowThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
		// set high cache miss threshold to ensure that we do not disseminate notification in this test
		params.Config.IWant.CacheMissThreshold = 99
		params.Logger = logger
	})
	// oracle must be set even though iWant messages do not have topic IDs
	defer consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification")

	msgIds := unittest.IdentifierListFixture(98).Strings() // one less than cache miss threshold
	inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixture(msgIds...)))
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()

	allIwantsChecked := sync.WaitGroup{}
	allIwantsChecked.Add(len(msgIds))
	// returns false each time to imitate cache misses; however, since the number of cache misses is below the threshold, no notification should be disseminated.
	rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(false).Run(func(args mock.Arguments) {
		defer allIwantsChecked.Done()
		id, ok := args[0].(string)
		require.True(t, ok)
		require.Contains(t, msgIds, id)
	})

	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
	unittest.RequireReturnsBefore(t, allIwantsChecked.Wait, 1*time.Second, "all iwant messages should be checked for cache misses")

	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestControlMessageInspection_ExceedingErrThreshold ensures inspector disseminates invalid control message notifications for RPCs that exceed the configured error threshold.
func TestPublishMessageInspection_ExceedingErrThreshold(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	errThreshold := 500
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Config.PublishMessages.ErrorThreshold = errThreshold
		params.Logger = logger
	})
	// create unknown topic
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", unittest.IdentifierFixture(), sporkID)).String()
	// create malformed topic
	malformedTopic := channels.Topic("!@#$%^&**((").String()
	// a topics spork ID is considered invalid if it does not match the current spork ID
	invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture())).String()
	publisher := unittest.PeerIdFixture(t)
	publisherId := unittest.IdentityFixture()
	// create 10 normal messages
	pubsubMsgs := unittest.GossipSubMessageFixtures(50, fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID), p2ptest.WithFrom(publisher))
	// add 550 invalid messages to force notification dissemination
	invalidMessageFixtures := []*pubsub_pb.Message{
		{Topic: &unknownTopic, From: []byte(publisher)},
		{Topic: &malformedTopic, From: []byte(publisher)},
		{Topic: &invalidSporkIDTopic, From: []byte(publisher)},
	}
	for i := 0; i < errThreshold+1; i++ {
		pubsubMsgs = append(pubsubMsgs, invalidMessageFixtures[rand.Intn(len(invalidMessageFixtures))])
	}
	rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
	topics := make([]string, len(pubsubMsgs))
	for i, msg := range pubsubMsgs {
		topics[i] = *msg.Topic
	}

	// set topic oracle to return list of topics to avoid hasSubscription errors and force topic validation
	topicProviderOracle.UpdateTopics(topics)
	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	idProvider.On("ByPeerID", publisher).Return(publisherId, true).Times(len(rpc.Publish))
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()

	checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr, p2p.CtrlMsgNonClusterTopicType)
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestControlMessageInspection_MissingSubscription ensures inspector disseminates invalid control message notifications for RPCs that the peer is not subscribed to.
func TestPublishMessageInspection_MissingSubscription(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	errThreshold := 500
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Config.PublishMessages.ErrorThreshold = errThreshold
		params.Logger = logger
	})
	publisher := unittest.PeerIdFixture(t)
	publisherId := unittest.IdentityFixture()
	pubsubMsgs := unittest.GossipSubMessageFixtures(errThreshold+1, fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID), p2ptest.WithFrom(publisher))
	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	idProvider.On("ByPeerID", publisher).Return(publisherId, true).Times(len(pubsubMsgs))
	rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
	checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr, p2p.CtrlMsgNonClusterTopicType)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestPublishMessageInspection_MissingTopic ensures inspector disseminates invalid control message notifications for published messages with missing topics.
func TestPublishMessageInspection_MissingTopic(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	errThreshold := 500
	inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
		// 5 invalid pubsub messages will force notification dissemination
		params.Config.PublishMessages.ErrorThreshold = errThreshold
		params.Logger = logger
	})
	publisher := unittest.PeerIdFixture(t)
	publisherId := unittest.IdentityFixture()
	pubsubMsgs := unittest.GossipSubMessageFixtures(errThreshold+1, "", p2ptest.WithFrom(publisher))
	rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
	for _, msg := range pubsubMsgs {
		msg.Topic = nil
	}
	from := unittest.PeerIdFixture(t)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
	idProvider.On("ByPeerID", publisher).Return(publisherId, true).Times(len(pubsubMsgs))
	checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr, p2p.CtrlMsgNonClusterTopicType)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestRpcInspectionDeactivatedOnPublicNetwork ensures inspector does not inspect RPCs on public networks.
func TestRpcInspectionDeactivatedOnPublicNetwork(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, _, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
		params.NetworkingType = network.PublicNetwork
	})
	from := unittest.PeerIdFixture(t)
	defer idProvider.AssertNotCalled(t, "ByPeerID", from)
	topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
	topicProviderOracle.UpdateTopics([]string{topic})
	pubsubMsgs := unittest.GossipSubMessageFixtures(10, topic, unittest.WithFrom(from))
	rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	require.NoError(t, inspector.Inspect(from, rpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestInspection_Unstaked_Peer ensures inspector disseminates invalid control message notifications for rpc's from unstaked peers when running private network.
func TestInspection_Unstaked_Peer(t *testing.T) {
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		// override the inspector and params, run the inspector in private mode
		params.NetworkingType = network.PrivateNetwork
	})
	unstakedPeer := unittest.PeerIdFixture(t)
	topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
	topicProviderOracle.UpdateTopics([]string{topic})
	idProvider.On("ByPeerID", unstakedPeer).Return(nil, false).Once()
	checkNotification := checkNotificationFunc(t, unstakedPeer, p2pmsg.CtrlMsgRPC, validation.IsErrUnstakedPeer, p2p.CtrlMsgNonClusterTopicType)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.Error(t, inspector.Inspect(unstakedPeer, unittest.P2PRPCFixture()))
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestControlMessageInspection_Unstaked_From ensures inspector disseminates invalid control message notifications for published messages from unstaked peers.
func TestPublishMessageInspection_Unstaked_From(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		// override the inspector and params, run the inspector in private mode
		params.NetworkingType = network.PrivateNetwork
		params.Config.GraftPrune.InvalidTopicIdThreshold = 0
		params.Logger = logger
	})
	from := unittest.PeerIdFixture(t)
	unstakedPeer := unittest.PeerIdFixture(t)
	topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
	topicProviderOracle.UpdateTopics([]string{topic})
	// default RpcMessageErrorThreshold is 500, 501 messages should trigger a notification
	pubsubMsgs := unittest.GossipSubMessageFixtures(501, topic, unittest.WithFrom(unstakedPeer))
	idProvider.On("ByPeerID", unstakedPeer).Return(nil, false).Times(501)
	idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Times(1)
	rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
	checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr, p2p.CtrlMsgNonClusterTopicType)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestControlMessageInspection_Ejected_From ensures inspector disseminates invalid control message notifications for published messages from ejected peers.
func TestPublishMessageInspection_Ejected_From(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		// override the inspector and params, run the inspector in private mode
		params.NetworkingType = network.PrivateNetwork
		params.Config.GraftPrune.InvalidTopicIdThreshold = 0
		params.Logger = logger
	})

	from := unittest.PeerIdFixture(t)
	id := unittest.IdentityFixture()

	ejectedNode := unittest.PeerIdFixture(t)
	ejectedId := unittest.IdentityFixture()
	ejectedId.EpochParticipationStatus = flow.EpochParticipationStatusEjected

	topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
	topicProviderOracle.UpdateTopics([]string{topic})
	pubsubMsgs := unittest.GossipSubMessageFixtures(501, topic, unittest.WithFrom(ejectedNode))
	idProvider.On("ByPeerID", ejectedNode).Return(ejectedId, true).Times(501)
	idProvider.On("ByPeerID", from).Return(id, true).Once()

	rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
	checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr, p2p.CtrlMsgNonClusterTopicType)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	require.Eventually(t, func() bool {
		return logCounter.Load() == 1
	}, time.Second, 500*time.Millisecond)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestNewControlMsgValidationInspector_validateClusterPrefixedTopic ensures cluster prefixed topics are validated as expected.
func TestNewControlMsgValidationInspector_validateClusterPrefixedTopic(t *testing.T) {
	t.Run("validateClusterPrefixedTopic should not return an error for valid cluster prefixed topics", func(t *testing.T) {
		logCounter := atomic.NewInt64(0)
		logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)

		inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Logger = logger
		})
		defer consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), sporkID)).String()
		topicProviderOracle.UpdateTopics([]string{clusterPrefixedTopic})
		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic)))
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		inspector.ActiveClustersChanged(flow.ChainIDList{clusterID, flow.ChainID(unittest.IdentifierFixture().String()), flow.ChainID(unittest.IdentifierFixture().String())})
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		require.Eventually(t, func() bool {
			return logCounter.Load() == 1
		}, time.Second, 500*time.Millisecond)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("validateClusterPrefixedTopic should not return error if cluster prefixed hard threshold not exceeded for unknown cluster ids", func(t *testing.T) {
		logCounter := atomic.NewInt64(0)
		logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
		inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			// set hard threshold to small number , ensure that a single unknown cluster prefix id does not cause a notification to be disseminated
			params.Config.ClusterPrefixedMessage.HardThreshold = 2
			params.Logger = logger
		})
		defer consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), sporkID)).String()
		from := unittest.PeerIdFixture(t)
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic)))
		id := unittest.IdentityFixture()
		idProvider.On("ByPeerID", from).Return(id, true).Once()
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		require.Eventually(t, func() bool {
			return logCounter.Load() == 1
		}, time.Second, 500*time.Millisecond)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("validateClusterPrefixedTopic should return error if cluster prefixed hard threshold exceeded for unknown cluster ids", func(t *testing.T) {
		logCounter := atomic.NewInt64(0)
		logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)
		inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
			// the 11th unknown cluster ID error should cause an error
			params.Config.ClusterPrefixedMessage.HardThreshold = 10
			params.Config.GraftPrune.InvalidTopicIdThreshold = 0
			params.Logger = logger
		})
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), sporkID)).String()
		topicProviderOracle.UpdateTopics([]string{clusterPrefixedTopic})
		from := unittest.PeerIdFixture(t)
		identity := unittest.IdentityFixture()
		idProvider.On("ByPeerID", from).Return(identity, true).Times(11)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgGraft, validation.IsInvalidTopicIDThresholdExceeded, p2p.CtrlMsgTopicTypeClusterPrefixed)
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic)))
		inspector.ActiveClustersChanged(flow.ChainIDList{flow.ChainID(unittest.IdentifierFixture().String())})
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		for i := 0; i < 11; i++ {
			require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		}
		require.Eventually(t, func() bool {
			return logCounter.Load() == 11
		}, time.Second, 100*time.Millisecond)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})
}

// TestControlMessageValidationInspector_ActiveClustersChanged validates the expected update of the active cluster IDs list.
func TestControlMessageValidationInspector_ActiveClustersChanged(t *testing.T) {
	logCounter := atomic.NewInt64(0)
	logger := hookedLogger(logCounter, zerolog.TraceLevel, worker.QueuedItemProcessedLog)

	inspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Logger = logger
	})
	defer consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification")
	identity := unittest.IdentityFixture()
	idProvider.On("ByPeerID", mock.AnythingOfType("peer.ID")).Return(identity, true).Times(5)
	activeClusterIds := make(flow.ChainIDList, 0)
	for _, id := range unittest.IdentifierListFixture(5) {
		activeClusterIds = append(activeClusterIds, flow.ChainID(id.String()))
	}
	inspector.ActiveClustersChanged(activeClusterIds)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	from := unittest.PeerIdFixture(t)
	for _, id := range activeClusterIds {
		topic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(id), sporkID)).String()
		rpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&topic)))
		require.NoError(t, inspector.Inspect(from, rpc))
	}
	// sleep for 1 second to ensure rpc's is processed
	require.Eventually(t, func() bool {
		return logCounter.Load() == int64(len(activeClusterIds))
	}, time.Second, 500*time.Millisecond)

	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestControlMessageValidationInspector_TruncationConfigToggle ensures that rpc's are not truncated when truncation is disabled through configs.
func TestControlMessageValidationInspector_TruncationConfigToggle(t *testing.T) {
	t.Run("should not perform truncation when disabled is set to true", func(t *testing.T) {
		numOfMsgs := 5000
		logCounter := atomic.NewInt64(0)
		logger := hookedLogger(logCounter, zerolog.TraceLevel, validation.RPCTruncationDisabledWarning, worker.QueuedItemProcessedLog)
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.GraftPrune.MessageCountThreshold = numOfMsgs
			params.Logger = logger
			// disable truncation for all control message types
			params.Config.InspectionProcess.Truncate.Disabled = true
		})

		// topic validation is ignored set any topic oracle
		consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		inspector.Start(signalerCtx)

		rpc := unittest.P2PRPCFixture(
			unittest.WithGrafts(unittest.P2PRPCGraftFixtures(unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(numOfMsgs, unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithIWants(unittest.P2PRPCIWantFixtures(numOfMsgs, numOfMsgs)...),
		)

		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
		require.NoError(t, inspector.Inspect(from, rpc))

		require.Eventually(t, func() bool {
			return logCounter.Load() == 2
		}, time.Second, 500*time.Millisecond)

		// ensure truncation not performed
		require.Len(t, rpc.GetControl().GetGraft(), numOfMsgs)
		require.Len(t, rpc.GetControl().GetPrune(), numOfMsgs)
		require.Len(t, rpc.GetControl().GetIhave(), numOfMsgs)
		ensureMessageIdsLen(t, p2pmsg.CtrlMsgIHave, rpc, numOfMsgs)
		require.Len(t, rpc.GetControl().GetIwant(), numOfMsgs)
		ensureMessageIdsLen(t, p2pmsg.CtrlMsgIWant, rpc, numOfMsgs)

		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("should not perform truncation when disabled for each individual control message type directly", func(t *testing.T) {
		numOfMsgs := 5000
		expectedLogStrs := []string{
			validation.GraftTruncationDisabledWarning,
			validation.PruneTruncationDisabledWarning,
			validation.IHaveTruncationDisabledWarning,
			validation.IHaveMessageIDTruncationDisabledWarning,
			validation.IWantTruncationDisabledWarning,
			validation.IWantMessageIDTruncationDisabledWarning,
			worker.QueuedItemProcessedLog,
		}
		logCounter := atomic.NewInt64(0)
		logger := hookedLogger(logCounter, zerolog.TraceLevel, expectedLogStrs...)
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.GraftPrune.MessageCountThreshold = numOfMsgs
			params.Logger = logger
			// disable truncation for all control message types individually
			params.Config.InspectionProcess.Truncate.EnableGraft = false
			params.Config.InspectionProcess.Truncate.EnablePrune = false
			params.Config.InspectionProcess.Truncate.EnableIHave = false
			params.Config.InspectionProcess.Truncate.EnableIHaveMessageIds = false
			params.Config.InspectionProcess.Truncate.EnableIWant = false
			params.Config.InspectionProcess.Truncate.EnableIWantMessageIds = false
		})

		// topic validation is ignored set any topic oracle
		consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		inspector.Start(signalerCtx)

		rpc := unittest.P2PRPCFixture(
			unittest.WithGrafts(unittest.P2PRPCGraftFixtures(unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(numOfMsgs, unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithIWants(unittest.P2PRPCIWantFixtures(numOfMsgs, numOfMsgs)...),
		)

		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
		require.NoError(t, inspector.Inspect(from, rpc))

		require.Eventually(t, func() bool {
			return logCounter.Load() == int64(len(expectedLogStrs))
		}, time.Second, 500*time.Millisecond)

		// ensure truncation not performed
		require.Len(t, rpc.GetControl().GetGraft(), numOfMsgs)
		require.Len(t, rpc.GetControl().GetPrune(), numOfMsgs)
		require.Len(t, rpc.GetControl().GetIhave(), numOfMsgs)
		ensureMessageIdsLen(t, p2pmsg.CtrlMsgIHave, rpc, numOfMsgs)
		require.Len(t, rpc.GetControl().GetIwant(), numOfMsgs)
		ensureMessageIdsLen(t, p2pmsg.CtrlMsgIWant, rpc, numOfMsgs)

		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})
}

// TestControlMessageValidationInspector_InspectionConfigToggle ensures that rpc's are not inspected when inspection is disabled through configs.
func TestControlMessageValidationInspector_InspectionConfigToggle(t *testing.T) {
	t.Run("should not perform inspection when disabled is set to true", func(t *testing.T) {
		numOfMsgs := 5000
		logCounter := atomic.NewInt64(0)
		logger := hookedLogger(logCounter, zerolog.TraceLevel, validation.RPCInspectionDisabledWarning)
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Logger = logger
			// disable inspector for all control message types
			params.Config.InspectionProcess.Inspect.Disabled = true
		})

		// notification consumer should never be called when inspection is disabled
		defer consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification")
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		inspector.Start(signalerCtx)

		rpc := unittest.P2PRPCFixture(
			unittest.WithGrafts(unittest.P2PRPCGraftFixtures(unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(numOfMsgs, unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithIWants(unittest.P2PRPCIWantFixtures(numOfMsgs, numOfMsgs)...),
		)

		from := unittest.PeerIdFixture(t)
		require.NoError(t, inspector.Inspect(from, rpc))

		require.Eventually(t, func() bool {
			return logCounter.Load() == 1
		}, time.Second, 500*time.Millisecond)

		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("should not check identity when reject-unstaked-peers is false", func(t *testing.T) {
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			// disable inspector for all control message types
			params.Config.InspectionProcess.Inspect.RejectUnstakedPeers = false
		})

		// notification consumer should never be called when inspection is disabled
		defer consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification")
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()

		from := unittest.PeerIdFixture(t)

		defer idProvider.AssertNotCalled(t, "ByPeerID", from)
		inspector.Start(signalerCtx)

		require.NoError(t, inspector.Inspect(from, unittest.P2PRPCFixture()))

		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("should check identity when reject-unstaked-peers is true", func(t *testing.T) {
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			// disable inspector for all control message types
			params.Config.InspectionProcess.Inspect.RejectUnstakedPeers = true
		})

		// notification consumer should never be called when inspection is disabled
		consumer.On("OnInvalidControlMessageNotification", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(func(args mock.Arguments) {
			notification, ok := args.Get(0).(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.True(t, validation.IsErrUnstakedPeer(notification.Error))
		})
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()

		from := unittest.PeerIdFixture(t)

		idProvider.On("ByPeerID", from).Return(nil, false).Once()
		inspector.Start(signalerCtx)

		require.Error(t, inspector.Inspect(from, unittest.P2PRPCFixture()))

		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("should not perform inspection when disabled for each individual control message type directly", func(t *testing.T) {
		numOfMsgs := 5000
		expectedLogStrs := []string{
			validation.GraftInspectionDisabledWarning,
			validation.PruneInspectionDisabledWarning,
			validation.IHaveInspectionDisabledWarning,
			validation.IWantInspectionDisabledWarning,
			validation.PublishInspectionDisabledWarning,
			worker.QueuedItemProcessedLog,
		}
		logCounter := atomic.NewInt64(0)
		logger := hookedLogger(logCounter, zerolog.TraceLevel, expectedLogStrs...)
		inspector, signalerCtx, cancel, consumer, rpcTracker, _, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.GraftPrune.MessageCountThreshold = numOfMsgs
			params.Logger = logger
			// disable inspection for all control message types individually
			params.Config.InspectionProcess.Inspect.EnableGraft = false
			params.Config.InspectionProcess.Inspect.EnablePrune = false
			params.Config.InspectionProcess.Inspect.EnableIHave = false
			params.Config.InspectionProcess.Inspect.EnableIWant = false
			params.Config.InspectionProcess.Inspect.EnablePublish = false
		})

		// notification consumer should never be called when inspection is disabled
		defer consumer.AssertNotCalled(t, "OnInvalidControlMessageNotification")
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		inspector.Start(signalerCtx)

		rpc := unittest.P2PRPCFixture(
			unittest.WithGrafts(unittest.P2PRPCGraftFixtures(unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(numOfMsgs, unittest.IdentifierListFixture(numOfMsgs).Strings()...)...),
			unittest.WithIWants(unittest.P2PRPCIWantFixtures(numOfMsgs, numOfMsgs)...),
			unittest.WithPubsubMessages(unittest.GossipSubMessageFixtures(numOfMsgs, unittest.RandomStringFixture(t, 100), unittest.WithFrom(unittest.PeerIdFixture(t)))...),
		)

		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
		require.NoError(t, inspector.Inspect(from, rpc))

		require.Eventually(t, func() bool {
			return logCounter.Load() == int64(len(expectedLogStrs))
		}, time.Second, 500*time.Millisecond)

		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})
}

// invalidTopics returns 3 invalid topics.
// - unknown topic
// - malformed topic
// - topic with invalid spork ID
func invalidTopics(t *testing.T, sporkID flow.Identifier) (string, string, string) {
	// create unknown topic
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", unittest.IdentifierFixture(), sporkID)).String()
	// create malformed topic
	malformedTopic := channels.Topic(unittest.RandomStringFixture(t, 100)).String()
	// a topics spork ID is considered invalid if it does not match the current spork ID
	invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture())).String()
	return unknownTopic, malformedTopic, invalidSporkIDTopic
}

// checkNotificationFunc returns util func used to ensure invalid control message notification disseminated contains expected information.
func checkNotificationFunc(t *testing.T,
	expectedPeerID peer.ID,
	expectedMsgType p2pmsg.ControlMessageType,
	isExpectedErr func(err error) bool,
	topicType p2p.CtrlMsgTopicType) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, topicType, notification.TopicType)
		require.Equal(t, expectedPeerID, notification.PeerID)
		require.Equal(t, expectedMsgType, notification.MsgType)
		require.True(t, isExpectedErr(notification.Error))
	}
}

func inspectorFixture(t *testing.T, opts ...func(params *validation.InspectorParams)) (*validation.ControlMsgValidationInspector,
	*irrecoverable.MockSignalerContext,
	context.CancelFunc, *mockp2p.GossipSubInvalidControlMessageNotificationConsumer,
	*mockp2p.RpcControlTracking,
	flow.Identifier,
	*mockmodule.IdentityProvider,
	*p2ptest.UpdatableTopicProviderFixture) {

	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	consumer := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
	idProvider := mockmodule.NewIdentityProvider(t)
	rpcTracker := mockp2p.NewRpcControlTracking(t)
	topicProviderOracle := p2ptest.NewUpdatableTopicProviderFixture()
	params := &validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              rpcTracker,
		InvalidControlMessageNotificationConsumer: consumer,
		NetworkingType: network.PrivateNetwork,
		TopicOracle: func() p2p.TopicProvider {
			return topicProviderOracle
		},
	}
	for _, opt := range opts {
		opt(params)
	}
	validationInspector, err := validation.NewControlMsgValidationInspector(params)
	require.NoError(t, err, "failed to create control message validation inspector fixture")
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	return validationInspector, signalerCtx, cancel, consumer, rpcTracker, sporkID, idProvider, topicProviderOracle
}

// utility function to track the number of expected logs for the expected log level.
func hookedLogger(counter *atomic.Int64, expectedLogLevel zerolog.Level, expectedLogs ...string) zerolog.Logger {
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == expectedLogLevel {
			for _, s := range expectedLogs {
				if message == s {
					counter.Inc()
				}
			}
		}
	})
	return zerolog.New(os.Stdout).Level(expectedLogLevel).Hook(hook)
}

// ensureMessageIdsLen ensures RPC IHave and IWant message ids are the expected len.
func ensureMessageIdsLen(t *testing.T, msgType p2pmsg.ControlMessageType, rpc *pubsub.RPC, expectedLen int) {
	switch msgType {
	case p2pmsg.CtrlMsgIHave:
		for _, ihave := range rpc.GetControl().GetIhave() {
			require.Len(t, ihave.GetMessageIDs(), expectedLen)
		}
	case p2pmsg.CtrlMsgIWant:
		for _, iwant := range rpc.GetControl().GetIwant() {
			require.Len(t, iwant.GetMessageIDs(), expectedLen)
		}
	default:
		require.Fail(t, "control message type provided does not contain message ids expected ihave or iwant")
	}
}
