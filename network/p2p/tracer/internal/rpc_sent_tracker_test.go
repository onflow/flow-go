package internal

import (
	"testing"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

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
	tracker := mockTracker(t)
	require.NotNil(t, tracker)

	t.Run("WasIHaveRPCSent should return false for iHave message Id that has not been tracked", func(t *testing.T) {
		require.False(t, tracker.WasIHaveRPCSent("topic_id", "message_id"))
	})

	t.Run("WasIHaveRPCSent should return true for iHave message after it is tracked with OnIHaveRPCSent", func(t *testing.T) {
		topicID := channels.PushBlocks.String()
		messageID := unittest.IdentifierFixture().String()
		iHaves := []*pb.ControlIHave{{
			TopicID:    &topicID,
			MessageIDs: []string{messageID},
		}}
		rpc := rpcFixture(withIhaves(iHaves))
		tracker.OnIHaveRPCSent(rpc)
		require.True(t, tracker.WasIHaveRPCSent(topicID, messageID))
	})
}

func mockTracker(t *testing.T) *RPCSentTracker {
	logger := zerolog.Nop()
	sizeLimit := uint32(100)
	collector := metrics.NewNoopCollector()
	tracker, err := NewRPCSentTracker(logger, sizeLimit, collector)
	require.NoError(t, err)
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
