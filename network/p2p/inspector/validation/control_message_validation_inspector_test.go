package validation_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/internal"
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
		distributor := mockp2p.NewGossipSubInspectorNotifDistributor(t)
		idProvider := mockmodule.NewIdentityProvider(t)
		topicProvider := internal.NewMockUpdatableTopicProvider()
		inspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
			Logger:                  unittest.Logger(),
			SporkID:                 sporkID,
			Config:                  &flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation,
			Distributor:             distributor,
			IdProvider:              idProvider,
			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
			InspectorMetrics:        metrics.NewNoopCollector(),
			RpcTracker:              mockp2p.NewRpcControlTracking(t),
			NetworkingType:          network.PublicNetwork,
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
			Distributor:             nil,
			IdProvider:              nil,
			HeroCacheMetricsFactory: nil,
			InspectorMetrics:        nil,
			RpcTracker:              nil,
			TopicOracle:             nil,
		})
		require.Nil(t, inspector)
		require.Error(t, err)
		s := err.Error()
		require.Contains(t, s, "validation for 'Config' failed on the 'required'")
		require.Contains(t, s, "validation for 'Distributor' failed on the 'required'")
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
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.GraftPrune.MessageCountThreshold = graftPruneMessageMaxSampleSize
		})
		// topic validation is ignored set any topic oracle
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
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
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.GraftPrune.MessageCountThreshold = graftPruneMessageMaxSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Twice()

		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		// unittest.RequireCloseBefore(t, inspector.Ready(), 100*time.Millisecond, "inspector did not start")
		// topic validation not performed, so we can use random strings
		prunesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(2000).Strings()...)...))
		require.Greater(t, len(prunesGreaterThanMaxSampleSize.GetControl().GetPrune()), graftPruneMessageMaxSampleSize)
		prunesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(prunesLessThanMaxSampleSize.GetControl().GetPrune()), graftPruneMessageMaxSampleSize)
		from := unittest.PeerIdFixture(t)
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
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IHave.MessageCountThreshold = maxSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Twice()
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		// topic validation not performed so we can use random strings
		iHavesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(2000, unittest.IdentifierListFixture(2000).Strings()...)...))
		require.Greater(t, len(iHavesGreaterThanMaxSampleSize.GetControl().GetIhave()), maxSampleSize)
		iHavesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(200, unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(iHavesLessThanMaxSampleSize.GetControl().GetIhave()), maxSampleSize)

		from := unittest.PeerIdFixture(t)
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
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IHave.MessageIdCountThreshold = maxMessageIDSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Twice()
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		// topic validation not performed so we can use random strings
		iHavesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(2000, unittest.IdentifierListFixture(10).Strings()...)...))
		iHavesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(50, unittest.IdentifierListFixture(10).Strings()...)...))

		from := unittest.PeerIdFixture(t)
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
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IWant.MessageCountThreshold = maxSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		iWantsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(200, 200)...))
		require.Greater(t, uint(len(iWantsGreaterThanMaxSampleSize.GetControl().GetIwant())), maxSampleSize)
		iWantsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(50, 200)...))
		require.Less(t, uint(len(iWantsLessThanMaxSampleSize.GetControl().GetIwant())), maxSampleSize)

		from := unittest.PeerIdFixture(t)
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
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IWant.MessageIdCountThreshold = maxMessageIDSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		iWantsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 2000)...))
		iWantsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 50)...))

		from := unittest.PeerIdFixture(t)
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

// TestControlMessageValidationInspector_processInspectRPCReq verifies the correct behavior of control message validation.
// It ensures that valid RPC control messages do not trigger erroneous invalid control message notifications,
// while all types of invalid control messages trigger expected notifications.
func TestControlMessageValidationInspector_processInspectRPCReq(t *testing.T) {
	// duplicate graft topic ids beyond threshold should trigger invalid control message notification
	t.Run("duplicate graft topic ids beyond threshold", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
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
		rpc := unittest.P2PRPCFixture(unittest.WithGrafts(grafts...))
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(func(args mock.Arguments) {
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, notification.TopicType, p2p.CtrlMsgNonClusterTopicType, "expected p2p.CtrlMsgNonClusterTopicType notification type, no RPC with cluster prefixed topic sent in this test")
			require.Equal(t, from, notification.PeerID)
			require.Equal(t, p2pmsg.CtrlMsgGraft, notification.MsgType)
			require.True(t, validation.IsDuplicateTopicErr(notification.Error))
		})

		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 100*time.Millisecond, inspector)

		require.NoError(t, inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	// duplicate graft topic ids below threshold should NOT trigger invalid control message notification
	t.Run("duplicate graft topic ids below threshold", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
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
		rpc := unittest.P2PRPCFixture(unittest.WithGrafts(grafts...))
		// no notification should be disseminated for valid messages as long as the number of duplicates is below the threshold
		distributor.AssertNotCalled(t, "Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif"))

		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 100*time.Millisecond, inspector)

		require.NoError(t, inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	// duplicate prune topic ids beyond threshold should trigger invalid control message notification
	t.Run("duplicate prune topic ids beyond threshold", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
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
		rpc := unittest.P2PRPCFixture(unittest.WithPrunes(prunes...))
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(func(args mock.Arguments) {
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, notification.TopicType, p2p.CtrlMsgNonClusterTopicType, "expected p2p.CtrlMsgNonClusterTopicType notification type, no RPC with cluster prefixed topic sent in this test")
			require.Equal(t, from, notification.PeerID)
			require.Equal(t, p2pmsg.CtrlMsgPrune, notification.MsgType)
			require.True(t, validation.IsDuplicateTopicErr(notification.Error))
		})

		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 100*time.Millisecond, inspector)

		require.NoError(t, inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	// duplicate prune topic ids below threshold should NOT trigger invalid control message notification
	t.Run("duplicate prune topic ids below threshold", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
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
		rpc := unittest.P2PRPCFixture(unittest.WithPrunes(prunes...))
		// no notification should be disseminated for valid messages as long as the number of duplicates is below the threshold
		distributor.AssertNotCalled(t, "Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif"))

		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 100*time.Millisecond, inspector)

		require.NoError(t, inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	// invalid graft topic ids should trigger invalid control message notification
	t.Run("invalid graft topic", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		// avoid unknown topics errors
		topicProviderOracle.UpdateTopics([]string{unknownTopic, malformedTopic, invalidSporkIDTopic})
		unknownTopicGraft := unittest.P2PRPCGraftFixture(&unknownTopic)
		malformedTopicGraft := unittest.P2PRPCGraftFixture(&malformedTopic)
		invalidSporkIDTopicGraft := unittest.P2PRPCGraftFixture(&invalidSporkIDTopic)

		unknownTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(unknownTopicGraft))
		malformedTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(malformedTopicGraft))
		invalidSporkIDTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(invalidSporkIDTopicGraft))

		from := unittest.PeerIdFixture(t)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgGraft, channels.IsInvalidTopicErr, p2p.CtrlMsgNonClusterTopicType)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)

		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		require.NoError(t, inspector.Inspect(from, unknownTopicReq))
		require.NoError(t, inspector.Inspect(from, malformedTopicReq))
		require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicReq))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	// invalid prune topic ids should trigger invalid control message notification
	t.Run("invalid prune topic", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		unknownTopicPrune := unittest.P2PRPCPruneFixture(&unknownTopic)
		malformedTopicPrune := unittest.P2PRPCPruneFixture(&malformedTopic)
		invalidSporkIDTopicPrune := unittest.P2PRPCPruneFixture(&invalidSporkIDTopic)
		// avoid unknown topics errors
		topicProviderOracle.UpdateTopics([]string{unknownTopic, malformedTopic, invalidSporkIDTopic})
		unknownTopicRpc := unittest.P2PRPCFixture(unittest.WithPrunes(unknownTopicPrune))
		malformedTopicRpc := unittest.P2PRPCFixture(unittest.WithPrunes(malformedTopicPrune))
		invalidSporkIDTopicRpc := unittest.P2PRPCFixture(unittest.WithPrunes(invalidSporkIDTopicPrune))

		from := unittest.PeerIdFixture(t)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgPrune, channels.IsInvalidTopicErr, p2p.CtrlMsgNonClusterTopicType)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)

		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		require.NoError(t, inspector.Inspect(from, unknownTopicRpc))
		require.NoError(t, inspector.Inspect(from, malformedTopicRpc))
		require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	// invalid ihave topic ids should trigger invalid control message notification
	t.Run("invalid ihave topic", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		// avoid unknown topics errors
		topicProviderOracle.UpdateTopics([]string{unknownTopic, malformedTopic, invalidSporkIDTopic})
		unknownTopicIhave := unittest.P2PRPCIHaveFixture(&unknownTopic, unittest.IdentifierListFixture(5).Strings()...)
		malformedTopicIhave := unittest.P2PRPCIHaveFixture(&malformedTopic, unittest.IdentifierListFixture(5).Strings()...)
		invalidSporkIDTopicIhave := unittest.P2PRPCIHaveFixture(&invalidSporkIDTopic, unittest.IdentifierListFixture(5).Strings()...)

		unknownTopicRpc := unittest.P2PRPCFixture(unittest.WithIHaves(unknownTopicIhave))
		malformedTopicRpc := unittest.P2PRPCFixture(unittest.WithIHaves(malformedTopicIhave))
		invalidSporkIDTopicRpc := unittest.P2PRPCFixture(unittest.WithIHaves(invalidSporkIDTopicIhave))

		from := unittest.PeerIdFixture(t)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIHave, channels.IsInvalidTopicErr, p2p.CtrlMsgNonClusterTopicType)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		require.NoError(t, inspector.Inspect(from, unknownTopicRpc))
		require.NoError(t, inspector.Inspect(from, malformedTopicRpc))
		require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	// ihave duplicate topic ids below threshold should NOT trigger invalid control message notification.
	t.Run("ihave duplicate message ids below threshold", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
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

		// no notification should be disseminated for valid messages as long as the number of duplicates is below the threshold
		distributor.AssertNotCalled(t, "Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif"))
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		require.NoError(t, inspector.Inspect(from, duplicateMsgIDRpc))
		// TODO: this sleeps should be replaced with a queue size checker.
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	// ihave duplicate topic ids beyond threshold should trigger invalid control message notification.
	t.Run("ihave duplicate message ids above threshold", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
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

		// one notification should be disseminated for invalid messages when the number of duplicates exceeds the threshold
		checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIHave, validation.IsDuplicateMessageIDErr, p2p.CtrlMsgNonClusterTopicType)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		require.NoError(t, inspector.Inspect(from, duplicateMsgIDRpc))
		// TODO: this sleeps should be replaced with a queue size checker.
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})
}

// TestControlMessageInspection_ValidRpc ensures inspector does not disseminate invalid control message notifications for a valid RPC.
func TestControlMessageInspection_ValidRpc(t *testing.T) {
	inspector, signalerCtx, cancel, distributor, rpcTracker, sporkID, _, topicProviderOracle := inspectorFixture(t)
	defer distributor.AssertNotCalled(t, "Distribute")

	topics := []string{
		fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID),
		fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID),
		fmt.Sprintf("%s/%s", channels.SyncCommittee, sporkID),
		fmt.Sprintf("%s/%s", channels.RequestChunks, sporkID),
	}
	// avoid unknown topics errors
	topicProviderOracle.UpdateTopics(topics)
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	grafts := unittest.P2PRPCGraftFixtures(topics...)
	prunes := unittest.P2PRPCPruneFixtures(topics...)
	ihaves := unittest.P2PRPCIHaveFixtures(50, topics...)
	// makes sure each iwant has enough message ids to trigger cache misses check
	iwants := unittest.P2PRPCIWantFixtures(2, int(cfg.NetworkConfig.GossipSub.RpcInspector.Validation.IWant.CacheMissCheckSize)+1)
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
	require.NoError(t, inspector.Inspect(from, rpc))
	// sleep for 1 second to ensure rpc is processed
	time.Sleep(time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestIWantInspection_DuplicateMessageIds_AboveThreshold ensures inspector disseminates invalid control message notifications for iWant messages when duplicate message ids exceeds allowed threshold.
func TestIWantInspection_DuplicateMessageIds_AboveThreshold(t *testing.T) {
	inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t)
	// oracle must be set even though iWant messages do not have topic IDs
	duplicateMsgID := unittest.IdentifierFixture()
	duplicates := flow.IdentifierList{}
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// includes as many duplicates as allowed by the threshold
	for i := 0; i < int(cfg.NetworkConfig.GossipSub.RpcInspector.Validation.IWant.DuplicateMsgIDThreshold)+2; i++ {
		duplicates = append(duplicates, duplicateMsgID)
	}
	msgIds := append(duplicates, unittest.IdentifierListFixture(5)...).Strings()
	duplicateMsgIDIWant := unittest.P2PRPCIWantFixture(msgIds...)

	duplicateMsgIDRpc := unittest.P2PRPCFixture(unittest.WithIWants(duplicateMsgIDIWant))

	from := unittest.PeerIdFixture(t)
	checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIWant, validation.IsIWantDuplicateMsgIDThresholdErr, p2p.CtrlMsgNonClusterTopicType)
	distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
	rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Run(func(args mock.Arguments) {
		id, ok := args[0].(string)
		require.True(t, ok)
		require.Contains(t, msgIds, id)
	}).Maybe() // if iwant message ids count are not bigger than cache miss check size, this method is not called, anyway in this test we do not care about this method.

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, duplicateMsgIDRpc))
	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestIWantInspection_CacheMiss_AboveThreshold ensures inspector disseminates invalid control message notifications for iWant messages when cache misses exceeds allowed threshold.
func TestIWantInspection_CacheMiss_AboveThreshold(t *testing.T) {
	inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Config.IWant.CacheMissCheckSize = 99 // when size of an iwant message is above this threshold, it is checked for cache misses
		// set high cache miss threshold to ensure we only disseminate notification when it is exceeded
		params.Config.IWant.CacheMissThreshold = 900
	})
	// 10 iwant messages, each with 100 message ids; total of 1000 message ids, which when imitated as cache misses should trigger notification dissemination.
	inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 100)...))

	from := unittest.PeerIdFixture(t)
	checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIWant, validation.IsIWantCacheMissThresholdErr, p2p.CtrlMsgNonClusterTopicType)
	distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
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

	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

func TestIWantInspection_CacheMiss_BelowThreshold(t *testing.T) {
	inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
		// if size of iwants is below cache miss check size, it is not checked for cache misses
		params.Config.IWant.CacheMissCheckSize = 90
		// set high cache miss threshold to ensure that we do not disseminate notification in this test
		params.Config.IWant.CacheMissThreshold = 99
	})
	// oracle must be set even though iWant messages do not have topic IDs
	defer distributor.AssertNotCalled(t, "Distribute")

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
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
	unittest.RequireReturnsBefore(t, allIwantsChecked.Wait, 1*time.Second, "all iwant messages should be checked for cache misses")

	// waits one more second to ensure no notification is disseminated
	time.Sleep(1 * time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestControlMessageInspection_ExceedingErrThreshold ensures inspector disseminates invalid control message notifications for RPCs that exceed the configured error threshold.
func TestPublishMessageInspection_ExceedingErrThreshold(t *testing.T) {
	errThreshold := 500
	inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Config.PublishMessages.ErrorThreshold = errThreshold
	})
	// create unknown topic
	unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", unittest.IdentifierFixture(), sporkID)).String()
	// create malformed topic
	malformedTopic := channels.Topic("!@#$%^&**((").String()
	// a topics spork ID is considered invalid if it does not match the current spork ID
	invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture())).String()
	// create 10 normal messages
	pubsubMsgs := unittest.GossipSubMessageFixtures(50, fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID))
	// add 550 invalid messages to force notification dissemination
	invalidMessageFixtures := []*pubsub_pb.Message{
		{Topic: &unknownTopic},
		{Topic: &malformedTopic},
		{Topic: &invalidSporkIDTopic},
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
	checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr, p2p.CtrlMsgNonClusterTopicType)
	distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)

	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestControlMessageInspection_MissingSubscription ensures inspector disseminates invalid control message notifications for RPCs that the peer is not subscribed to.
func TestPublishMessageInspection_MissingSubscription(t *testing.T) {
	errThreshold := 500
	inspector, signalerCtx, cancel, distributor, _, sporkID, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
		params.Config.PublishMessages.ErrorThreshold = errThreshold
	})
	pubsubMsgs := unittest.GossipSubMessageFixtures(errThreshold+1, fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID))
	from := unittest.PeerIdFixture(t)
	rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
	checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr, p2p.CtrlMsgNonClusterTopicType)
	distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestPublishMessageInspection_MissingTopic ensures inspector disseminates invalid control message notifications for published messages with missing topics.
func TestPublishMessageInspection_MissingTopic(t *testing.T) {
	errThreshold := 500
	inspector, signalerCtx, cancel, distributor, _, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
		// 5 invalid pubsub messages will force notification dissemination
		params.Config.PublishMessages.ErrorThreshold = errThreshold
	})
	pubsubMsgs := unittest.GossipSubMessageFixtures(errThreshold+1, "")
	rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
	for _, msg := range pubsubMsgs {
		msg.Topic = nil
	}
	from := unittest.PeerIdFixture(t)
	checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr, p2p.CtrlMsgNonClusterTopicType)
	distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestRpcInspectionDeactivatedOnPublicNetwork ensures inspector does not inspect RPCs on public networks.
func TestRpcInspectionDeactivatedOnPublicNetwork(t *testing.T) {
	inspector, signalerCtx, cancel, _, _, sporkID, idProvider, topicProviderOracle := inspectorFixture(t)
	from := unittest.PeerIdFixture(t)
	defer idProvider.AssertNotCalled(t, "ByPeerID", from)
	topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
	topicProviderOracle.UpdateTopics([]string{topic})
	pubsubMsgs := unittest.GossipSubMessageFixtures(10, topic, unittest.WithFrom(from))
	rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestControlMessageInspection_Unstaked_From ensures inspector disseminates invalid control message notifications for published messages from unstaked peers.
func TestPublishMessageInspection_Unstaked_From(t *testing.T) {
	inspector, signalerCtx, cancel, distributor, _, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		// override the inspector and params, run the inspector in private mode
		params.NetworkingType = network.PrivateNetwork
	})
	from := unittest.PeerIdFixture(t)
	topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
	topicProviderOracle.UpdateTopics([]string{topic})
	// default RpcMessageErrorThreshold is 500, 501 messages should trigger a notification
	pubsubMsgs := unittest.GossipSubMessageFixtures(501, topic, unittest.WithFrom(from))
	idProvider.On("ByPeerID", from).Return(nil, false).Times(501)
	rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
	checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr, p2p.CtrlMsgNonClusterTopicType)
	distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestControlMessageInspection_Ejected_From ensures inspector disseminates invalid control message notifications for published messages from ejected peers.
func TestPublishMessageInspection_Ejected_From(t *testing.T) {
	inspector, signalerCtx, cancel, distributor, _, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
		// override the inspector and params, run the inspector in private mode
		params.NetworkingType = network.PrivateNetwork
	})
	from := unittest.PeerIdFixture(t)
	id := unittest.IdentityFixture()
	id.Ejected = true
	topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
	topicProviderOracle.UpdateTopics([]string{topic})
	pubsubMsgs := unittest.GossipSubMessageFixtures(501, topic, unittest.WithFrom(from))
	idProvider.On("ByPeerID", from).Return(id, true).Times(501)
	rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
	checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr, p2p.CtrlMsgNonClusterTopicType)
	distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	require.NoError(t, inspector.Inspect(from, rpc))
	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}

// TestNewControlMsgValidationInspector_validateClusterPrefixedTopic ensures cluster prefixed topics are validated as expected.
func TestNewControlMsgValidationInspector_validateClusterPrefixedTopic(t *testing.T) {
	t.Run("validateClusterPrefixedTopic should not return an error for valid cluster prefixed topics", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, idProvider, topicProviderOracle := inspectorFixture(t)
		defer distributor.AssertNotCalled(t, "Distribute")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), sporkID)).String()
		topicProviderOracle.UpdateTopics([]string{clusterPrefixedTopic})
		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic)))
		inspector.ActiveClustersChanged(flow.ChainIDList{clusterID, flow.ChainID(unittest.IdentifierFixture().String()), flow.ChainID(unittest.IdentifierFixture().String())})
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("validateClusterPrefixedTopic should not return error if cluster prefixed hard threshold not exceeded for unknown cluster ids", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, idProvider, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			// set hard threshold to small number , ensure that a single unknown cluster prefix id does not cause a notification to be disseminated
			params.Config.ClusterPrefixedMessage.HardThreshold = 2
		})
		defer distributor.AssertNotCalled(t, "Distribute")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), sporkID)).String()
		from := unittest.PeerIdFixture(t)
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic)))
		id := unittest.IdentityFixture()
		idProvider.On("ByPeerID", from).Return(id, true).Once()
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("validateClusterPrefixedTopic should return an error when sender is unstaked", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, idProvider, topicProviderOracle := inspectorFixture(t)
		defer distributor.AssertNotCalled(t, "Distribute")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), sporkID)).String()
		topicProviderOracle.UpdateTopics([]string{clusterPrefixedTopic})
		from := unittest.PeerIdFixture(t)
		idProvider.On("ByPeerID", from).Return(nil, false).Once()
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic)))
		inspector.ActiveClustersChanged(flow.ChainIDList{flow.ChainID(unittest.IdentifierFixture().String())})
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})

	t.Run("validateClusterPrefixedTopic should return error if cluster prefixed hard threshold exceeded for unknown cluster ids", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, idProvider, topicProviderOracle := inspectorFixture(t, func(params *validation.InspectorParams) {
			// the 11th unknown cluster ID error should cause an error
			params.Config.ClusterPrefixedMessage.HardThreshold = 10
		})
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), sporkID)).String()
		topicProviderOracle.UpdateTopics([]string{clusterPrefixedTopic})
		from := unittest.PeerIdFixture(t)
		identity := unittest.IdentityFixture()
		idProvider.On("ByPeerID", from).Return(identity, true).Times(11)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgGraft, channels.IsUnknownClusterIDErr, p2p.CtrlMsgTopicTypeClusterPrefixed)
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic)))
		inspector.ActiveClustersChanged(flow.ChainIDList{flow.ChainID(unittest.IdentifierFixture().String())})
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		inspector.Start(signalerCtx)
		unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

		for i := 0; i < 11; i++ {
			require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		}
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		cancel()
		unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
	})
}

// TestControlMessageValidationInspector_ActiveClustersChanged validates the expected update of the active cluster IDs list.
func TestControlMessageValidationInspector_ActiveClustersChanged(t *testing.T) {
	inspector, signalerCtx, cancel, distributor, _, sporkID, idProvider, _ := inspectorFixture(t)
	defer distributor.AssertNotCalled(t, "Distribute")
	identity := unittest.IdentityFixture()
	idProvider.On("ByPeerID", mock.AnythingOfType("peer.ID")).Return(identity, true).Times(5)
	activeClusterIds := make(flow.ChainIDList, 0)
	for _, id := range unittest.IdentifierListFixture(5) {
		activeClusterIds = append(activeClusterIds, flow.ChainID(id.String()))
	}
	inspector.ActiveClustersChanged(activeClusterIds)
	inspector.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 1*time.Second, inspector)

	from := unittest.PeerIdFixture(t)
	for _, id := range activeClusterIds {
		topic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(id), sporkID)).String()
		rpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&topic)))
		require.NoError(t, inspector.Inspect(from, rpc))
	}
	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
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
	context.CancelFunc,
	*mockp2p.GossipSubInspectorNotificationDistributor,
	*mockp2p.RpcControlTracking,
	flow.Identifier,
	*mockmodule.IdentityProvider,
	*internal.MockUpdatableTopicProvider) {
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(t)
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)
	idProvider := mockmodule.NewIdentityProvider(t)
	rpcTracker := mockp2p.NewRpcControlTracking(t)
	topicProviderOracle := internal.NewMockUpdatableTopicProvider()
	params := &validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &flowConfig.NetworkConfig.GossipSub.RpcInspector.Validation,
		Distributor:             distributor,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              rpcTracker,
		NetworkingType:          network.PublicNetwork,
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
	return validationInspector, signalerCtx, cancel, distributor, rpcTracker, sporkID, idProvider, topicProviderOracle
}
