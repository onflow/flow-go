package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/module/limiters"
)

func TestWithStreamLimit_RejectsWhenLimitReached(t *testing.T) {
	limiter, err := limiters.NewConcurrencyLimiter(1)
	require.NoError(t, err)

	h := &Handler{limiter: limiter}

	// Saturate the limiter by blocking inside Allow.
	started := make(chan struct{})
	unblock := make(chan struct{})
	go func() {
		limiter.Allow(func() {
			close(started)
			<-unblock
		})
	}()
	<-started

	// A streaming call while the limiter is full must return ResourceExhausted.
	err = h.withStreamLimit(func() error {
		t.Fatal("fn should not be called when limiter is full")
		return nil
	})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))

	close(unblock)
}
