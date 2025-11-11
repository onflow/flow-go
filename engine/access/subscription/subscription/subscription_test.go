package subscription

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestSubscription_SendReceive verifies ordering and delivery over the channel.
func TestSubscription_SendReceive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	sub := NewSubscription[string](1)
	assert.NotEmpty(t, sub.ID())

	messages := []string{"a", "b", "c", "d", "e"}

	// reader
	recv := make([]string, 0, len(messages))
	done := make(chan struct{})
	go func() {
		for v := range sub.Channel() {
			recv = append(recv, v)
		}
		close(done)
	}()

	for _, m := range messages {
		require.NoError(t, sub.Send(ctx, m, 100*time.Millisecond))
	}
	sub.Close()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("receiver did not finish in time")
	}

	assert.NoError(t, sub.Err())
	assert.Equal(t, messages, recv)
}

func TestSubscription_Failures(t *testing.T) {
	t.Parallel()

	t.Run("close idempotent", func(t *testing.T) {
		sub := NewSubscription[int](1)
		sub.Close()
		sub.Close()
		assert.NoError(t, sub.Err())
	})

	t.Run("close with error preserved", func(t *testing.T) {
		sub := NewSubscription[int](1)
		err := io.EOF
		sub.CloseWithError(err)
		assert.ErrorIs(t, sub.Err(), err)
		// additional close should be safe
		sub.Close()
	})

	t.Run("send after close returns error and keeps Err", func(t *testing.T) {
		sub := NewSubscription[string](1)
		sub.CloseWithError(io.ErrUnexpectedEOF)
		err := sub.Send(context.Background(), "x", 10*time.Millisecond)
		assert.Error(t, err)
		assert.ErrorIs(t, sub.Err(), io.ErrUnexpectedEOF)
	})
}

func TestNewFailedSubscription_PreservesGRPCStatus(t *testing.T) {
	t.Parallel()

	base := status.Error(codes.InvalidArgument, "bad input")
	sub := NewFailedSubscription[int](base, "wrap msg")

	err := sub.Err()
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected grpc status error, got: %v", err)
	}
	assert.Equal(t, codes.InvalidArgument, st.Code())
	// message should include our wrap text and original message
	assert.Contains(t, st.Message(), "wrap msg")
	assert.Contains(t, st.Message(), "bad input")
}
