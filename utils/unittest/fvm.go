package unittest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// EnsureEventsIndexSeq checks if values of given event index sequence are monotonically increasing.
func EnsureEventsIndexSeq(t *testing.T, events []flow.Event, chainID flow.ChainID) {
	expectedEventIndex := uint32(0)
	for _, event := range events {
		require.Equal(t, expectedEventIndex, event.EventIndex)
		expectedEventIndex++
	}
}
