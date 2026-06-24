package internal

import (
	"context"
	"os"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewRPCSentTracker ensures *RPCSenTracker is created as expected.
func TestNewRPCSentTracker(t *testing.T) {
	tracker := mockTracker(t, time.Minute)
	require.NotNil(t, tracker)
}

// TestRPCSentTracker_IHave ensures *RPCSentTracker tracks sent iHave control messages as expected.
func TestRPCSentTracker_IHave(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	tracker := mockTracker(t, time.Minute)
	require.NotNil(t, tracker)

	tracker.Start(signalerCtx)
	defer func() {
		cancel()
		unittest.RequireComponentsDoneBefore(t, time.Second, tracker)
	}()

	t.Run("WasIHaveRPCSent should return false for iHave message Id that has not been tracked", func(t *testing.T) {
		require.False(t, tracker.WasIHaveRPCSent("message_id"))
	})

	t.Run("WasIHaveRPCSent should return true for iHave message after it is tracked with iHaveRPCSent", func(t *testing.T) {
		numOfMsgIds := 100
		testCases := []struct {
			messageIDS []string
		}{
			{unittest.IdentifierListFixture(numOfMsgIds).Strings()},
			{unittest.IdentifierListFixture(numOfMsgIds).Strings()},
			{unittest.IdentifierListFixture(numOfMsgIds).Strings()},
			{unittest.IdentifierListFixture(numOfMsgIds).Strings()},
		}
		iHaves := make([]*pb.ControlIHave, len(testCases))
		for i, testCase := range testCases {
			testCase := testCase
			iHaves[i] = &pb.ControlIHave{
				MessageIDs: testCase.messageIDS,
			}
		}
		rpc := rpcFixture(withIhaves(iHaves))
		require.NoError(t, tracker.Track(rpc))

		// eventually we should have tracked numOfMsgIds per single topic
		require.Eventually(t, func() bool {
			return tracker.cache.size() == uint(len(testCases)*numOfMsgIds)
		}, time.Second, 100*time.Millisecond)

		for _, testCase := range testCases {
			for _, messageID := range testCase.messageIDS {
				require.True(t, tracker.WasIHaveRPCSent(messageID))
			}
		}
	})

}

// TestRPCSentTracker_DuplicateMessageID ensures the worker pool of the RPC tracker processes req with the same message ID but different nonce.
func TestRPCSentTracker_DuplicateMessageID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	processedWorkLogs := atomic.NewInt64(0)
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.DebugLevel {
			if message == iHaveRPCTrackedLog {
				processedWorkLogs.Inc()
			}
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).Hook(hook)

	tracker := mockTracker(t, time.Minute)
	require.NotNil(t, tracker)
	tracker.logger = logger
	tracker.Start(signalerCtx)
	defer func() {
		cancel()
		unittest.RequireComponentsDoneBefore(t, time.Second, tracker)
	}()

	messageID := unittest.IdentifierFixture().String()
	rpc := rpcFixture(withIhaves([]*pb.ControlIHave{{
		MessageIDs: []string{messageID},
	}}))
	// track duplicate RPC's each will be processed by a worker
	require.NoError(t, tracker.Track(rpc))
	require.NoError(t, tracker.Track(rpc))

	// eventually we should have processed both RPCs
	require.Eventually(t, func() bool {
		return processedWorkLogs.Load() == 2
	}, time.Second, 100*time.Millisecond)
}

// TestRPCSentTracker_ConcurrentTracking ensures that all message IDs in RPC's are tracked as expected when tracked concurrently.
func TestRPCSentTracker_ConcurrentTracking(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	tracker := mockTracker(t, time.Minute)
	require.NotNil(t, tracker)

	tracker.Start(signalerCtx)
	defer func() {
		cancel()
		unittest.RequireComponentsDoneBefore(t, time.Second, tracker)
	}()

	numOfMsgIds := 100
	numOfRPCs := 100
	rpcs := make([]*pubsub.RPC, numOfRPCs)
	for i := 0; i < numOfRPCs; i++ {
		i := i
		go func() {
			rpc := rpcFixture(withIhaves([]*pb.ControlIHave{{MessageIDs: unittest.IdentifierListFixture(numOfMsgIds).Strings()}}))
			require.NoError(t, tracker.Track(rpc))
			rpcs[i] = rpc
		}()
	}

	// eventually we should have tracked numOfMsgIds per single topic
	require.Eventually(t, func() bool {
		return tracker.cache.size() == uint(numOfRPCs*numOfMsgIds)
	}, time.Second, 100*time.Millisecond)

	for _, rpc := range rpcs {
		ihaves := rpc.GetControl().GetIhave()
		for _, messageID := range ihaves[0].GetMessageIDs() {
			require.True(t, tracker.WasIHaveRPCSent(messageID))
		}
	}
}

// TestRPCSentTracker_IHave ensures *RPCSentTracker tracks the last largest iHave size as expected.
func TestRPCSentTracker_LastHighestIHaveRPCSize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	tracker := mockTracker(t, 3*time.Second)
	require.NotNil(t, tracker)

	tracker.Start(signalerCtx)
	defer func() {
		cancel()
		unittest.RequireComponentsDoneBefore(t, time.Second, tracker)
	}()

	expectedLastHighestSize := 1000
	// adding a single message ID to the iHave enables us to track the expected cache size by the amount of iHaves.
	numOfMessageIds := 1
	testCases := []struct {
		rpcFixture  *pubsub.RPC
		numOfIhaves int
	}{
		{rpcFixture(withIhaves(mockIHaveFixture(10, numOfMessageIds))), 10},
		{rpcFixture(withIhaves(mockIHaveFixture(100, numOfMessageIds))), 100},
		{rpcFixture(withIhaves(mockIHaveFixture(expectedLastHighestSize, numOfMessageIds))), expectedLastHighestSize},
		{rpcFixture(withIhaves(mockIHaveFixture(999, numOfMessageIds))), 999},
		{rpcFixture(withIhaves(mockIHaveFixture(23, numOfMessageIds))), 23},
	}

	expectedCacheSize := 0
	for _, testCase := range testCases {
		require.NoError(t, tracker.Track(testCase.rpcFixture))
		expectedCacheSize += testCase.numOfIhaves
	}

	// eventually we should have tracked numOfMsgIds per single topic
	require.Eventually(t, func() bool {
		return tracker.cache.size() == uint(expectedCacheSize)
	}, time.Second, 100*time.Millisecond)

	require.Equal(t, int64(expectedLastHighestSize), tracker.LastHighestIHaveRPCSize())

	// after setting sending large RPC lastHighestIHaveRPCSize should reset to 0 after lastHighestIHaveRPCSize reset loop tick
	largeIhave := 50000
	require.NoError(t, tracker.Track(rpcFixture(withIhaves(mockIHaveFixture(largeIhave, numOfMessageIds)))))
	require.Eventually(t, func() bool {
		return tracker.LastHighestIHaveRPCSize() == int64(largeIhave)
	}, 1*time.Second, 100*time.Millisecond)

	// we expect lastHighestIHaveRPCSize to be set to the current rpc size being tracked if it hasn't been updated since the configured lastHighestIHaveRPCSizeResetInterval
	expectedEventualLastHighest := 8
	require.Eventually(t, func() bool {
		require.NoError(t, tracker.Track(rpcFixture(withIhaves(mockIHaveFixture(expectedEventualLastHighest, numOfMessageIds)))))
		return tracker.LastHighestIHaveRPCSize() == int64(expectedEventualLastHighest)
	}, 4*time.Second, 100*time.Millisecond)
}

// mockIHaveFixture generate list of iHaves of size n. Each iHave will be created with m number of random message ids.
func mockIHaveFixture(n, m int) []*pb.ControlIHave {
	iHaves := make([]*pb.ControlIHave, n)
	for i := 0; i < n; i++ {
		// topic does not have to be a valid flow topic, for teting purposes we can use a random string
		topic := unittest.IdentifierFixture().String()
		iHaves[i] = &pb.ControlIHave{
			TopicID:    &topic,
			MessageIDs: unittest.IdentifierListFixture(m).Strings(),
		}
	}
	return iHaves
}

func mockTracker(t *testing.T, lastHighestIhavesSentResetInterval time.Duration) *RPCSentTracker {
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	tracker := NewRPCSentTracker(&RPCSentTrackerConfig{
		Logger:                             zerolog.Nop(),
		RPCSentCacheSize:                   cfg.NetworkConfig.GossipSub.RpcTracer.RPCSentTrackerCacheSize,
		RPCSentCacheCollector:              metrics.NewNoopCollector(),
		WorkerQueueCacheCollector:          metrics.NewNoopCollector(),
		WorkerQueueCacheSize:               cfg.NetworkConfig.GossipSub.RpcTracer.RPCSentTrackerQueueCacheSize,
		NumOfWorkers:                       1,
		LastHighestIhavesSentResetInterval: lastHighestIhavesSentResetInterval,
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
