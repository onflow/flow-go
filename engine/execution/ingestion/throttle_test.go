package ingestion

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCaughtUp(t *testing.T) {
	require.True(t, caughtUp(100, 200, 500))
	require.True(t, caughtUp(100, 100, 500))
	require.True(t, caughtUp(100, 600, 500))

	require.False(t, caughtUp(100, 601, 500))
	require.False(t, caughtUp(100, 602, 500))
}
