package validation_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

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
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

type ControlMsgValidationInspectorSuite struct {
	suite.Suite
	sporkID             flow.Identifier
	config              *p2pconf.GossipSubRPCValidationInspectorConfigs
	distributor         *mockp2p.GossipSubInspectorNotificationDistributor
	params              *validation.InspectorParams
	rpcTracker          *mockp2p.RpcControlTracking
	idProvider          *mockmodule.IdentityProvider
	inspector           *validation.ControlMsgValidationInspector
	signalerCtx         *irrecoverable.MockSignalerContext
	topicProviderOracle *internal.MockUpdatableTopicProvider
	cancel              context.CancelFunc
}

func TestControlMsgValidationInspector(t *testing.T) {
	suite.Run(t, new(ControlMsgValidationInspectorSuite))
}

func (suite *ControlMsgValidationInspectorSuite) SetupTest() {
	suite.sporkID = unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(suite.T(), err, "failed to get default flow config")
	suite.config = &flowConfig.NetworkConfig.GossipSubRPCValidationInspectorConfigs
	distributor := mockp2p.NewGossipSubInspectorNotificationDistributor(suite.T())
	p2ptest.MockInspectorNotificationDistributorReadyDoneAware(distributor)
	suite.distributor = distributor
	suite.idProvider = mockmodule.NewIdentityProvider(suite.T())
	rpcTracker := mockp2p.NewRpcControlTracking(suite.T())
	suite.rpcTracker = rpcTracker
	suite.topicProviderOracle = internal.NewMockUpdatableTopicProvider()
	params := &validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 suite.sporkID,
		Config:                  &flowConfig.NetworkConfig.GossipSubRPCValidationInspectorConfigs,
		Distributor:             distributor,
		IdProvider:              suite.idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              rpcTracker,
		NetworkingType:          network.PublicNetwork,
		TopicOracle: func() p2p.TopicProvider {
			return suite.topicProviderOracle
		},
	}
	suite.params = params
	inspector, err := validation.NewControlMsgValidationInspector(params)
	require.NoError(suite.T(), err, "failed to create control message validation inspector fixture")
	suite.inspector = inspector
	ctx, cancel := context.WithCancel(context.Background())
	suite.cancel = cancel
	suite.signalerCtx = irrecoverable.NewMockSignalerContext(suite.T(), ctx)
}

func (suite *ControlMsgValidationInspectorSuite) StopInspector() {
	suite.cancel()
	unittest.RequireCloseBefore(suite.T(), suite.inspector.Done(), 500*time.Millisecond, "inspector did not stop")
}

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
			Config:                  &flowConfig.NetworkConfig.GossipSubRPCValidationInspectorConfigs,
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
func (suite *ControlMsgValidationInspectorSuite) TestControlMessageValidationInspector_truncateRPC() {
	suite.T().Run("truncateGraftMessages should truncate graft messages as expected", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		// topic validation is ignored set any topic oracle
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		suite.rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		suite.rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		suite.config.GraftPruneMessageMaxSampleSize = 100
		suite.inspector.Start(suite.signalerCtx)

		// topic validation not performed so we can use random strings
		graftsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixtures(unittest.IdentifierListFixture(200).Strings()...)...))
		require.Greater(t, len(graftsGreaterThanMaxSampleSize.GetControl().GetGraft()), suite.config.GraftPruneMessageMaxSampleSize)
		graftsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixtures(unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(graftsLessThanMaxSampleSize.GetControl().GetGraft()), suite.config.GraftPruneMessageMaxSampleSize)

		from := unittest.PeerIdFixture(t)
		require.NoError(t, suite.inspector.Inspect(from, graftsGreaterThanMaxSampleSize))
		require.NoError(t, suite.inspector.Inspect(from, graftsLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			// rpc with grafts greater than configured max sample size should be truncated to GraftPruneMessageMaxSampleSize
			shouldBeTruncated := len(graftsGreaterThanMaxSampleSize.GetControl().GetGraft()) == suite.config.GraftPruneMessageMaxSampleSize
			// rpc with grafts less than GraftPruneMessageMaxSampleSize should not be truncated
			shouldNotBeTruncated := len(graftsLessThanMaxSampleSize.GetControl().GetGraft()) == 50
			return shouldBeTruncated && shouldNotBeTruncated
		}, time.Second, 500*time.Millisecond)
	})

	suite.T().Run("truncatePruneMessages should truncate prune messages as expected", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		suite.rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		suite.rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Twice()
		suite.config.GraftPruneMessageMaxSampleSize = 100

		suite.inspector.Start(suite.signalerCtx)
		// unittest.RequireCloseBefore(t, inspector.Ready(), 100*time.Millisecond, "inspector did not start")
		// topic validation not performed, so we can use random strings
		prunesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(200).Strings()...)...))
		require.Greater(t, len(prunesGreaterThanMaxSampleSize.GetControl().GetPrune()), suite.config.GraftPruneMessageMaxSampleSize)
		prunesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(prunesLessThanMaxSampleSize.GetControl().GetPrune()), suite.config.GraftPruneMessageMaxSampleSize)
		from := unittest.PeerIdFixture(t)
		require.NoError(t, suite.inspector.Inspect(from, prunesGreaterThanMaxSampleSize))
		require.NoError(t, suite.inspector.Inspect(from, prunesLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			// rpc with prunes greater than configured max sample size should be truncated to GraftPruneMessageMaxSampleSize
			shouldBeTruncated := len(prunesGreaterThanMaxSampleSize.GetControl().GetPrune()) == suite.config.GraftPruneMessageMaxSampleSize
			// rpc with prunes less than GraftPruneMessageMaxSampleSize should not be truncated
			shouldNotBeTruncated := len(prunesLessThanMaxSampleSize.GetControl().GetPrune()) == 50
			return shouldBeTruncated && shouldNotBeTruncated
		}, time.Second, 500*time.Millisecond)
	})

	suite.T().Run("truncateIHaveMessages should truncate iHave messages as expected", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		suite.rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		suite.rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Twice()
		suite.config.IHaveRPCInspectionConfig.MaxSampleSize = 100
		suite.inspector.Start(suite.signalerCtx)

		// topic validation not performed so we can use random strings
		iHavesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(200,
			unittest.IdentifierListFixture(200).Strings()...)...))
		require.Greater(t, len(iHavesGreaterThanMaxSampleSize.GetControl().GetIhave()), suite.config.IHaveRPCInspectionConfig.MaxSampleSize)
		iHavesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(200, unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(iHavesLessThanMaxSampleSize.GetControl().GetIhave()), suite.config.IHaveRPCInspectionConfig.MaxSampleSize)

		from := unittest.PeerIdFixture(t)
		require.NoError(t, suite.inspector.Inspect(from, iHavesGreaterThanMaxSampleSize))
		require.NoError(t, suite.inspector.Inspect(from, iHavesLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			// rpc with iHaves greater than configured max sample size should be truncated to MaxSampleSize
			shouldBeTruncated := len(iHavesGreaterThanMaxSampleSize.GetControl().GetIhave()) == suite.config.IHaveRPCInspectionConfig.MaxSampleSize
			// rpc with iHaves less than MaxSampleSize should not be truncated
			shouldNotBeTruncated := len(iHavesLessThanMaxSampleSize.GetControl().GetIhave()) == 50
			return shouldBeTruncated && shouldNotBeTruncated
		}, time.Second, 500*time.Millisecond)
	})

	suite.T().Run("truncateIHaveMessageIds should truncate iHave message ids as expected", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		suite.rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		suite.rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Twice()
		suite.config.IHaveRPCInspectionConfig.MaxMessageIDSampleSize = 100
		suite.inspector.Start(suite.signalerCtx)

		// topic validation not performed so we can use random strings
		iHavesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(200,
			unittest.IdentifierListFixture(10).Strings()...)...))
		iHavesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(50, unittest.IdentifierListFixture(10).Strings()...)...))

		from := unittest.PeerIdFixture(t)
		require.NoError(t, suite.inspector.Inspect(from, iHavesGreaterThanMaxSampleSize))
		require.NoError(t, suite.inspector.Inspect(from, iHavesLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			for _, iHave := range iHavesGreaterThanMaxSampleSize.GetControl().GetIhave() {
				// rpc with iHaves message ids greater than configured max sample size should be truncated to MaxSampleSize
				if len(iHave.GetMessageIDs()) != suite.config.IHaveRPCInspectionConfig.MaxMessageIDSampleSize {
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
	})

	suite.T().Run("truncateIWantMessages should truncate iWant messages as expected", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		suite.rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		suite.rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		suite.config.IWantRPCInspectionConfig.MaxSampleSize = 100
		suite.inspector.Start(suite.signalerCtx)

		iWantsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(200, 200)...))
		require.Greater(t, uint(len(iWantsGreaterThanMaxSampleSize.GetControl().GetIwant())), suite.config.IWantRPCInspectionConfig.MaxSampleSize)
		iWantsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(50, 200)...))
		require.Less(t, uint(len(iWantsLessThanMaxSampleSize.GetControl().GetIwant())), suite.config.IWantRPCInspectionConfig.MaxSampleSize)

		from := unittest.PeerIdFixture(t)
		require.NoError(t, suite.inspector.Inspect(from, iWantsGreaterThanMaxSampleSize))
		require.NoError(t, suite.inspector.Inspect(from, iWantsLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			// rpc with iWants greater than configured max sample size should be truncated to MaxSampleSize
			shouldBeTruncated := len(iWantsGreaterThanMaxSampleSize.GetControl().GetIwant()) == int(suite.config.IWantRPCInspectionConfig.MaxSampleSize)
			// rpc with iWants less than MaxSampleSize should not be truncated
			shouldNotBeTruncated := len(iWantsLessThanMaxSampleSize.GetControl().GetIwant()) == 50
			return shouldBeTruncated && shouldNotBeTruncated
		}, time.Second, 500*time.Millisecond)
	})

	suite.T().Run("truncateIWantMessageIds should truncate iWant message ids as expected", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		suite.rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		suite.rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Maybe()
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Maybe()
		suite.config.IWantRPCInspectionConfig.MaxMessageIDSampleSize = 100
		suite.inspector.Start(suite.signalerCtx)

		iWantsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 200)...))
		iWantsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 50)...))

		from := unittest.PeerIdFixture(t)
		require.NoError(t, suite.inspector.Inspect(from, iWantsGreaterThanMaxSampleSize))
		require.NoError(t, suite.inspector.Inspect(from, iWantsLessThanMaxSampleSize))
		require.Eventually(t, func() bool {
			for _, iWant := range iWantsGreaterThanMaxSampleSize.GetControl().GetIwant() {
				// rpc with iWants message ids greater than configured max sample size should be truncated to MaxSampleSize
				if len(iWant.GetMessageIDs()) != suite.config.IWantRPCInspectionConfig.MaxMessageIDSampleSize {
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
	})
}

// TestControlMessageValidationInspector_processInspectRPCReq verifies the correct behavior of control message validation.
// It ensures that valid RPC control messages do not trigger erroneous invalid control message notifications,
// while all types of invalid control messages trigger expected notifications.
func (suite *ControlMsgValidationInspectorSuite) TestControlMessageValidationInspector_processInspectRPCReq() {
	suite.T().Run("processInspectRPCReq should not disseminate any invalid notification errors for valid RPC's", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		defer suite.distributor.AssertNotCalled(t, "Distribute")

		topics := []string{
			fmt.Sprintf("%s/%s", channels.TestNetworkChannel, suite.sporkID),
			fmt.Sprintf("%s/%s", channels.PushBlocks, suite.sporkID),
			fmt.Sprintf("%s/%s", channels.SyncCommittee, suite.sporkID),
			fmt.Sprintf("%s/%s", channels.RequestChunks, suite.sporkID),
		}
		suite.topicProviderOracle.UpdateTopics(topics)
		suite.inspector.Start(suite.signalerCtx)
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
		suite.rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		suite.rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Run(func(args mock.Arguments) {
			id, ok := args[0].(string)
			require.True(t, ok)
			require.Contains(t, expectedMsgIds, id)
		})

		from := unittest.PeerIdFixture(t)
		require.NoError(t, suite.inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc is processed
		time.Sleep(time.Second)
	})

	suite.T().Run("processInspectRPCReq should disseminate invalid control message notification for control messages with duplicate topics", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		duplicateTopic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, suite.sporkID)
		// avoid unknown topics errors
		suite.topicProviderOracle.UpdateTopics([]string{duplicateTopic})
		// create control messages with duplicate topic
		grafts := []*pubsub_pb.ControlGraft{unittest.P2PRPCGraftFixture(&duplicateTopic), unittest.P2PRPCGraftFixture(&duplicateTopic)}
		prunes := []*pubsub_pb.ControlPrune{unittest.P2PRPCPruneFixture(&duplicateTopic), unittest.P2PRPCPruneFixture(&duplicateTopic)}
		ihaves := []*pubsub_pb.ControlIHave{unittest.P2PRPCIHaveFixture(&duplicateTopic, unittest.IdentifierListFixture(20).Strings()...),
			unittest.P2PRPCIHaveFixture(&duplicateTopic, unittest.IdentifierListFixture(20).Strings()...)}
		from := unittest.PeerIdFixture(t)
		duplicateTopicGraftsRpc := unittest.P2PRPCFixture(unittest.WithGrafts(grafts...))
		duplicateTopicPrunesRpc := unittest.P2PRPCFixture(unittest.WithPrunes(prunes...))
		duplicateTopicIHavesRpc := unittest.P2PRPCFixture(unittest.WithIHaves(ihaves...))
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(func(args mock.Arguments) {
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, from, notification.PeerID)
			require.Contains(t, []p2pmsg.ControlMessageType{p2pmsg.CtrlMsgGraft, p2pmsg.CtrlMsgPrune, p2pmsg.CtrlMsgIHave}, notification.MsgType)
			require.True(t, validation.IsDuplicateTopicErr(notification.Error))
		})

		suite.inspector.Start(suite.signalerCtx)

		require.NoError(t, suite.inspector.Inspect(from, duplicateTopicGraftsRpc))
		require.NoError(t, suite.inspector.Inspect(from, duplicateTopicPrunesRpc))
		require.NoError(t, suite.inspector.Inspect(from, duplicateTopicIHavesRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})

	suite.T().Run("inspectGraftMessages should disseminate invalid control message notification for invalid graft messages as expected", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, suite.sporkID)
		// avoid unknown topics errors
		suite.topicProviderOracle.UpdateTopics([]string{unknownTopic, malformedTopic, invalidSporkIDTopic})
		unknownTopicGraft := unittest.P2PRPCGraftFixture(&unknownTopic)
		malformedTopicGraft := unittest.P2PRPCGraftFixture(&malformedTopic)
		invalidSporkIDTopicGraft := unittest.P2PRPCGraftFixture(&invalidSporkIDTopic)

		unknownTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(unknownTopicGraft))
		malformedTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(malformedTopicGraft))
		invalidSporkIDTopicReq := unittest.P2PRPCFixture(unittest.WithGrafts(invalidSporkIDTopicGraft))

		from := unittest.PeerIdFixture(t)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgGraft, channels.IsInvalidTopicErr)
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)

		suite.inspector.Start(suite.signalerCtx)

		require.NoError(t, suite.inspector.Inspect(from, unknownTopicReq))
		require.NoError(t, suite.inspector.Inspect(from, malformedTopicReq))
		require.NoError(t, suite.inspector.Inspect(from, invalidSporkIDTopicReq))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})

	suite.T().Run("inspectPruneMessages should disseminate invalid control message notification for invalid prune messages as expected", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, suite.sporkID)
		unknownTopicPrune := unittest.P2PRPCPruneFixture(&unknownTopic)
		malformedTopicPrune := unittest.P2PRPCPruneFixture(&malformedTopic)
		invalidSporkIDTopicPrune := unittest.P2PRPCPruneFixture(&invalidSporkIDTopic)
		// avoid unknown topics errors
		suite.topicProviderOracle.UpdateTopics([]string{unknownTopic, malformedTopic, invalidSporkIDTopic})
		unknownTopicRpc := unittest.P2PRPCFixture(unittest.WithPrunes(unknownTopicPrune))
		malformedTopicRpc := unittest.P2PRPCFixture(unittest.WithPrunes(malformedTopicPrune))
		invalidSporkIDTopicRpc := unittest.P2PRPCFixture(unittest.WithPrunes(invalidSporkIDTopicPrune))

		from := unittest.PeerIdFixture(t)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgPrune, channels.IsInvalidTopicErr)
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)

		suite.inspector.Start(suite.signalerCtx)

		require.NoError(t, suite.inspector.Inspect(from, unknownTopicRpc))
		require.NoError(t, suite.inspector.Inspect(from, malformedTopicRpc))
		require.NoError(t, suite.inspector.Inspect(from, invalidSporkIDTopicRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})

	suite.T().Run("inspectIHaveMessages should disseminate invalid control message notification for iHave messages with invalid topics as expected", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, suite.sporkID)
		// avoid unknown topics errors
		suite.topicProviderOracle.UpdateTopics([]string{unknownTopic, malformedTopic, invalidSporkIDTopic})
		unknownTopicIhave := unittest.P2PRPCIHaveFixture(&unknownTopic, unittest.IdentifierListFixture(5).Strings()...)
		malformedTopicIhave := unittest.P2PRPCIHaveFixture(&malformedTopic, unittest.IdentifierListFixture(5).Strings()...)
		invalidSporkIDTopicIhave := unittest.P2PRPCIHaveFixture(&invalidSporkIDTopic, unittest.IdentifierListFixture(5).Strings()...)

		unknownTopicRpc := unittest.P2PRPCFixture(unittest.WithIHaves(unknownTopicIhave))
		malformedTopicRpc := unittest.P2PRPCFixture(unittest.WithIHaves(malformedTopicIhave))
		invalidSporkIDTopicRpc := unittest.P2PRPCFixture(unittest.WithIHaves(invalidSporkIDTopicIhave))

		from := unittest.PeerIdFixture(t)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIHave, channels.IsInvalidTopicErr)
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)
		suite.inspector.Start(suite.signalerCtx)

		require.NoError(t, suite.inspector.Inspect(from, unknownTopicRpc))
		require.NoError(t, suite.inspector.Inspect(from, malformedTopicRpc))
		require.NoError(t, suite.inspector.Inspect(from, invalidSporkIDTopicRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})

	suite.T().Run("inspectIHaveMessages should disseminate invalid control message notification for iHave messages with duplicate message ids as expected",
		func(t *testing.T) {
			suite.SetupTest()
			defer suite.StopInspector()
			validTopic := fmt.Sprintf("%s/%s", channels.PushBlocks.String(), suite.sporkID)
			// avoid unknown topics errors
			suite.topicProviderOracle.UpdateTopics([]string{validTopic})
			duplicateMsgID := unittest.IdentifierFixture()
			msgIds := flow.IdentifierList{duplicateMsgID, duplicateMsgID, duplicateMsgID}
			duplicateMsgIDIHave := unittest.P2PRPCIHaveFixture(&validTopic, append(msgIds, unittest.IdentifierListFixture(5)...).Strings()...)

			duplicateMsgIDRpc := unittest.P2PRPCFixture(unittest.WithIHaves(duplicateMsgIDIHave))

			from := unittest.PeerIdFixture(t)
			checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIHave, validation.IsDuplicateTopicErr)
			suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
			suite.inspector.Start(suite.signalerCtx)

			require.NoError(t, suite.inspector.Inspect(from, duplicateMsgIDRpc))
			// sleep for 1 second to ensure rpc's is processed
			time.Sleep(time.Second)
		})

	suite.T().Run("inspectIWantMessages should disseminate invalid control message notification for iWant messages when duplicate message ids exceeds the allowed threshold",
		func(t *testing.T) {
			suite.SetupTest()
			defer suite.StopInspector()
			duplicateMsgID := unittest.IdentifierFixture()
			duplicates := flow.IdentifierList{duplicateMsgID, duplicateMsgID}
			msgIds := append(duplicates, unittest.IdentifierListFixture(5)...).Strings()
			duplicateMsgIDIWant := unittest.P2PRPCIWantFixture(msgIds...)

			duplicateMsgIDRpc := unittest.P2PRPCFixture(unittest.WithIWants(duplicateMsgIDIWant))

			from := unittest.PeerIdFixture(t)
			checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIWant, validation.IsIWantDuplicateMsgIDThresholdErr)
			suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
			suite.rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
			suite.rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Run(func(args mock.Arguments) {
				id, ok := args[0].(string)
				require.True(t, ok)
				require.Contains(t, msgIds, id)
			})

			suite.inspector.Start(suite.signalerCtx)

			require.NoError(t, suite.inspector.Inspect(from, duplicateMsgIDRpc))
			// sleep for 1 second to ensure rpc's is processed
			time.Sleep(time.Second)
		})

	suite.T().Run("inspectIWantMessages should disseminate invalid control message notification for iWant messages when cache misses exceeds allowed threshold",
		func(t *testing.T) {
			suite.SetupTest()
			defer suite.StopInspector()
			// set cache miss check size to 0 forcing the inspector to check the cache misses with only a single iWant
			suite.config.CacheMissCheckSize = 0
			// set high cache miss threshold to ensure we only disseminate notification when it is exceeded
			suite.config.IWantRPCInspectionConfig.CacheMissThreshold = .9
			msgIds := unittest.IdentifierListFixture(100).Strings()
			// oracle must be set even though iWant messages do not have topic IDs
			inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixture(msgIds...)))

			from := unittest.PeerIdFixture(t)
			checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgIWant, validation.IsIWantCacheMissThresholdErr)
			suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
			suite.rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
			// return false each time to eventually force a notification to be disseminated when the cache miss count finally exceeds the 90% threshold
			suite.rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(false).Run(func(args mock.Arguments) {
				id, ok := args[0].(string)
				require.True(t, ok)
				require.Contains(t, msgIds, id)
			})

			suite.inspector.Start(suite.signalerCtx)

			require.NoError(t, suite.inspector.Inspect(from, inspectMsgRpc))
			// sleep for 1 second to ensure rpc's is processed
			time.Sleep(time.Second)
		})

	suite.T().Run("inspectIWantMessages should not disseminate invalid control message notification for iWant messages when cache misses exceeds allowed threshold if cache miss check size not exceeded",
		func(t *testing.T) {
			suite.SetupTest()
			defer suite.StopInspector()
			defer suite.distributor.AssertNotCalled(t, "Distribute")
			// if size of iwants not greater than 10 cache misses will not be checked
			suite.config.CacheMissCheckSize = 10
			// set high cache miss threshold to ensure we only disseminate notification when it is exceeded
			suite.config.IWantRPCInspectionConfig.CacheMissThreshold = .9
			msgIds := unittest.IdentifierListFixture(100).Strings()
			inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixture(msgIds...)))
			suite.rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
			// return false each time to eventually force a notification to be disseminated when the cache miss count finally exceeds the 90% threshold
			suite.rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(false).Run(func(args mock.Arguments) {
				id, ok := args[0].(string)
				require.True(t, ok)
				require.Contains(t, msgIds, id)
			})

			from := unittest.PeerIdFixture(t)
			suite.inspector.Start(suite.signalerCtx)

			require.NoError(t, suite.inspector.Inspect(from, inspectMsgRpc))
			// sleep for 1 second to ensure rpc's is processed
			time.Sleep(time.Second)
		})

	suite.T().Run("inspectRpcPublishMessages should disseminate invalid control message notification when invalid pubsub messages count greater than configured RpcMessageErrorThreshold",
		func(t *testing.T) {
			suite.SetupTest()
			defer suite.StopInspector()
			// 5 invalid pubsub messages will force notification dissemination
			suite.config.RpcMessageErrorThreshold = 4
			// create unknown topic
			unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", unittest.IdentifierFixture(), suite.sporkID)).String()
			// create malformed topic
			malformedTopic := channels.Topic("!@#$%^&**((").String()
			// a topics spork ID is considered invalid if it does not match the current spork ID
			invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture())).String()

			// create 10 normal messages
			pubsubMsgs := unittest.GossipSubMessageFixtures(10, fmt.Sprintf("%s/%s", channels.TestNetworkChannel, suite.sporkID))
			// add 5 invalid messages to force notification dissemination
			pubsubMsgs = append(pubsubMsgs, []*pubsub_pb.Message{
				{Topic: &unknownTopic},
				{Topic: &malformedTopic},
				{Topic: &malformedTopic},
				{Topic: &invalidSporkIDTopic},
				{Topic: &invalidSporkIDTopic},
			}...)
			rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
			topics := make([]string, len(pubsubMsgs))
			for i, msg := range pubsubMsgs {
				topics[i] = *msg.Topic
			}
			// set topic oracle to return list of topics to avoid hasSubscription errors and force topic validation
			suite.topicProviderOracle.UpdateTopics(topics)
			from := unittest.PeerIdFixture(t)
			checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr)
			suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)

			suite.inspector.Start(suite.signalerCtx)

			require.NoError(t, suite.inspector.Inspect(from, rpc))
			// sleep for 1 second to ensure rpc's is processed
			time.Sleep(time.Second)
		})

	suite.T().Run("inspectRpcPublishMessages should disseminate invalid control message notification when subscription missing for topic", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		// 5 invalid pubsub messages will force notification dissemination
		suite.config.RpcMessageErrorThreshold = 4
		pubsubMsgs := unittest.GossipSubMessageFixtures(5, fmt.Sprintf("%s/%s", channels.TestNetworkChannel, suite.sporkID))
		from := unittest.PeerIdFixture(t)
		rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
		checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr)
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		suite.inspector.Start(suite.signalerCtx)
		require.NoError(t, suite.inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})

	suite.T().Run("inspectRpcPublishMessages should disseminate invalid control message notification when publish messages contain no topic", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		// 5 invalid pubsub messages will force notification dissemination
		suite.config.RpcMessageErrorThreshold = 4

		pubsubMsgs := unittest.GossipSubMessageFixtures(10, "")
		rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
		topics := make([]string, len(pubsubMsgs))
		for i, msg := range pubsubMsgs {
			topics[i] = *msg.Topic
		}
		from := unittest.PeerIdFixture(t)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr)
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		suite.inspector.Start(suite.signalerCtx)
		require.NoError(t, suite.inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})
	suite.T().Run("inspectRpcPublishMessages should not inspect pubsub message sender on public networks", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		from := unittest.PeerIdFixture(t)
		defer suite.idProvider.AssertNotCalled(t, "ByPeerID", from)
		topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, suite.sporkID)
		suite.topicProviderOracle.UpdateTopics([]string{topic})
		pubsubMsgs := unittest.GossipSubMessageFixtures(10, topic, unittest.WithFrom(from))
		rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
		suite.inspector.Start(suite.signalerCtx)
		require.NoError(t, suite.inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})
	suite.T().Run("inspectRpcPublishMessages should disseminate invalid control message notification when message is from unstaked peer", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()

		// override the inspector and params, run the inspector in private mode
		suite.params.NetworkingType = network.PrivateNetwork
		var err error
		suite.inspector, err = validation.NewControlMsgValidationInspector(suite.params)
		require.NoError(suite.T(), err, "failed to create control message validation inspector fixture")

		from := unittest.PeerIdFixture(t)
		topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, suite.sporkID)
		suite.topicProviderOracle.UpdateTopics([]string{topic})
		// default RpcMessageErrorThreshold is 500, 501 messages should trigger a notification
		pubsubMsgs := unittest.GossipSubMessageFixtures(501, topic, unittest.WithFrom(from))
		suite.idProvider.On("ByPeerID", from).Return(nil, false).Times(501)
		rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
		checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr)
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		suite.inspector.Start(suite.signalerCtx)
		require.NoError(t, suite.inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})
	suite.T().Run("inspectRpcPublishMessages should disseminate invalid control message notification when message is from ejected peer", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()

		// override the inspector and params, run the inspector in private mode
		suite.params.NetworkingType = network.PrivateNetwork
		var err error
		suite.inspector, err = validation.NewControlMsgValidationInspector(suite.params)
		require.NoError(suite.T(), err, "failed to create control message validation inspector fixture")

		from := unittest.PeerIdFixture(t)
		id := unittest.IdentityFixture()
		id.Ejected = true
		topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, suite.sporkID)
		suite.topicProviderOracle.UpdateTopics([]string{topic})
		pubsubMsgs := unittest.GossipSubMessageFixtures(501, topic, unittest.WithFrom(from))
		suite.idProvider.On("ByPeerID", from).Return(id, true).Times(501)
		rpc := unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...))
		require.NoError(t, err, "failed to get inspect message request")
		checkNotification := checkNotificationFunc(t, from, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr)
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		suite.inspector.Start(suite.signalerCtx)
		require.NoError(t, suite.inspector.Inspect(from, rpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})
}

// TestNewControlMsgValidationInspector_validateClusterPrefixedTopic ensures cluster prefixed topics are validated as expected.
func (suite *ControlMsgValidationInspectorSuite) TestNewControlMsgValidationInspector_validateClusterPrefixedTopic() {
	suite.T().Run("validateClusterPrefixedTopic should not return an error for valid cluster prefixed topics", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		defer suite.distributor.AssertNotCalled(t, "Distribute")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), suite.sporkID)).String()
		suite.topicProviderOracle.UpdateTopics([]string{clusterPrefixedTopic})
		from := unittest.PeerIdFixture(t)
		suite.idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic)))
		suite.inspector.ActiveClustersChanged(flow.ChainIDList{clusterID,
			flow.ChainID(unittest.IdentifierFixture().String()),
			flow.ChainID(unittest.IdentifierFixture().String())})
		suite.inspector.Start(suite.signalerCtx)
		require.NoError(t, suite.inspector.Inspect(from, inspectMsgRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})

	suite.T().Run("validateClusterPrefixedTopic should not return error if cluster prefixed hard threshold not exceeded for unknown cluster ids", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		defer suite.distributor.AssertNotCalled(t, "Distribute")
		// set hard threshold to small number , ensure that a single unknown cluster prefix id does not cause a notification to be disseminated
		suite.config.ClusterPrefixHardThreshold = 2
		defer suite.distributor.AssertNotCalled(t, "Distribute")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), suite.sporkID)).String()
		from := unittest.PeerIdFixture(t)
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic)))
		suite.idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
		suite.inspector.Start(suite.signalerCtx)
		require.NoError(t, suite.inspector.Inspect(from, inspectMsgRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})

	suite.T().Run("validateClusterPrefixedTopic should return an error when sender is unstaked", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		defer suite.distributor.AssertNotCalled(t, "Distribute")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), suite.sporkID)).String()
		suite.topicProviderOracle.UpdateTopics([]string{clusterPrefixedTopic})
		from := unittest.PeerIdFixture(t)
		suite.idProvider.On("ByPeerID", from).Return(nil, false).Once()
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic)))
		suite.inspector.ActiveClustersChanged(flow.ChainIDList{flow.ChainID(unittest.IdentifierFixture().String())})

		suite.inspector.Start(suite.signalerCtx)
		require.NoError(t, suite.inspector.Inspect(from, inspectMsgRpc))
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})

	suite.T().Run("validateClusterPrefixedTopic should return error if cluster prefixed hard threshold exceeded for unknown cluster ids", func(t *testing.T) {
		suite.SetupTest()
		defer suite.StopInspector()
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), suite.sporkID)).String()
		suite.topicProviderOracle.UpdateTopics([]string{clusterPrefixedTopic})
		// the 11th unknown cluster ID error should cause an error
		suite.config.ClusterPrefixHardThreshold = 10
		from := unittest.PeerIdFixture(t)
		identity := unittest.IdentityFixture()
		suite.idProvider.On("ByPeerID", from).Return(identity, true).Times(11)
		checkNotification := checkNotificationFunc(t, from, p2pmsg.CtrlMsgGraft, channels.IsUnknownClusterIDErr)
		inspectMsgRpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic)))
		suite.inspector.ActiveClustersChanged(flow.ChainIDList{flow.ChainID(unittest.IdentifierFixture().String())})
		suite.distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		suite.inspector.Start(suite.signalerCtx)
		for i := 0; i < 11; i++ {
			require.NoError(t, suite.inspector.Inspect(from, inspectMsgRpc))
		}
		// sleep for 1 second to ensure rpc's is processed
		time.Sleep(time.Second)
	})
}

// TestControlMessageValidationInspector_ActiveClustersChanged validates the expected update of the active cluster IDs list.
func (suite *ControlMsgValidationInspectorSuite) TestControlMessageValidationInspector_ActiveClustersChanged() {
	suite.SetupTest()
	defer suite.StopInspector()
	defer suite.distributor.AssertNotCalled(suite.T(), "Distribute")
	identity := unittest.IdentityFixture()
	suite.idProvider.On("ByPeerID", mock.AnythingOfType("peer.ID")).Return(identity, true).Times(5)
	activeClusterIds := make(flow.ChainIDList, 0)
	for _, id := range unittest.IdentifierListFixture(5) {
		activeClusterIds = append(activeClusterIds, flow.ChainID(id.String()))
	}
	suite.inspector.ActiveClustersChanged(activeClusterIds)
	suite.inspector.Start(suite.signalerCtx)
	from := unittest.PeerIdFixture(suite.T())
	for _, id := range activeClusterIds {
		topic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(id), suite.sporkID)).String()
		rpc := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&topic)))
		require.NoError(suite.T(), suite.inspector.Inspect(from, rpc))
	}
	// sleep for 1 second to ensure rpc's is processed
	time.Sleep(time.Second)
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
	isExpectedErr func(err error) bool) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, expectedPeerID, notification.PeerID)
		require.Equal(t, expectedMsgType, notification.MsgType)
		require.True(t, isExpectedErr(notification.Error))
	}
}
