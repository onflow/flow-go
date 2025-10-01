package unittest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func IsServiceEvent(event flow.Event, chainID flow.ChainID) bool {
	serviceEvents := systemcontracts.ServiceEventsForChain(chainID)
	for _, serviceEvent := range serviceEvents.All() {
		if serviceEvent.EventType() == event.Type {
			return true
		}
	}
	return false
}

// EnsureEventsIndexSeq checks if values of given event index sequence are monotonically increasing.
func EnsureEventsIndexSeq(t *testing.T, events []flow.Event, chainID flow.ChainID) {
	expectedEventIndex := uint32(0)
	for _, event := range events {
		if expectedEventIndex != event.EventIndex {
			require.Equal(t, expectedEventIndex, event.EventIndex)
		}
		require.Equal(t, expectedEventIndex, event.EventIndex)
		expectedEventIndex++
	}
}
