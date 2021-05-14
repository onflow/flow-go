package convert

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventConversion(t *testing.T) {

	t.Run("epoch setup", func(t *testing.T) {

		fixture, expected := epochSetupFixture()

		// convert Cadence types to Go types
		event, err := ServiceEvent(fixture)
		require.NoError(t, err)
		require.NotNil(t, event)

		// cast event type to epoch setup
		actual, ok := event.Event.(*flow.EpochSetup)
		require.True(t, ok)

		assert.Equal(t, expected, actual)

	})

	t.Run("epoch commit", func(t *testing.T) {

		fixture, expected := epochCommitFixture()

		// convert Cadence types to Go types
		event, err := ServiceEvent(fixture)
		require.NoError(t, err)
		require.NotNil(t, event)

		// cast event type to epoch commit
		actual, ok := event.Event.(*flow.EpochCommit)
		require.True(t, ok)

		fmt.Println(actual.ClusterQCs[0].SigData.String())
		fmt.Println(actual.ClusterQCs[1].SigData.String())

		assert.Equal(t, expected, actual)
	})
}
