package complete

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestHasEnoughSegmentsToTriggerCheckpoint(t *testing.T) {
	c := &Trigger{checkpointedSegumentNum: atomic.NewInt32(0), checkpointDistance: 3}
	require.False(t, c.hasEnoughSegmentsToTriggerCheckpoint(1))
	require.False(t, c.hasEnoughSegmentsToTriggerCheckpoint(2))
	require.True(t, c.hasEnoughSegmentsToTriggerCheckpoint(3))
	require.True(t, c.hasEnoughSegmentsToTriggerCheckpoint(4))
}

func TestNotifyTrieUpdateWrittenToWAL(t *testing.T) {
	// TODO
}
