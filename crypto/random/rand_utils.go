package random

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
)

// BasicDistributionTest is a test function to run a basic statistic test on `randf` output.
// `randf` is a function that outputs random integers.
// It partitions all outputs into `n` continuous classes and computes the distribution
// over the partition. Each class has a width of `classWidth`: first class is [0..classWidth-1],
// secons class is [classWidth..2*classWidth-1], etc..
// It computes the frequency of outputs in the `n` classes and computes the
// standard deviation of frequencies. A small standard deviation is a necessary
// condition for a uniform distribution of `randf` (though is not a guarantee of
// uniformity)
func BasicDistributionTest(t *testing.T, n uint64, classWidth uint64, randf func() (uint64, error)) {
	// sample size should ideally be a high number multiple of `n`
	// but if `n` is too small, we could use a small sample size so that the test
	// isn't too slow
	sampleSize := 1000 * n
	if n < 100 {
		sampleSize = (80000 / n) * n // highest multiple of n less than 80000
	}
	distribution := make([]float64, n)
	// populate the distribution
	for i := uint64(0); i < sampleSize; i++ {
		r, err := randf()
		require.NoError(t, err)
		if n*classWidth != 0 {
			require.Less(t, r, n*classWidth)
		}
		distribution[r/classWidth] += 1.0
	}
	EvaluateDistributionUniformity(t, distribution)
}

// EvaluateDistributionUniformity evaluates if the input distribution is close to uinform
// through a basic quick test.
// The test computes the standard deviation and checks it is small enough compared
// to the distribution mean.
func EvaluateDistributionUniformity(t *testing.T, distribution []float64) {
	tolerance := 0.05
	stdev := stat.StdDev(distribution, nil)
	mean := stat.Mean(distribution, nil)
	assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic randomness test failed: n: %d, stdev: %v, mean: %v", len(distribution), stdev, mean))
}
