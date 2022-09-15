package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/convert"
	"github.com/onflow/flow-go/model/convert/fixtures"
	"github.com/onflow/flow-go/model/flow"
)

func TestEventConversion(t *testing.T) {

	chainID := flow.Emulator

	t.Run("epoch setup", func(t *testing.T) {

		fixture, expected := fixtures.EpochSetupFixtureByChainID(chainID)

		// convert Cadence types to Go types
		event, err := convert.ServiceEvent(chainID, fixture)
		require.NoError(t, err)
		require.NotNil(t, event)

		// cast event type to epoch setup
		actual, ok := event.Event.(*flow.EpochSetup)
		require.True(t, ok)

		assert.Equal(t, expected, actual)

	})

	t.Run("epoch commit", func(t *testing.T) {

		fixture, expected := fixtures.EpochCommitFixtureByChainID(chainID)

		// convert Cadence types to Go types
		event, err := convert.ServiceEvent(chainID, fixture)
		require.NoError(t, err)
		require.NotNil(t, event)

		// cast event type to epoch commit
		actual, ok := event.Event.(*flow.EpochCommit)
		require.True(t, ok)

		assert.Equal(t, expected, actual)
	})
}
