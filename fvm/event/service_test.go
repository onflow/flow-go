package event

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
)

func Test_IsServiceEvent(t *testing.T) {

	chain := flow.Mainnet.Chain()

	t.Run("correct", func(t *testing.T) {
		assert.True(t,
			IsServiceEvent(cadence.Event{
				EventType: &cadence.EventType{
					Location: common.AddressLocation{
						Address: common.BytesToAddress(chain.ServiceAddress().Bytes()),
					},
					QualifiedIdentifier: "EpochManager.EpochSetup",
				},
			}, chain),
		)
	})

	t.Run("wrong chain", func(t *testing.T) {
		assert.False(t,
			IsServiceEvent(cadence.Event{
				EventType: &cadence.EventType{
					Location: common.AddressLocation{
						Address: common.BytesToAddress(append([]byte{1, 2, 3}, chain.ServiceAddress().Bytes()...)),
					},
				},
			}, chain),
		)
	})

	t.Run("wrong type", func(t *testing.T) {
		assert.False(t,
			IsServiceEvent(cadence.Event{
				EventType: &cadence.EventType{
					Location: common.AddressLocation{
						Address: common.BytesToAddress(chain.ServiceAddress().Bytes()),
					},
					QualifiedIdentifier: "SomeContract.SomeEvent",
				},
			}, chain),
		)
	})

}
