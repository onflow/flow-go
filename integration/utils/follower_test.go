package utils

import (
	"context"
	"testing"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// TestTxFollower creates new follower with a fixed block height and stops it.
func TestTxFollower(t *testing.T) {
	// TODO(rbtz): test against a mock client, but for now we just expire
	// the context so that the followere wont be able to progress.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f, err := NewTxFollower(ctx,
		&client.Client{},
		WithBlockHeight(2),
		WithInteval(1*time.Hour),
	)
	require.NoError(t, err)
	f.Stop()
}

// TestNopTxFollower creates a new follower with a fixed block height and
// verifies that it does not block.
func TestNopTxFollower(t *testing.T) {
	// TODO(rbtz): test against a mock client, but for now we just expire
	// the context so that the followere wont be able to progress.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f, err := NewNopTxFollower(ctx,
		&client.Client{},
		WithBlockHeight(1),
		WithInteval(1*time.Hour),
	)
	require.NoError(t, err)
	unittest.AssertClosesBefore(t, f.CompleteChanByID(flowsdk.Identifier{}), 1*time.Second)
	f.Stop()
}
