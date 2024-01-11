package validation_test

import (
	"context"
	"fmt"
	"math/rand"
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
	t.Run("truncateGraftMessages should truncate graft messages as expected", func(t *testing.T) {
		graftPruneMessageMaxSampleSize := 1000
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.GraftPrune.MaxSampleSize = graftPruneMessageMaxSampleSize
		})
		// topic validation is ignored set any topic oracle
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		inspector.Start(signalerCtx)
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
		stopInspector(t, cancel, inspector)
	})

	t.Run("truncatePruneMessages should truncate prune messages as expected", func(t *testing.T) {
		graftPruneMessageMaxSampleSize := 1000
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.GraftPrune.MaxSampleSize = graftPruneMessageMaxSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Twice()

		inspector.Start(signalerCtx)
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
		stopInspector(t, cancel, inspector)
	})

	t.Run("truncateIHaveMessages should truncate iHave messages as expected", func(t *testing.T) {
		maxSampleSize := 1000
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IHave.MaxSampleSize = maxSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Twice()
		inspector.Start(signalerCtx)

		// topic validation not performed so we can use random strings
		iHavesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(2000, unittest.IdentifierListFixture(2000).Strings()...)...))
		require.Greater(t, len(iHavesGreaterThanMaxSampleSize.GetControl().GetIhave()), maxSampleSize)
		iHavesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(200, unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(iHavesLessThanMaxSampleSize.GetControl().GetIhave()), maxSampleSize)

		from := unittest.PeerIdFixture(t)
		require.NoError(t, inspector.Inspect(from, iHavesGreaterThanMaxSampleSize))
		require.NoError(t, inspector.Inspect(from, iHavesLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			// rpc with iHaves greater than configured max sample size should be truncated to MaxSampleSize
			shouldBeTruncated := len(iHavesGreaterThanMaxSampleSize.GetControl().GetIhave()) == maxSampleSize
			// rpc with iHaves less than MaxSampleSize should not be truncated
			shouldNotBeTruncated := len(iHavesLessThanMaxSampleSize.GetControl().GetIhave()) == 50
			return shouldBeTruncated && shouldNotBeTruncated
		}, time.Second, 500*time.Millisecond)
		stopInspector(t, cancel, inspector)
	})

	t.Run("truncateIHaveMessageIds should truncate iHave message ids as expected", func(t *testing.T) {
		maxMessageIDSampleSize := 1000
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IHave.MaxMessageIDSampleSize = maxMessageIDSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Twice()
		inspector.Start(signalerCtx)

		// topic validation not performed so we can use random strings
		iHavesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(2000, unittest.IdentifierListFixture(10).Strings()...)...))
		iHavesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(50, unittest.IdentifierListFixture(10).Strings()...)...))

		from := unittest.PeerIdFixture(t)
		require.NoError(t, inspector.Inspect(from, iHavesGreaterThanMaxSampleSize))
		require.NoError(t, inspector.Inspect(from, iHavesLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			for _, iHave := range iHavesGreaterThanMaxSampleSize.GetControl().GetIhave() {
				// rpc with iHaves message ids greater than configured max sample size should be truncated to MaxSampleSize
				if len(iHave.GetMessageIDs()) != maxMessageIDSampleSize {
					return false
				}
			}
			for _, iHave := range iHavesLessThanMaxSampleSize.GetControl().GetIhave() {
				// rpc with iHaves message ids less than MaxSampleSize should not be truncated
				if len(iHave.GetMessageIDs()) != 50 {
					return false
				}
			}
			return true
		}, time.Second, 500*time.Millisecond)
		stopInspector(t, cancel, inspector)
	})

	t.Run("truncateIWantMessages should truncate iWant messages as expected", func(t *testing.T) {
		maxSampleSize := uint(100)
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IWant.MaxSampleSize = maxSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		inspector.Start(signalerCtx)
		iWantsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(200, 200)...))
		require.Greater(t, uint(len(iWantsGreaterThanMaxSampleSize.GetControl().GetIwant())), maxSampleSize)
		iWantsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(50, 200)...))
		require.Less(t, uint(len(iWantsLessThanMaxSampleSize.GetControl().GetIwant())), maxSampleSize)

		from := unittest.PeerIdFixture(t)
		require.NoError(t, inspector.Inspect(from, iWantsGreaterThanMaxSampleSize))
		require.NoError(t, inspector.Inspect(from, iWantsLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			// rpc with iWants greater than configured max sample size should be truncated to MaxSampleSize
			shouldBeTruncated := len(iWantsGreaterThanMaxSampleSize.GetControl().GetIwant()) == int(maxSampleSize)
			// rpc with iWants less than MaxSampleSize should not be truncated
			shouldNotBeTruncated := len(iWantsLessThanMaxSampleSize.GetControl().GetIwant()) == 50
			return shouldBeTruncated && shouldNotBeTruncated
		}, time.Second, 500*time.Millisecond)
		stopInspector(t, cancel, inspector)
	})

	t.Run("truncateIWantMessageIds should truncate iWant message ids as expected", func(t *testing.T) {
		maxMessageIDSampleSize := 1000
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IWant.MaxMessageIDSampleSize = maxMessageIDSampleSize
		})
		// topic validation is ignored set any topic oracle
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		inspector.Start(signalerCtx)
		iWantsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 2000)...))
		iWantsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 50)...))

		from := unittest.PeerIdFixture(t)
		require.NoError(t, inspector.Inspect(from, iWantsGreaterThanMaxSampleSize))
		require.NoError(t, inspector.Inspect(from, iWantsLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			for _, iWant := range iWantsGreaterThanMaxSampleSize.GetControl().GetIwant() {
				// rpc with iWants message ids greater than configured max sample size should be truncated to MaxSampleSize
				if len(iWant.GetMessageIDs()) != maxMessageIDSampleSize {
					return false
				}
			}
			for _, iWant := range iWantsLessThanMaxSampleSize.GetControl().GetIwant() {
				// rpc with iWants less than MaxSampleSize should not be truncated
				if len(iWant.GetMessageIDs()) != 50 {
					return false
				}
			}
			return true
		}, time.Second, 500*time.Millisecond)
		stopInspector(t, cancel, inspector)
	})
}

// TestControlMessageValidationInspector_processInspectRPCReq verifies the correct behavior of control message validation.
// It ensures that valid RPC control messages do not trigger erroneous invalid control message notifications,
// while all types of invalid control messages trigger expected notifications.
func TestControlMessageValidationInspector_processInspectRPCReq(t *testing.T) {
	t.Run("processInspectRPCReq should not disseminate any invalid notification errors for valid RPC's", func(t *testing.T) {
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
		inspector.Start(signalerCtx)
		grafts := unittest.P2PRPCGraftFixtures(topics...)
		prunes := unittest.P2PRPCPruneFixtures(topics...)
		ihaves := unittest.P2PRPCIHaveFixtures(50, topics...)
		iwants := unittest.P2PRPCIWantFixtures(2, 5)
		pubsubMsgs := unittest.GossipSubMessageFixtures(10, topics[0])

		// avoid cache misses for iwant messages.
		iwants[0].MessageIDs = ihaves[0].MessageIDs[:10]
		iwants[1].MessageIDs = ihaves[1].MessageIDs[11:20]
		expectedMsgIds := make([]string, 0)
		expectedMsgIds = append(expectedMsgIds, ihaves[0].MessageIDs...)
		expectedMsgIds = append(expectedMsgIds, ihaves[1].MessageIDs...)
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
			require.Contains(t, expectedMsgIds, id)
		})

		from := unittest.PeerIdFixture(t)
		require.NoError(t, inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})

	// duplicate graft topic ids beyond threshold should trigger invalid control message notification
	t.Run("duplicate graft topic ids beyond threshold", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
		duplicateTopic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
		// avoid unknown topics errors
		topicProviderOracle.UpdateTopics([]string{duplicateTopic})
		var grafts []*pubsub_pb.ControlGraft
		cfg, err := config.DefaultConfig()
		require.NoError(t, err)
		for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.GraftPrune.MaxTotalDuplicateTopicIdThreshold+2; i++ {
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
		stopInspector(t, cancel, inspector)
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
		for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.GraftPrune.MaxTotalDuplicateTopicIdThreshold+2; i++ {
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
		stopInspector(t, cancel, inspector)
	})

	t.Run("inspectGraftMessages should disseminate invalid control message notification for invalid graft messages as expected", func(t *testing.T) {
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

		require.NoError(t, inspector.Inspect(from, unknownTopicReq))
		require.NoError(t, inspector.Inspect(from, malformedTopicReq))
		require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicReq))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})

	t.Run("inspectPruneMessages should disseminate invalid control message notification for invalid prune messages as expected", func(t *testing.T) {
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

		require.NoError(t, inspector.Inspect(from, unknownTopicRpc))
		require.NoError(t, inspector.Inspect(from, malformedTopicRpc))
		require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})

	t.Run("inspectIHaveMessages should disseminate invalid control message notification for iHave messages with invalid topics as expected", func(t *testing.T) {
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

		require.NoError(t, inspector.Inspect(from, unknownTopicRpc))
		require.NoError(t, inspector.Inspect(from, malformedTopicRpc))
		require.NoError(t, inspector.Inspect(from, invalidSporkIDTopicRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})

	t.Run("inspectIHaveMessages should NOT disseminate invalid control message notification for iHave messages when duplicate message ids are below the threshold", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
		validTopic := fmt.Sprintf("%s/%s", channels.PushBlocks.String(), sporkID)
		// avoid unknown topics errors
		topicProviderOracle.UpdateTopics([]string{validTopic})
		duplicateMsgID := unittest.IdentifierFixture()

		cfg, err := config.DefaultConfig()
		require.NoError(t, err)
		msgIds := flow.IdentifierList{}
		// includes as many duplicates as allowed by the threshold
		for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.IHave.MaxTotalDuplicateMessageIdThreshold; i++ {
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
		stopInspector(t, cancel, inspector)
	})

	t.Run("inspectIHaveMessages should disseminate invalid control message notification for iHave messages when duplicate message ids are above the threshold", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, _, sporkID, _, topicProviderOracle := inspectorFixture(t)
		validTopic := fmt.Sprintf("%s/%s", channels.PushBlocks.String(), sporkID)
		// avoid unknown topics errors
		topicProviderOracle.UpdateTopics([]string{validTopic})
		duplicateMsgID := unittest.IdentifierFixture()

		cfg, err := config.DefaultConfig()
		require.NoError(t, err)
		msgIds := flow.IdentifierList{}
		// includes as many duplicates as beyond the threshold
		for i := 0; i < cfg.NetworkConfig.GossipSub.RpcInspector.Validation.IHave.MaxTotalDuplicateMessageIdThreshold+2; i++ {
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
		stopInspector(t, cancel, inspector)
	})

	t.Run("inspectIWantMessages should disseminate invalid control message notification for iWant messages when duplicate message ids exceeds the allowed threshold", func(t *testing.T) {
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t)
		// oracle must be set even though iWant messages do not have topic IDs
		duplicateMsgID := unittest.IdentifierFixture()
		duplicates := flow.IdentifierList{duplicateMsgID, duplicateMsgID}
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
		})

		inspector.Start(signalerCtx)

		require.NoError(t, inspector.Inspect(from, duplicateMsgIDRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})

	t.Run("inspectIWantMessages should disseminate invalid control message notification for iWant messages when cache misses exceeds allowed threshold", func(t *testing.T) {
		cacheMissCheckSize := 1000
		inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.IWant.CacheMissCheckSize = cacheMissCheckSize
			// set high cache miss threshold to ensure we only disseminate notification when it is exceeded
			params.Config.IWant.CacheMissThreshold = .9
		})
		// oracle must be set even though iWant messages do not have topic IDs
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(cacheMissCheckSize+1, 100)...))

		from := unittest.PeerIdFixture(t)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIWant, validation.IsIWantCacheMissThresholdErr, p2p.CtrlMsgNonClusterTopicType)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		// return false each time to eventually force a notification to be disseminated when the cache miss count finally exceeds the 90% threshold
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(false).Run(func(args mock.Arguments) {
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

		require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})

	t.Run("inspectIWantMessages should not disseminate invalid control message notification for iWant messages when cache misses exceeds allowed threshold if cache miss check size not exceeded",
		func(t *testing.T) {
			inspector, signalerCtx, cancel, distributor, rpcTracker, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
				// if size of iwants not greater than 10 cache misses will not be checked
				params.Config.IWant.CacheMissCheckSize = 10
				// set high cache miss threshold to ensure we only disseminate notification when it is exceeded
				params.Config.IWant.CacheMissThreshold = .9
			})
			// oracle must be set even though iWant messages do not have topic IDs
			defer distributor.AssertNotCalled(t, "Distribute")

			msgIds := unittest.IdentifierListFixture(100).Strings()
			inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixture(msgIds...)))
			rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
			// return false each time to eventually force a notification to be disseminated when the cache miss count finally exceeds the 90% threshold
			rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(false).Run(func(args mock.Arguments) {
				id, ok := args[0].(string)
				require.True(t, ok)
				require.Contains(t, msgIds, id)
			})

			from := unittest.PeerIdFixture(t)
			inspector.Start(signalerCtx)

			require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
			// sleep for 1 second to ensure rpc's is processed
			time.Sleep(time.Second)
			stopInspector(t, cancel, inspector)
		})

	t.Run("inspectRpcPublishMessages should disseminate invalid control message notification when invalid pubsub messages count greater than configured RpcMessageErrorThreshold", func(t *testing.T) {
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

		require.NoError(t, inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})
	t.Run("inspectRpcPublishMessages should disseminate invalid control message notification when subscription missing for topic", func(t *testing.T) {
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
		require.NoError(t, inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})

	t.Run("inspectRpcPublishMessages should disseminate invalid control message notification when publish messages contain no topic", func(t *testing.T) {
		errThreshold := 500
		inspector, signalerCtx, cancel, distributor, _, _, _, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			// 5 invalid pubsub messages will force notification dissemination
			params.Config.PublishMessages.ErrorThreshold = errThreshold
		})
		pubsubMsgs := unittest.GossipSubMessageFixtures(errThreshold+1, "")
		rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
		topics := make([]string, len(pubsubMsgs))
		for i, msg := range pubsubMsgs {
			topics[i] = *msg.Topic
		}
		// set topic oracle to return list of topics excluding first topic sent
		from := unittest.PeerIdFixture(t)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr, p2p.CtrlMsgNonClusterTopicType)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		inspector.Start(signalerCtx)
		require.NoError(t, inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})
	t.Run("inspectRpcPublishMessages should not inspect pubsub message sender on public networks", func(t *testing.T) {
		inspector, signalerCtx, cancel, _, _, sporkID, idProvider, topicProviderOracle := inspectorFixture(t)
		from := unittest.PeerIdFixture(t)
		defer idProvider.AssertNotCalled(t, "ByPeerID", from)
		topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
		topicProviderOracle.UpdateTopics([]string{topic})
		pubsubMsgs := unittest.GossipSubMessageFixtures(10, topic, unittest.WithFrom(from))
		rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
		inspector.Start(signalerCtx)
		require.NoError(t, inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})
	t.Run("inspectRpcPublishMessages should disseminate invalid control message notification when message is from unstaked peer", func(t *testing.T) {
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
		require.NoError(t, inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})
	t.Run("inspectRpcPublishMessages should disseminate invalid control message notification when message is from ejected peer", func(t *testing.T) {
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
		require.NoError(t, inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
	})
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
		require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
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
		require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
		stopInspector(t, cancel, inspector)
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
		require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
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
		for i := 0; i < 11; i++ {
			require.NoError(t, inspector.Inspect(from, inspectMsgRpc))
		}
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
		stopInspector(t, cancel, inspector)
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
	from := unittest.PeerIdFixture(t)
	for _, id := range activeClusterIds {
		topic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(id), sporkID)).String()
		rpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&topic)))
		require.NoError(t, inspector.Inspect(from, rpc))
	}
	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
	stopInspector(t, cancel, inspector)
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

func stopInspector(t *testing.T, cancel context.CancelFunc, inspector *validation.ControlMsgValidationInspector) {
	cancel()
	unittest.RequireCloseBefore(t, inspector.Done(), 5*time.Second, "inspector did not stop")
}
