package scoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

// TestAfterSilencePeriod verifies the state machine of the scoring registry's startup silence
// period, in particular that the silence period does not end before the registry has started.
// Regression: the start time used to be a plain time.Time (a data race with the startup worker),
// and its zero value made time.Since(zero) exceed any configured duration, spuriously ending the
// silence period if the score function was queried before startup.
func TestAfterSilencePeriod(t *testing.T) {
	reg := &GossipSubAppSpecificScoreRegistry{
		silencePeriodDuration:  time.Hour,
		silencePeriodStartTime: atomic.NewPointer[time.Time](nil),
		silencePeriodElapsed:   atomic.NewBool(false),
	}

	// before startup (start time not set), the silence period has not even begun
	require.False(t, reg.afterSilencePeriod())

	// silence period started, but not yet over
	now := time.Now()
	reg.silencePeriodStartTime.Store(&now)
	require.False(t, reg.afterSilencePeriod())

	// silence period over
	past := time.Now().Add(-2 * time.Hour)
	reg.silencePeriodStartTime.Store(&past)
	require.True(t, reg.afterSilencePeriod())

	// once elapsed, the silence period stays elapsed
	require.True(t, reg.silencePeriodElapsed.Load())
	require.True(t, reg.afterSilencePeriod())
}
