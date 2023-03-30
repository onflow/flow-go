package scoring_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/network/p2p/scoring"
)

// TestGeometricDecay_PositiveScore tests the GeometricDecay function with positive score.
// It tests the normal case, overflow case, and underflow case.
// The normal case is the most common case where the score is not too large or too small.
// The overflow case is when the score is too large and the decay factor is too large.
// The underflow case is when the score is too small and the decay factor is too small.
func TestGeometricDecay_PositiveScore(t *testing.T) {
	score := 10.0
	decay := 0.5
	lastUpdated := time.Now().Add(-10 * time.Minute)

	// tests normal case.
	expected := score * math.Pow(decay, time.Since(lastUpdated).Seconds())
	actual, err := scoring.GeometricDecay(score, decay, lastUpdated)
	assert.Nil(t, err)
	assert.Less(t, math.Abs(expected-actual), 1e-10)

	// tests overflow case.
	score = 1e300
	decay = 2 // although such decay is not practical, it is still valid mathematically.
	lastUpdated = time.Now().Add(-10 * time.Minute)
	actual, err = scoring.GeometricDecay(score, decay, lastUpdated)
	assert.Errorf(t, err, "decay factor is too large for %f^%f", decay, time.Since(lastUpdated).Seconds())
	assert.Equal(t, 0.0, actual)

	// tests underflow case.
	score = 1e-300
	decay = 0.5
	lastUpdated = time.Now().Add(-10 * time.Minute)
	actual, err = scoring.GeometricDecay(score, decay, lastUpdated)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, actual)
}

// TestGeometricDecay_NegativeScore tests the GeometricDecay function with negative score.
// It tests the normal case, overflow case, and underflow case.
// The normal case is the most common case where the score is not too large or too small.
// The overflow case is when the score is too large and the decay factor is too large.
// The underflow case is when the score is too small and the decay factor is too small.
func TestGeometricDecay_NegativeScore(t *testing.T) {
	score := -10.0
	decay := 0.5
	lastUpdated := time.Now().Add(-10 * time.Minute)

	// tests normal case.
	expected := score * math.Pow(decay, time.Since(lastUpdated).Seconds())
	actual, err := scoring.GeometricDecay(score, decay, lastUpdated)
	assert.Nil(t, err)
	assert.Less(t, math.Abs(expected-actual), 1e-10)

	// test overflow case with negative score.
	score = -1e300
	decay = 2
	lastUpdated = time.Now().Add(-10 * time.Minute)
	expected = math.Inf(-1)
	actual, err = scoring.GeometricDecay(score, decay, lastUpdated)
	assert.Equal(t, expected, actual)

	// test underflow case with negative score.
	score = -1e-300
	decay = 0.5
	lastUpdated = time.Now().Add(-10 * time.Minute)
	expected = 0.0
	actual, err = scoring.GeometricDecay(score, decay, lastUpdated)
	assert.Equal(t, expected, actual)
}
