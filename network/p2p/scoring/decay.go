package scoring

import (
	"fmt"
	"math"
	"time"
)

// GeometricDecay returns the decayed penalty based on the decay factor and the time since the last update.
// The recommended decay factor is between (0, 1), however, the function does not enforce this.
// The decayed penalty is calculated as follows:
// penalty = penalty * decay^t where t is the time since the last update.
// Args:
// - penalty: the penalty to be decayed.
// - decay: the decay factor, it should be in the range of (0, 1].
// - lastUpdated: the time when the penalty was last updated.
// Returns:
//   - the decayed penalty.
//   - an error if the decay factor is not in the range of (0, 1] or the decayed penalty is NaN.
//     it also returns an error if the last updated time is in the future (to avoid overflow or Inf).
//     The error is considered irrecoverable (unless the parameters can be adjusted).
func GeometricDecay(score float64, decay float64, lastUpdated time.Time) (float64, error) {
	if decay <= 0 || decay > 1 {
		return 0.0, fmt.Errorf("decay factor must be in the range [0, 1], got %f", decay)
	}

	now := time.Now()
	if lastUpdated.After(now) {
		return 0.0, fmt.Errorf("last updated time is in the future %v now: %v", lastUpdated, now)
	}

	t := time.Since(lastUpdated).Seconds()
	decayFactor := math.Pow(decay, t)

	if math.IsNaN(decayFactor) {
		return 0.0, fmt.Errorf("decay factor is NaN for %f^%f", decay, t)
	}

	return score * decayFactor, nil
}
