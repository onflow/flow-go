package utils

import (
	"context"
	"testing"
	"time"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/stretchr/testify/require"
)

// TestFollower creates new follower with a random block height and stops it.
// TODO(rbtz): test against a mock client
func TestFollower(t *testing.T) {
	f, err := NewTxFollower(context.Background(),
		&client.Client{},
		WithBlockHeight(2),
		WithInteval(1*time.Hour),
	)
	require.NoError(t, err)
	f.Stop()
}
