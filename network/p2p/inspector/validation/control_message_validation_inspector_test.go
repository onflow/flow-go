package validation_test

import (
	"fmt"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	"testing"

	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestNewControlMsgValidationInspector(t *testing.T) {
	t.Run("should create validation inspector without error", func(t *testing.T) {
		sporkID := unittest.IdentifierFixture()
		flowConfig, err := config.DefaultConfig()
		require.NoError(t, err, "failed to get default flow config")
		distributor := mockp2p.NewGossipSubInspectorNotifDistributor(t)
		idProvider := mockmodule.NewIdentityProvider(t)
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
	})
}

// TestControlMessageValidationInspector_TruncateRPC verifies the expected truncation behavior of RPC control messages.
// Message truncation for each control message type occurs when the count of control
// messages exceeds the configured maximum sample size for that control message type.
func TestControlMessageValidationInspector_truncateRPC(t *testing.T) {
	t.Run("truncateGraftMessages should truncate graft messages as expected", func(t *testing.T) {
		inspector, _, _, _, _, config := inspectorFixture(t)
		config.GraftPruneMessageMaxSampleSize = 100
		// topic validation not performed so we can use random strings
		graftsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixtures(unittest.IdentifierListFixture(200).Strings()...)...))
		require.Greater(t, len(graftsGreaterThanMaxSampleSize.GetControl().GetGraft()), config.GraftPruneMessageMaxSampleSize)
		graftsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixtures(unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(graftsLessThanMaxSampleSize.GetControl().GetGraft()), config.GraftPruneMessageMaxSampleSize)
		inspector.TruncateGraftMessages(graftsGreaterThanMaxSampleSize)
		inspector.TruncateGraftMessages(graftsLessThanMaxSampleSize)
		// rpc with grafts greater than configured max sample size should be truncated to GraftPruneMessageMaxSampleSize
		require.Len(t, graftsGreaterThanMaxSampleSize.GetControl().GetGraft(), config.GraftPruneMessageMaxSampleSize)
		// rpc with grafts less than GraftPruneMessageMaxSampleSize should not be truncated
		require.Len(t, graftsLessThanMaxSampleSize.GetControl().GetGraft(), 50)
	})

	t.Run("truncatePruneMessages should truncate prune messages as expected", func(t *testing.T) {
		inspector, _, _, _, _, config := inspectorFixture(t)
		config.GraftPruneMessageMaxSampleSize = 100
		// topic validation not performed so we can use random strings
		prunesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(200).Strings()...)...))
		require.Greater(t, len(prunesGreaterThanMaxSampleSize.GetControl().GetPrune()), config.GraftPruneMessageMaxSampleSize)
		prunesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithPrunes(unittest.P2PRPCPruneFixtures(unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(prunesLessThanMaxSampleSize.GetControl().GetPrune()), config.GraftPruneMessageMaxSampleSize)
		inspector.TruncatePruneMessages(prunesGreaterThanMaxSampleSize)
		inspector.TruncatePruneMessages(prunesLessThanMaxSampleSize)
		// rpc with prunes greater than configured max sample size should be truncated to GraftPruneMessageMaxSampleSize
		require.Len(t, prunesGreaterThanMaxSampleSize.GetControl().GetPrune(), config.GraftPruneMessageMaxSampleSize)
		// rpc with prunes less than GraftPruneMessageMaxSampleSize should not be truncated
		require.Len(t, prunesLessThanMaxSampleSize.GetControl().GetPrune(), 50)
	})

	t.Run("TruncateIHaveMessages should truncate iHave messages as expected", func(t *testing.T) {
		inspector, _, _, _, _, config := inspectorFixture(t)
		config.IHaveRPCInspectionConfig.MaxSampleSize = 100
		// topic validation not performed so we can use random strings
		iHavesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(200, unittest.IdentifierListFixture(200).Strings()...)...))
		require.Greater(t, len(iHavesGreaterThanMaxSampleSize.GetControl().GetIhave()), config.IHaveRPCInspectionConfig.MaxSampleSize)
		iHavesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(200, unittest.IdentifierListFixture(50).Strings()...)...))
		require.Less(t, len(iHavesLessThanMaxSampleSize.GetControl().GetIhave()), config.IHaveRPCInspectionConfig.MaxSampleSize)
		inspector.TruncateIHaveMessages(iHavesGreaterThanMaxSampleSize)
		inspector.TruncateIHaveMessages(iHavesLessThanMaxSampleSize)
		// rpc with iHaves greater than configured max sample size should be truncated to MaxSampleSize
		require.Len(t, iHavesGreaterThanMaxSampleSize.GetControl().GetIhave(), config.IHaveRPCInspectionConfig.MaxSampleSize)
		// rpc with iHaves less than MaxSampleSize should not be truncated
		require.Len(t, iHavesLessThanMaxSampleSize.GetControl().GetIhave(), 50)
	})

	t.Run("truncateIHaveMessageIds should truncate iHave message ids as expected", func(t *testing.T) {
		inspector, _, _, _, _, config := inspectorFixture(t)
		config.IHaveRPCInspectionConfig.MaxMessageIDSampleSize = 100
		// topic validation not performed so we can use random strings
		iHavesGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(200, unittest.IdentifierListFixture(10).Strings()...)...))
		iHavesLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIHaves(unittest.P2PRPCIHaveFixtures(50, unittest.IdentifierListFixture(10).Strings()...)...))
		inspector.TruncateIHaveMessageIds(iHavesGreaterThanMaxSampleSize)
		inspector.TruncateIHaveMessageIds(iHavesLessThanMaxSampleSize)
		for _, iHave := range iHavesGreaterThanMaxSampleSize.GetControl().GetIhave() {
			// rpc with iHaves message ids greater than configured max sample size should be truncated to MaxSampleSize
			require.Len(t, iHave.GetMessageIDs(), config.IHaveRPCInspectionConfig.MaxMessageIDSampleSize)
		}
		for _, iHave := range iHavesLessThanMaxSampleSize.GetControl().GetIhave() {
			// rpc with iHaves message ids less than MaxSampleSize should not be truncated
			require.Len(t, iHave.GetMessageIDs(), 50)
		}
	})

	t.Run("truncateIWantMessages should truncate iWant messages as expected", func(t *testing.T) {
		inspector, _, rpcTracker, _, _, config := inspectorFixture(t)
		config.IWantRPCInspectionConfig.MaxSampleSize = 100
		iWantsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(200, 200)...))
		require.Greater(t, uint(len(iWantsGreaterThanMaxSampleSize.GetControl().GetIwant())), config.IWantRPCInspectionConfig.MaxSampleSize)
		iWantsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(50, 200)...))
		require.Less(t, uint(len(iWantsLessThanMaxSampleSize.GetControl().GetIwant())), config.IWantRPCInspectionConfig.MaxSampleSize)
		peerID := peer.ID("peer")
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Twice()
		inspector.TruncateIWantMessages(peerID, iWantsGreaterThanMaxSampleSize)
		inspector.TruncateIWantMessages(peerID, iWantsLessThanMaxSampleSize)
		// rpc with iWants greater than configured max sample size should be truncated to MaxSampleSize
		require.Len(t, iWantsGreaterThanMaxSampleSize.GetControl().GetIwant(), int(config.IWantRPCInspectionConfig.MaxSampleSize))
		// rpc with iWants less than MaxSampleSize should not be truncated
		require.Len(t, iWantsLessThanMaxSampleSize.GetControl().GetIwant(), 50)
	})

	t.Run("truncateIWantMessageIds should truncate iWant message ids as expected", func(t *testing.T) {
		inspector, _, rpcTracker, _, _, config := inspectorFixture(t)
		config.IWantRPCInspectionConfig.MaxMessageIDSampleSize = 100
		iWantsGreaterThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 200)...))
		iWantsLessThanMaxSampleSize := unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixtures(10, 50)...))
		peerID := peer.ID("peer")
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Twice()
		inspector.TruncateIWantMessages(peerID, iWantsGreaterThanMaxSampleSize)
		inspector.TruncateIWantMessages(peerID, iWantsLessThanMaxSampleSize)
		for _, iWant := range iWantsGreaterThanMaxSampleSize.GetControl().GetIwant() {
			// rpc with iWants message ids greater than configured max sample size should be truncated to MaxSampleSize
			require.Len(t, iWant.GetMessageIDs(), config.IWantRPCInspectionConfig.MaxMessageIDSampleSize)
		}
		for _, iWant := range iWantsLessThanMaxSampleSize.GetControl().GetIwant() {
			// rpc with iWants less than MaxSampleSize should not be truncated
			require.Len(t, iWant.GetMessageIDs(), 50)
		}
	})
}

// TestControlMessageValidationInspector_processInspectRPCReq verifies the correct behavior of control message validation.
// It ensures that valid RPC control messages do not trigger erroneous invalid control message notifications,
// while all types of invalid control messages trigger expected notifications.
func TestControlMessageValidationInspector_processInspectRPCReq(t *testing.T) {
	t.Run("processInspectRPCReq should not disseminate any invalid notification errors for valid RPC's", func(t *testing.T) {
		inspector, distributor, rpcTracker, _, sporkID, _ := inspectorFixture(t)
		defer distributor.AssertNotCalled(t, "Distribute")
		topics := []string{
			fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID),
			fmt.Sprintf("%s/%s", channels.PushBlocks, sporkID),
			fmt.Sprintf("%s/%s", channels.SyncCommittee, sporkID),
			fmt.Sprintf("%s/%s", channels.RequestChunks, sporkID),
		}
		// set topic oracle to return list of topics excluding first topic sent
		require.NoError(t, inspector.SetTopicOracle(func() []string {
			return topics
		}))
		grafts := unittest.P2PRPCGraftFixtures(topics...)
		prunes := unittest.P2PRPCPruneFixtures(topics...)
		ihaves := unittest.P2PRPCIHaveFixtures(50, topics...)
		iwants := unittest.P2PRPCIWantFixtures(2, 5)
		pubsubMsgs := unittest.GossipSubMessageFixtures(10, topics[0])

		// avoid cache misses for iwant messages.
		iwants[0].MessageIDs = ihaves[0].MessageIDs[:10]
		iwants[1].MessageIDs = ihaves[1].MessageIDs[11:20]
		expectedMsgIds := make([]string, 0)
		expectedMsgIds = append(expectedMsgIds, ihaves[0].MessageIDs[:10]...)
		expectedMsgIds = append(expectedMsgIds, ihaves[1].MessageIDs[11:20]...)
		expectedPeerID := unittest.PeerIdFixture(t)
		req, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(
			unittest.WithGrafts(grafts...),
			unittest.WithPrunes(prunes...),
			unittest.WithIHaves(ihaves...),
			unittest.WithIWants(iwants...),
			unittest.WithPubsubMessages(pubsubMsgs...)),
		)
		require.NoError(t, err, "failed to get inspect message request")
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Run(func(args mock.Arguments) {
			id, ok := args[0].(string)
			require.True(t, ok)
			require.Contains(t, expectedMsgIds, id)
		})
		require.NoError(t, inspector.ProcessInspectRPCReq(req))
	})

	t.Run("processInspectRPCReq should disseminate invalid control message notification for control messages with duplicate topics", func(t *testing.T) {
		inspector, distributor, _, _, sporkID, _ := inspectorFixture(t)
		defer distributor.AssertNotCalled(t, "Distribute")
		duplicateTopic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
		// create control messages with duplicate topic
		grafts := []*pubsub_pb.ControlGraft{unittest.P2PRPCGraftFixture(&duplicateTopic), unittest.P2PRPCGraftFixture(&duplicateTopic)}
		prunes := []*pubsub_pb.ControlPrune{unittest.P2PRPCPruneFixture(&duplicateTopic), unittest.P2PRPCPruneFixture(&duplicateTopic)}
		ihaves := []*pubsub_pb.ControlIHave{unittest.P2PRPCIHaveFixture(&duplicateTopic, unittest.IdentifierListFixture(20).Strings()...), unittest.P2PRPCIHaveFixture(&duplicateTopic, unittest.IdentifierListFixture(20).Strings()...)}
		expectedPeerID := unittest.PeerIdFixture(t)
		duplicateTopicGraftsReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithGrafts(grafts...)))
		require.NoError(t, err, "failed to get inspect message request")
		duplicateTopicPrunesReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithPrunes(prunes...)))
		require.NoError(t, err, "failed to get inspect message request")
		duplicateTopicIHavesReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithIHaves(ihaves...)))
		require.NoError(t, err, "failed to get inspect message request")
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(func(args mock.Arguments) {
			notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
			require.True(t, ok)
			require.Equal(t, expectedPeerID, notification.PeerID)
			require.Contains(t, []p2pmsg.ControlMessageType{p2pmsg.CtrlMsgGraft, p2pmsg.CtrlMsgPrune, p2pmsg.CtrlMsgIHave}, notification.MsgType)
			require.True(t, validation.IsDuplicateTopicErr(notification.Error))
		})

		require.NoError(t, inspector.ProcessInspectRPCReq(duplicateTopicGraftsReq))
		require.NoError(t, inspector.ProcessInspectRPCReq(duplicateTopicPrunesReq))
		require.NoError(t, inspector.ProcessInspectRPCReq(duplicateTopicIHavesReq))
	})

	t.Run("inspectGraftMessages should disseminate invalid control message notification for invalid graft messages as expected", func(t *testing.T) {
		inspector, distributor, _, _, sporkID, _ := inspectorFixture(t)
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		unknownTopicGraft := unittest.P2PRPCGraftFixture(&unknownTopic)
		malformedTopicGraft := unittest.P2PRPCGraftFixture(&malformedTopic)
		invalidSporkIDTopicGraft := unittest.P2PRPCGraftFixture(&invalidSporkIDTopic)

		expectedPeerID := unittest.PeerIdFixture(t)
		unknownTopicReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithGrafts(unknownTopicGraft)))
		require.NoError(t, err, "failed to get inspect message request")
		malformedTopicReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithGrafts(malformedTopicGraft)))
		require.NoError(t, err, "failed to get inspect message request")
		invalidSporkIDTopicReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithGrafts(invalidSporkIDTopicGraft)))
		require.NoError(t, err, "failed to get inspect message request")

		checkNotification := checkNotificationFunc(t, expectedPeerID, p2pmsg.CtrlMsgGraft, channels.IsInvalidTopicErr)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)

		require.NoError(t, inspector.ProcessInspectRPCReq(unknownTopicReq))
		require.NoError(t, inspector.ProcessInspectRPCReq(malformedTopicReq))
		require.NoError(t, inspector.ProcessInspectRPCReq(invalidSporkIDTopicReq))
	})

	t.Run("inspectPruneMessages should disseminate invalid control message notification for invalid prune messages as expected", func(t *testing.T) {
		inspector, distributor, _, _, sporkID, _ := inspectorFixture(t)
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		unknownTopicPrune := unittest.P2PRPCPruneFixture(&unknownTopic)
		malformedTopicPrune := unittest.P2PRPCPruneFixture(&malformedTopic)
		invalidSporkIDTopicPrune := unittest.P2PRPCPruneFixture(&invalidSporkIDTopic)

		expectedPeerID := unittest.PeerIdFixture(t)
		unknownTopicReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithPrunes(unknownTopicPrune)))
		require.NoError(t, err, "failed to get inspect message request")
		malformedTopicReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithPrunes(malformedTopicPrune)))
		require.NoError(t, err, "failed to get inspect message request")
		invalidSporkIDTopicReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithPrunes(invalidSporkIDTopicPrune)))
		require.NoError(t, err, "failed to get inspect message request")

		checkNotification := checkNotificationFunc(t, expectedPeerID, p2pmsg.CtrlMsgPrune, channels.IsInvalidTopicErr)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)

		require.NoError(t, inspector.ProcessInspectRPCReq(unknownTopicReq))
		require.NoError(t, inspector.ProcessInspectRPCReq(malformedTopicReq))
		require.NoError(t, inspector.ProcessInspectRPCReq(invalidSporkIDTopicReq))
	})

	t.Run("inspectIHaveMessages should disseminate invalid control message notification for iHave messages with invalid topics as expected", func(t *testing.T) {
		inspector, distributor, _, _, sporkID, _ := inspectorFixture(t)
		// create unknown topic
		unknownTopic, malformedTopic, invalidSporkIDTopic := invalidTopics(t, sporkID)
		unknownTopicIhave := unittest.P2PRPCIHaveFixture(&unknownTopic, unittest.IdentifierListFixture(5).Strings()...)
		malformedTopicIhave := unittest.P2PRPCIHaveFixture(&malformedTopic, unittest.IdentifierListFixture(5).Strings()...)
		invalidSporkIDTopicIhave := unittest.P2PRPCIHaveFixture(&invalidSporkIDTopic, unittest.IdentifierListFixture(5).Strings()...)

		expectedPeerID := unittest.PeerIdFixture(t)
		unknownTopicReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithIHaves(unknownTopicIhave)))
		require.NoError(t, err, "failed to get inspect message request")
		malformedTopicReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithIHaves(malformedTopicIhave)))
		require.NoError(t, err, "failed to get inspect message request")
		invalidSporkIDTopicReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithIHaves(invalidSporkIDTopicIhave)))
		require.NoError(t, err, "failed to get inspect message request")

		checkNotification := checkNotificationFunc(t, expectedPeerID, p2pmsg.CtrlMsgIHave, channels.IsInvalidTopicErr)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Times(3).Run(checkNotification)

		require.NoError(t, inspector.ProcessInspectRPCReq(unknownTopicReq))
		require.NoError(t, inspector.ProcessInspectRPCReq(malformedTopicReq))
		require.NoError(t, inspector.ProcessInspectRPCReq(invalidSporkIDTopicReq))
	})

	t.Run("inspectIHaveMessages should disseminate invalid control message notification for iHave messages with duplicate message ids as expected", func(t *testing.T) {
		inspector, distributor, _, _, sporkID, _ := inspectorFixture(t)
		validTopic := fmt.Sprintf("%s/%s", channels.PushBlocks.String(), sporkID)
		duplicateMsgID := unittest.IdentifierFixture()
		msgIds := flow.IdentifierList{duplicateMsgID, duplicateMsgID, duplicateMsgID}
		duplicateMsgIDIHave := unittest.P2PRPCIHaveFixture(&validTopic, append(msgIds, unittest.IdentifierListFixture(5)...).Strings()...)

		expectedPeerID := unittest.PeerIdFixture(t)
		duplicateMsgIDReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithIHaves(duplicateMsgIDIHave)))
		require.NoError(t, err, "failed to get inspect message request")

		checkNotification := checkNotificationFunc(t, expectedPeerID, p2pmsg.CtrlMsgIHave, validation.IsDuplicateTopicErr)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		require.NoError(t, inspector.ProcessInspectRPCReq(duplicateMsgIDReq))
	})

	t.Run("inspectIWantMessages should disseminate invalid control message notification for iWant messages when duplicate message ids exceeds the allowed threshold", func(t *testing.T) {
		inspector, distributor, rpcTracker, _, _, _ := inspectorFixture(t)
		duplicateMsgID := unittest.IdentifierFixture()
		duplicates := flow.IdentifierList{duplicateMsgID, duplicateMsgID}
		msgIds := append(duplicates, unittest.IdentifierListFixture(5)...).Strings()
		duplicateMsgIDIWant := unittest.P2PRPCIWantFixture(msgIds...)

		expectedPeerID := unittest.PeerIdFixture(t)
		duplicateMsgIDReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithIWants(duplicateMsgIDIWant)))
		require.NoError(t, err, "failed to get inspect message request")

		checkNotification := checkNotificationFunc(t, expectedPeerID, p2pmsg.CtrlMsgIWant, validation.IsIWantDuplicateMsgIDThresholdErr)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(true).Run(func(args mock.Arguments) {
			id, ok := args[0].(string)
			require.True(t, ok)
			require.Contains(t, msgIds, id)
		})
		require.NoError(t, inspector.ProcessInspectRPCReq(duplicateMsgIDReq))
	})

	t.Run("inspectIWantMessages should disseminate invalid control message notification for iWant messages when cache misses exceeds allowed threshold", func(t *testing.T) {
		inspector, distributor, rpcTracker, _, _, config := inspectorFixture(t)
		// set cache miss check size to 0 forcing the inspector to check the cache misses with only a single iWant
		config.CacheMissCheckSize = 0
		// set high cache miss threshold to ensure we only disseminate notification when it is exceeded
		config.IWantRPCInspectionConfig.CacheMissThreshold = .9
		msgIds := unittest.IdentifierListFixture(100).Strings()
		expectedPeerID := unittest.PeerIdFixture(t)
		inspectMsgReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixture(msgIds...))))
		require.NoError(t, err, "failed to get inspect message request")

		checkNotification := checkNotificationFunc(t, expectedPeerID, p2pmsg.CtrlMsgIWant, validation.IsIWantCacheMissThresholdErr)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		// return false each time to eventually force a notification to be disseminated when the cache miss count finally exceeds the 90% threshold
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(false).Run(func(args mock.Arguments) {
			id, ok := args[0].(string)
			require.True(t, ok)
			require.Contains(t, msgIds, id)
		})
		require.NoError(t, inspector.ProcessInspectRPCReq(inspectMsgReq))
	})

	t.Run("inspectIWantMessages should not disseminate invalid control message notification for iWant messages when cache misses exceeds allowed threshold if cache miss check size not exceeded", func(t *testing.T) {
		inspector, distributor, rpcTracker, _, _, config := inspectorFixture(t)
		defer distributor.AssertNotCalled(t, "Distribute")
		// if size of iwants not greater than 10 cache misses will not be checked
		config.CacheMissCheckSize = 10
		// set high cache miss threshold to ensure we only disseminate notification when it is exceeded
		config.IWantRPCInspectionConfig.CacheMissThreshold = .9
		msgIds := unittest.IdentifierListFixture(100).Strings()
		expectedPeerID := unittest.PeerIdFixture(t)
		inspectMsgReq, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithIWants(unittest.P2PRPCIWantFixture(msgIds...))))
		require.NoError(t, err, "failed to get inspect message request")
		rpcTracker.On("LastHighestIHaveRPCSize").Return(int64(100)).Maybe()
		// return false each time to eventually force a notification to be disseminated when the cache miss count finally exceeds the 90% threshold
		rpcTracker.On("WasIHaveRPCSent", mock.AnythingOfType("string")).Return(false).Run(func(args mock.Arguments) {
			id, ok := args[0].(string)
			require.True(t, ok)
			require.Contains(t, msgIds, id)
		})
		require.NoError(t, inspector.ProcessInspectRPCReq(inspectMsgReq))
	})

	t.Run("inspectRpcPublishMessages should disseminate invalid control message notification when invalid pubsub messages count greater than configured RpcMessageErrorThreshold", func(t *testing.T) {
		inspector, distributor, _, _, sporkID, config := inspectorFixture(t)
		// 5 invalid pubsub messages will force notification dissemination
		config.RpcMessageErrorThreshold = 4
		// create unknown topic
		unknownTopic := channels.Topic(fmt.Sprintf("%s/%s", unittest.IdentifierFixture(), sporkID)).String()
		// create malformed topic
		malformedTopic := channels.Topic("!@#$%^&**((").String()
		// a topics spork ID is considered invalid if it does not match the current spork ID
		invalidSporkIDTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.PushBlocks, unittest.IdentifierFixture())).String()

		// create 10 normal messages
		pubsubMsgs := unittest.GossipSubMessageFixtures(10, fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID))
		// add 5 invalid messages to force notification dissemination
		pubsubMsgs = append(pubsubMsgs, []*pubsub_pb.Message{
			{Topic: &unknownTopic},
			{Topic: &malformedTopic},
			{Topic: &malformedTopic},
			{Topic: &invalidSporkIDTopic},
			{Topic: &invalidSporkIDTopic},
		}...)
		expectedPeerID := unittest.PeerIdFixture(t)
		req, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...)))
		require.NoError(t, err, "failed to get inspect message request")
		topics := make([]string, len(pubsubMsgs))
		for i, msg := range pubsubMsgs {
			topics[i] = *msg.Topic
		}
		// set topic oracle to return list of topics to avoid hasSubscription errors and force topic validation
		require.NoError(t, inspector.SetTopicOracle(func() []string {
			return topics
		}))
		checkNotification := checkNotificationFunc(t, expectedPeerID, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		require.NoError(t, inspector.ProcessInspectRPCReq(req))
	})

	t.Run("inspectRpcPublishMessages should disseminate invalid control message notification when subscription missing for topic", func(t *testing.T) {
		inspector, distributor, _, _, sporkID, config := inspectorFixture(t)
		// 5 invalid pubsub messages will force notification dissemination
		config.RpcMessageErrorThreshold = 4
		pubsubMsgs := unittest.GossipSubMessageFixtures(5, fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID))
		expectedPeerID := unittest.PeerIdFixture(t)
		req, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...)))
		require.NoError(t, err, "failed to get inspect message request")
		// set topic oracle to return list of topics excluding first topic sent
		require.NoError(t, inspector.SetTopicOracle(func() []string {
			return []string{}
		}))
		checkNotification := checkNotificationFunc(t, expectedPeerID, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		require.NoError(t, inspector.ProcessInspectRPCReq(req))
	})

	t.Run("inspectRpcPublishMessages should disseminate invalid control message notification when publish messages contain no topic", func(t *testing.T) {
		inspector, distributor, _, _, _, config := inspectorFixture(t)
		// 5 invalid pubsub messages will force notification dissemination
		config.RpcMessageErrorThreshold = 4
		pubsubMsgs := unittest.GossipSubMessageFixtures(10, "")
		expectedPeerID := unittest.PeerIdFixture(t)
		req, err := validation.NewInspectRPCRequest(expectedPeerID, unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...)))
		require.NoError(t, err, "failed to get inspect message request")
		topics := make([]string, len(pubsubMsgs))
		for i, msg := range pubsubMsgs {
			topics[i] = *msg.Topic
		}
		// set topic oracle to return list of topics excluding first topic sent
		require.NoError(t, inspector.SetTopicOracle(func() []string {
			return []string{}
		}))
		checkNotification := checkNotificationFunc(t, expectedPeerID, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		require.NoError(t, inspector.ProcessInspectRPCReq(req))
	})
	t.Run("inspectRpcPublishMessages should not inspect pubsub message sender on public networks", func(t *testing.T) {
		inspector, _, _, idProvider, sporkID, _ := inspectorFixture(t)
		pid := unittest.PeerIdFixture(t)
		defer idProvider.AssertNotCalled(t, "ByPeerID", pid)
		topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
		require.NoError(t, inspector.SetTopicOracle(func() []string {
			return []string{topic}
		}))
		pubsubMsgs := unittest.GossipSubMessageFixtures(10, topic, unittest.WithFrom(pid))
		req, err := validation.NewInspectRPCRequest(pid, unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...)))
		require.NoError(t, err, "failed to get inspect message request")
		require.NoError(t, inspector.ProcessInspectRPCReq(req))
	})
	t.Run("inspectRpcPublishMessages should disseminate invalid control message notification when message is from unstaked peer", func(t *testing.T) {
		inspector, distributor, _, idProvider, sporkID, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.NetworkingType = network.PrivateNetwork
		})
		pid := unittest.PeerIdFixture(t)
		topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
		require.NoError(t, inspector.SetTopicOracle(func() []string {
			return []string{topic}
		}))
		// default RpcMessageErrorThreshold is 500, 501 messages should trigger a notification
		pubsubMsgs := unittest.GossipSubMessageFixtures(501, topic, unittest.WithFrom(pid))
		idProvider.On("ByPeerID", pid).Return(nil, false).Times(501)
		req, err := validation.NewInspectRPCRequest(pid, unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...)))
		require.NoError(t, err, "failed to get inspect message request")
		checkNotification := checkNotificationFunc(t, pid, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		require.NoError(t, inspector.ProcessInspectRPCReq(req))
	})
	t.Run("inspectRpcPublishMessages should disseminate invalid control message notification when message is from ejected peer", func(t *testing.T) {
		inspector, distributor, _, idProvider, sporkID, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.NetworkingType = network.PrivateNetwork
		})
		pid := unittest.PeerIdFixture(t)
		id := unittest.IdentityFixture()
		id.Ejected = true
		topic := fmt.Sprintf("%s/%s", channels.TestNetworkChannel, sporkID)
		require.NoError(t, inspector.SetTopicOracle(func() []string {
			return []string{topic}
		}))
		pubsubMsgs := unittest.GossipSubMessageFixtures(501, topic, unittest.WithFrom(pid))
		idProvider.On("ByPeerID", pid).Return(id, true).Times(501)
		req, err := validation.NewInspectRPCRequest(pid, unittest.P2PRPCFixture(unittest.WithPubsubMessages(pubsubMsgs...)))
		require.NoError(t, err, "failed to get inspect message request")
		checkNotification := checkNotificationFunc(t, pid, p2pmsg.RpcPublishMessage, validation.IsInvalidRpcPublishMessagesErr)
		distributor.On("Distribute", mock.AnythingOfType("*p2p.InvCtrlMsgNotif")).Return(nil).Once().Run(checkNotification)
		require.NoError(t, inspector.ProcessInspectRPCReq(req))
	})
}

// TestNewControlMsgValidationInspector_validateClusterPrefixedTopic ensures cluster prefixed topics are validated as expected.
func TestNewControlMsgValidationInspector_validateClusterPrefixedTopic(t *testing.T) {
	t.Run("validateClusterPrefixedTopic should not return an error for valid cluster prefixed topics", func(t *testing.T) {
		inspector, distributor, _, idProvider, sporkID, _ := inspectorFixture(t)
		defer distributor.AssertNotCalled(t, "Distribute")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), sporkID))
		from := peer.ID("peerID987654321")
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
		require.NoError(t, inspector.ValidateTopic(from, clusterPrefixedTopic, flow.ChainIDList{clusterID, flow.ChainID(unittest.IdentifierFixture().String()), flow.ChainID(unittest.IdentifierFixture().String())}))
	})

	t.Run("validateClusterPrefixedTopic should not return error if cluster prefixed hard threshold not exceeded for unknown cluster ids", func(t *testing.T) {
		inspector, distributor, _, idProvider, sporkID, config := inspectorFixture(t)
		// set hard threshold to small number , ensure that a single unknown cluster prefix id does not cause a notification to be disseminated
		config.ClusterPrefixHardThreshold = 2
		defer distributor.AssertNotCalled(t, "Distribute")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), sporkID)).String()
		from := peer.ID("peerID987654321")
		inspectMsgReq, err := validation.NewInspectRPCRequest(from, unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&clusterPrefixedTopic))))
		require.NoError(t, err, "failed to get inspect message request")
		idProvider.On("ByPeerID", from).Return(unittest.IdentityFixture(), true).Once()
		require.NoError(t, inspector.ProcessInspectRPCReq(inspectMsgReq))
	})

	t.Run("validateClusterPrefixedTopic should return an error when sender is unstaked", func(t *testing.T) {
		inspector, distributor, _, idProvider, sporkID, _ := inspectorFixture(t)
		defer distributor.AssertNotCalled(t, "Distribute")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), sporkID))
		from := peer.ID("peerID987654321")
		idProvider.On("ByPeerID", from).Return(nil, false).Once()
		err := inspector.ValidateTopic(from, clusterPrefixedTopic, flow.ChainIDList{flow.ChainID(unittest.IdentifierFixture().String())})
		require.True(t, validation.IsErrUnstakedPeer(err))
	})

	t.Run("validateClusterPrefixedTopic should return error if cluster prefixed hard threshold exceeded for unknown cluster ids", func(t *testing.T) {
		inspector, distributor, _, idProvider, sporkID, _ := inspectorFixture(t, func(params *validation.InspectorParams) {
			params.Config.ClusterPrefixHardThreshold = 10
		})
		defer distributor.AssertNotCalled(t, "Distribute")
		clusterID := flow.ChainID(unittest.IdentifierFixture().String())
		clusterPrefixedTopic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(clusterID), sporkID))
		from := unittest.PeerIdFixture(t)
		identity := unittest.IdentityFixture()
		idProvider.On("ByPeerID", from).Return(identity, true).Times(11)
		for i := 0; i < 10; i++ {
			require.NoError(t, inspector.ValidateTopic(from, clusterPrefixedTopic, flow.ChainIDList{flow.ChainID(unittest.IdentifierFixture().String())}))
		}
		err := inspector.ValidateTopic(from, clusterPrefixedTopic, flow.ChainIDList{flow.ChainID(unittest.IdentifierFixture().String())})
		require.True(t, channels.IsUnknownClusterIDErr(err))
	})
}

// TestControlMessageValidationInspector_ActiveClustersChanged validates the expected update of the active cluster IDs list.
func TestControlMessageValidationInspector_ActiveClustersChanged(t *testing.T) {
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err, "failed to get default flow config")
	distributor := mockp2p.NewGossipSubInspectorNotifDistributor(t)
	idProvider := mockmodule.NewIdentityProvider(t)
	identity := unittest.IdentityFixture()
	idProvider.On("ByPeerID", mock.AnythingOfType("peer.ID")).Return(identity, true).Times(5)
	inspector, err := validation.NewControlMsgValidationInspector(&validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &flowConfig.NetworkConfig.GossipSubRPCValidationInspectorConfigs,
		Distributor:             distributor,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              mockp2p.NewRpcControlTracking(t),
		NetworkingType:          network.PrivateNetwork,
	})
	require.NoError(t, err)
	activeClusterIds := make(flow.ChainIDList, 0)
	for _, id := range unittest.IdentifierListFixture(5) {
		activeClusterIds = append(activeClusterIds, flow.ChainID(id.String()))
	}
	inspector.ActiveClustersChanged(activeClusterIds)
	for _, id := range activeClusterIds {
		topic := channels.Topic(fmt.Sprintf("%s/%s", channels.SyncCluster(id), sporkID)).String()
		req, err := validation.NewInspectRPCRequest(unittest.PeerIdFixture(t), unittest.P2PRPCFixture(unittest.WithGrafts(unittest.P2PRPCGraftFixture(&topic))))
		require.NoError(t, err, "failed to get inspect message request")
		require.NoError(t, inspector.ProcessInspectRPCReq(req))
	}
}

// inspectorFixture returns a *ControlMsgValidationInspector fixture.
func inspectorFixture(t *testing.T, opts ...func(*validation.InspectorParams)) (*validation.ControlMsgValidationInspector, *mockp2p.GossipSubInspectorNotifDistributor, *mockp2p.RpcControlTracking, *mockmodule.IdentityProvider, flow.Identifier, *p2pconf.GossipSubRPCValidationInspectorConfigs) {
	sporkID := unittest.IdentifierFixture()
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err, "failed to get default flow config")
	distributor := mockp2p.NewGossipSubInspectorNotifDistributor(t)
	idProvider := mockmodule.NewIdentityProvider(t)
	rpcTracker := mockp2p.NewRpcControlTracking(t)
	params := &validation.InspectorParams{
		Logger:                  unittest.Logger(),
		SporkID:                 sporkID,
		Config:                  &flowConfig.NetworkConfig.GossipSubRPCValidationInspectorConfigs,
		Distributor:             distributor,
		IdProvider:              idProvider,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		InspectorMetrics:        metrics.NewNoopCollector(),
		RpcTracker:              rpcTracker,
		NetworkingType:          network.PublicNetwork,
	}
	for _, opt := range opts {
		opt(params)
	}
	inspector, err := validation.NewControlMsgValidationInspector(params)
	require.NoError(t, err, "failed to create control message validation inspector fixture")
	return inspector, distributor, rpcTracker, idProvider, sporkID, &flowConfig.NetworkConfig.GossipSubRPCValidationInspectorConfigs
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
func checkNotificationFunc(t *testing.T, expectedPeerID peer.ID, expectedMsgType p2pmsg.ControlMessageType, isExpectedErr func(err error) bool) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		notification, ok := args[0].(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)
		require.Equal(t, expectedPeerID, notification.PeerID)
		require.Equal(t, expectedMsgType, notification.MsgType)
		require.True(t, isExpectedErr(notification.Error))
	}
}
