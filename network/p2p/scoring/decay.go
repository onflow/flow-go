package scoring

import (
	"fmt"
	"math"
	"time"
)

// GeometricDecay returns the decayed score based on the decay factor and the time since the last update.
// The recommended decay factor is between (0, 1), however, the function does not enforce this.
// The decayed score is calculated as follows:
// score = score * decay^t where t is the time since the last update.
func GeometricDecay(score float64, decay float64, lastUpdated time.Time) (float64, error) {
	t := time.Since(lastUpdated).Seconds()
	decayFactor := math.Pow(decay, t)

	if math.IsNaN(decayFactor) {
		return 0.0, fmt.Errorf("decay factor is NaN for %f^%f", decay, t)
	}

	if math.IsInf(decayFactor, 1) {
		return 0.0, fmt.Errorf("decay factor is too large for %f^%f", decay, t)
	}

	if math.IsInf(decayFactor, -1) {
		return 0.0, fmt.Errorf("decay factor is too small for %f^%f", decay, t)
	}

	return score * decayFactor, nil
}
