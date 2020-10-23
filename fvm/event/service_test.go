package event

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
)

func Test_IsServiceEvent(t *testing.T) {

	chain := flow.Mainnet.Chain()

	t.Run("correct", func(t *testing.T) {
		assert.True(t,
			IsServiceEvent(cadence.Event{
				EventType: &cadence.EventType{
					EventTypeID: cadence.CompositeTypeID{
						Location:   chain.ServiceAddress().String(),
						Identifier: "EpochManager.EpochSetup",
					},
				},
			}, chain),
		)
	})

	t.Run("wrong chain", func(t *testing.T) {
		assert.False(t,
			IsServiceEvent(cadence.Event{
				EventType: &cadence.EventType{
					EventTypeID: cadence.CompositeTypeID{
						Location:   chain.ServiceAddress().String() + "abc",
						Identifier: "EpochManager.EpochSetup",
					},
				},
			}, chain),
		)
	})

	t.Run("wrong type", func(t *testing.T) {
		assert.False(t,
			IsServiceEvent(cadence.Event{
				EventType: &cadence.EventType{
					EventTypeID: cadence.CompositeTypeID{
						Location:   chain.ServiceAddress().String(),
						Identifier: "SomeContract.SomeEvent",
					},
				},
			}, chain),
		)
	})

}
