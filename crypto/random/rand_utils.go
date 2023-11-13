package random

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
)

// this constant should be increased if tests are flakey, but it the higher the constant
// the slower the test
const sampleSizeConstant = 85000
const sampleCoefficient = sampleSizeConstant / 85

// BasicDistributionTest is a test function to run a basic statistic test on `randf` output.
// `randf` is a function that outputs random integers.
// It partitions all outputs into `n` continuous classes and computes the distribution
// over the partition. Each class has a width of `classWidth`: first class is [0..classWidth-1],
// second class is [classWidth..2*classWidth-1], etc..
// It computes the frequency of outputs in the `n` classes and computes the
// standard deviation of frequencies. A small standard deviation is a necessary
// condition for a uniform distribution of `randf` (but is not a guarantee of
// uniformity)
func BasicDistributionTest(t *testing.T, n uint64, classWidth uint64, randf func() (uint64, error)) {
	// sample size should ideally be a high number multiple of `n`
	sampleSize := sampleCoefficient * n
	if n < 80 {
		// but if `n` is too small, we use a "high enough" sample size
		sampleSize = ((sampleSizeConstant) / n) * n // highest multiple of n less than 80000
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

// EvaluateDistributionUniformity evaluates if the input distribution is close to uniform
// through a basic quick test.
// The test computes the standard deviation and checks it is small enough compared
// to the distribution mean.
func EvaluateDistributionUniformity(t *testing.T, distribution []float64) {
	tolerance := 0.05
	stdev := stat.StdDev(distribution, nil)
	mean := stat.Mean(distribution, nil)
	assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic randomness test failed: n: %d, stdev: %v, mean: %v", len(distribution), stdev, mean))
}

// computes a bijection from the set of all permutations
// into the the set [0, n!-1] (where `n` is the size of input `perm`).
// input `perm` is assumed to be a correct permutation of the set [0,n-1]
// (not checked in this function).
func EncodePermutation(perm []int) int {
	r := make([]int, len(perm))
	// generate Lehmer code
	// (for details https://en.wikipedia.org/wiki/Lehmer_code)
	for i, x := range perm {
		for _, y := range perm[i+1:] {
			if y < x {
				r[i]++
			}
		}
	}
	// Convert to an integer following the factorial number system
	// (for details https://en.wikipedia.org/wiki/Factorial_number_system)
	m := 0
	fact := 1
	for i := len(perm) - 1; i >= 0; i-- {
		m += r[i] * fact
		fact *= len(perm) - i
	}
	return m
}
