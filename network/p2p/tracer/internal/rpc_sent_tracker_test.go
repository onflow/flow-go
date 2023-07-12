package internal

import (
	"context"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewRPCSentTracker ensures *RPCSenTracker is created as expected.
func TestNewRPCSentTracker(t *testing.T) {
	tracker := mockTracker(t)
	require.NotNil(t, tracker)
}

// TestRPCSentTracker_IHave ensures *RPCSentTracker tracks sent iHave control messages as expected.
func TestRPCSentTracker_IHave(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	tracker := mockTracker(t)
	require.NotNil(t, tracker)

	tracker.Start(signalerCtx)
	defer func() {
		cancel()
		unittest.RequireComponentsDoneBefore(t, time.Second, tracker)
	}()

	t.Run("WasIHaveRPCSent should return false for iHave message Id that has not been tracked", func(t *testing.T) {
		require.False(t, tracker.WasIHaveRPCSent("topic_id", "message_id"))
	})

	t.Run("WasIHaveRPCSent should return true for iHave message after it is tracked with iHaveRPCSent", func(t *testing.T) {
		numOfMsgIds := 100
		testCases := []struct {
			topic      string
			messageIDS []string
		}{
			{channels.PushBlocks.String(), unittest.IdentifierListFixture(numOfMsgIds).Strings()},
			{channels.ReceiveApprovals.String(), unittest.IdentifierListFixture(numOfMsgIds).Strings()},
			{channels.SyncCommittee.String(), unittest.IdentifierListFixture(numOfMsgIds).Strings()},
			{channels.RequestChunks.String(), unittest.IdentifierListFixture(numOfMsgIds).Strings()},
		}
		iHaves := make([]*pb.ControlIHave, len(testCases))
		for i, testCase := range testCases {
			testCase := testCase
			iHaves[i] = &pb.ControlIHave{
				TopicID:    &testCase.topic,
				MessageIDs: testCase.messageIDS,
			}
		}
		rpc := rpcFixture(withIhaves(iHaves))
		tracker.RPCSent(rpc)

		// eventually we should have tracked numOfMsgIds per single topic
		require.Eventually(t, func() bool {
			return tracker.cache.size() == uint(len(testCases)*numOfMsgIds)
		}, time.Second, 100*time.Millisecond)

		for _, testCase := range testCases {
			for _, messageID := range testCase.messageIDS {
				require.True(t, tracker.WasIHaveRPCSent(testCase.topic, messageID))
			}
		}
	})
}

func mockTracker(t *testing.T) *RPCSentTracker {
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	tracker := NewRPCSentTracker(&RPCSentTrackerConfig{
		Logger:                    zerolog.Nop(),
		RPCSentCacheSize:          cfg.NetworkConfig.GossipSubConfig.RPCSentTrackerCacheSize,
		RPCSentCacheCollector:     metrics.NewNoopCollector(),
		WorkerQueueCacheCollector: metrics.NewNoopCollector(),
		WorkerQueueCacheSize:      cfg.NetworkConfig.GossipSubConfig.RPCSentTrackerQueueCacheSize,
		NumOfWorkers:              1,
	})
	return tracker
}

type rpcFixtureOpt func(*pubsub.RPC)

func withIhaves(iHave []*pb.ControlIHave) rpcFixtureOpt {
	return func(rpc *pubsub.RPC) {
		rpc.Control.Ihave = iHave
	}
}

func rpcFixture(opts ...rpcFixtureOpt) *pubsub.RPC {
	rpc := &pubsub.RPC{
		RPC: pb.RPC{
			Control: &pb.ControlMessage{},
		},
	}
	for _, opt := range opts {
		opt(rpc)
	}
	return rpc
}
