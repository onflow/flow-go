package timeout

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// TestConstructor tests that constructor performs needed checks and returns expected values depending on different inputs.
func TestConstructor(t *testing.T) {
	c, err := NewConfig(1200*time.Millisecond, 2000*time.Millisecond, 1.5, 3, 2000*time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, float64(1200), c.MinReplicaTimeout)
	require.Equal(t, float64(2000), c.MaxReplicaTimeout)
	require.Equal(t, float64(1.5), c.TimeoutAdjustmentFactor)
	require.Equal(t, uint64(3), c.HappyPathMaxRoundFailures)
	require.Equal(t, float64(2000), c.MaxTimeoutObjectRebroadcastInterval)

	// should not allow negative minReplicaTimeout
	c, err = NewConfig(-1200*time.Millisecond, 2000*time.Millisecond, 1.5, 3, 2000*time.Millisecond)
	require.True(t, model.IsConfigurationError(err))

	// should not allow 0 minReplicaTimeout
	c, err = NewConfig(0, 2000*time.Millisecond, 1.5, 3, 2000*time.Millisecond)
	require.True(t, model.IsConfigurationError(err))

	// should not allow maxReplicaTimeout < minReplicaTimeout
	c, err = NewConfig(1200*time.Millisecond, 1000*time.Millisecond, 1.5, 3, 2000*time.Millisecond)
	require.True(t, model.IsConfigurationError(err))

	// should not allow timeoutIncrease to be 1.0 or smaller
	c, err = NewConfig(1200*time.Millisecond, 2000*time.Millisecond, 1.0, 3, 2000*time.Millisecond)
	require.True(t, model.IsConfigurationError(err))

	// should accept only positive values for maxRebroadcastInterval
	c, err = NewConfig(1200*time.Millisecond, 2000*time.Millisecond, 1.5, 3, 0)
	require.True(t, model.IsConfigurationError(err))
	c, err = NewConfig(1200*time.Millisecond, 2000*time.Millisecond, 1.5, 3, -1000*time.Millisecond)
	require.True(t, model.IsConfigurationError(err))
}

// TestDefaultConfig tests that default config is filled with correct values.
func TestDefaultConfig(t *testing.T) {
	c := NewDefaultConfig()

	require.Equal(t, float64(3000), c.MinReplicaTimeout)
	require.Equal(t, 1.2, c.TimeoutAdjustmentFactor)
	require.Equal(t, uint64(6), c.HappyPathMaxRoundFailures)
}
