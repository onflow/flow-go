package rand

import (
	"fmt"
	"math"
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
)

// Evaluate if the input distribution is close to uinform through a basic quick test.
// The test computes the standard deviation and checks it is small enough compared
// to the distribution mean.
func evaluateDistributionUniformity(t *testing.T, distribution []float64) {
	tolerance := 0.05
	stdev := stat.StdDev(distribution, nil)
	mean := stat.Mean(distribution, nil)
	assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic randomness test failed. stdev %v, mean %v", stdev, mean))
}

// Simple unit tests using a very basic randomness test.
// It doesn't evaluate randomness of the output and doesn't perform advanced statistical tests.
func TestRandomIntegers(t *testing.T) {
	t.Run("basic randomness", func(t *testing.T) {
		sampleSize := 80000
		n := 10 + mrand.Intn(100)
		distribution := make([]float64, n)

		// generic test function to run a basic statistic test on `randf` output.
		// It partitions all outputs into `n` classes and compute the distribution
		// over the partition. Each class has a width of `classWidth`
		// It computes the frequency of outputs in the `n` classes and computes the
		// standard deviation of frequencies. A small standard deviation is a necessary
		// condition for a uniform distribution of `randf` (though is not a guarantee of
		// uniformity)
		basicDistributionTest := func(t *testing.T, classWidth uint64, randf func() (uint64, error)) {
			// populate the distribution
			for i := 0; i < sampleSize; i++ {
				r, err := randf()
				require.NoError(t, err)
				distribution[r/classWidth] += 1.0
			}
			evaluateDistributionUniformity(t, distribution)
		}

		t.Run("Uint", func(t *testing.T) {
			classWidth := math.MaxUint / uint(n)
			uintf := func() (uint64, error) {
				r, err := Uint()
				return uint64(r), err
			}
			basicDistributionTest(t, uint64(classWidth), uintf)
		})

		t.Run("Uint64", func(t *testing.T) {
			classWidth := math.MaxUint64 / uint64(n)
			basicDistributionTest(t, uint64(classWidth), Uint64)
		})

		t.Run("Uint32", func(t *testing.T) {
			classWidth := math.MaxUint32 / uint32(n)
			uintf := func() (uint64, error) {
				r, err := Uint32()
				return uint64(r), err
			}
			basicDistributionTest(t, uint64(classWidth), uintf)
		})

		t.Run("Uintn", func(t *testing.T) {
			uintf := func() (uint64, error) {
				r, err := Uintn(uint(n))
				return uint64(r), err
			}
			// classWidth is 1 since `n` is small
			basicDistributionTest(t, uint64(1), uintf)
		})

		t.Run("Uint64n", func(t *testing.T) {
			uintf := func() (uint64, error) {
				return Uint64n(uint64(n))
			}
			// classWidth is 1 since `n` is small
			basicDistributionTest(t, uint64(1), uintf)
		})

		t.Run("Uint32n", func(t *testing.T) {
			uintf := func() (uint64, error) {
				r, err := Uint32n(uint32(n))
				return uint64(r), err
			}
			// classWidth is 1 since `n` is small
			basicDistributionTest(t, uint64(1), uintf)
		})
	})

	t.Run("zero n error", func(t *testing.T) {
		t.Run("Uintn", func(t *testing.T) {
			_, err := Uintn(uint(0))
			require.Error(t, err)
		})
		t.Run("Uint64n", func(t *testing.T) {
			_, err := Uint64n(uint64(0))
			require.Error(t, err)
		})
		t.Run("Uint32n", func(t *testing.T) {
			_, err := Uint32n(uint32(0))
			require.Error(t, err)
		})
	})
}

// Simple unit testing of Shuffle using a basic randomness test.
// It doesn't evaluate randomness of the output and doesn't perform advanced statistical tests.
func TestShuffle(t *testing.T) {
	t.Run("basic randomness", func(t *testing.T) {
		// test parameters
		listSize := 100
		sampleSize := 80000
		// the distribution of a particular random element of the list, testElement
		distribution := make([]float64, listSize)
		testElement := mrand.Intn(listSize)
		// Slice to shuffle
		list := make([]int, listSize)

		// shuffles the slice and counts the frequency of the test element
		// in each position
		shuffleAndCount := func(t *testing.T) {
			err := Shuffle(uint(listSize), func(i, j uint) {
				list[i], list[j] = list[j], list[i]
			})
			require.NoError(t, err)
			has := make(map[int]struct{})
			for j, e := range list {
				// check for repetition
				_, ok := has[e]
				require.False(t, ok, "duplicated item")
				has[e] = struct{}{}
				// increment the frequency distribution in position `j`
				if e == testElement {
					distribution[j] += 1.0
				}
			}
		}

		t.Run("shuffle a random permutation", func(t *testing.T) {
			// initialize the list
			for i := 0; i < listSize; i++ {
				list[i] = i
			}
			// shuffle and count multiple times
			for k := 0; k < sampleSize; k++ {
				shuffleAndCount(t)
			}
			// if the shuffle is uniform, the test element
			// should end up uniformly in all positions of the slice
			evaluateDistributionUniformity(t, distribution)
		})

		t.Run("shuffle a same permutation", func(t *testing.T) {
			for k := 0; k < sampleSize; k++ {
				for i := 0; i < listSize; i++ {
					list[i] = i
				}
				// suffle the same permutation
				shuffleAndCount(t)
			}
			// if the shuffle is uniform, the test element
			// should end up uniformly in all positions of the slice
			evaluateDistributionUniformity(t, distribution)
		})
	})

	t.Run("empty slice", func(t *testing.T) {
		emptySlice := make([]float64, 0)
		err := Shuffle(0, func(i, j uint) {
			emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
		})
		require.NoError(t, err)
		assert.True(t, len(emptySlice) == 0)
	})
}

func TestSamples(t *testing.T) {
	t.Run("basic randmoness", func(t *testing.T) {
		listSize := 100
		samplesSize := 20
		sampleSize := 100000
		// tests the subset sampling randomness
		samplingDistribution := make([]float64, listSize)
		// tests the subset ordering randomness (using a particular element testElement)
		orderingDistribution := make([]float64, samplesSize)
		testElement := mrand.Intn(listSize)
		// Slice to shuffle
		list := make([]int, 0, listSize)
		for i := 0; i < listSize; i++ {
			list = append(list, i)
		}

		for i := 0; i < sampleSize; i++ {
			err := Samples(uint(listSize), uint(samplesSize), func(i, j uint) {
				list[i], list[j] = list[j], list[i]
			})
			require.NoError(t, err)
			has := make(map[int]struct{})
			for j, e := range list[:samplesSize] {
				// check for repetition
				_, ok := has[e]
				require.False(t, ok, "duplicated item")
				has[e] = struct{}{}
				// fill the distribution
				samplingDistribution[e] += 1.0
				if e == testElement {
					orderingDistribution[j] += 1.0
				}
			}
		}
		// if the sampling is uniform, all elements
		// should end up being sampled an equivalent number of times
		evaluateDistributionUniformity(t, samplingDistribution)
		// if the sampling is uniform, the test element
		// should end up uniformly in all positions of the sample slice
		evaluateDistributionUniformity(t, orderingDistribution)
	})

	t.Run("zero edge cases", func(t *testing.T) {
		// Sampling from an empty set
		emptySlice := make([]float64, 0)
		err := Samples(0, 0, func(i, j uint) {
			emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
		})
		require.NoError(t, err)
		assert.True(t, len(emptySlice) == 0)

		// drawing a sample of size zero from an non-empty list should leave the original list unmodified
		constant := []float64{0, 1, 2, 3, 4, 5}
		fullSlice := constant
		err = Samples(uint(len(fullSlice)), 0, func(i, j uint) { // modifies fullSlice in-place
			emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
		})
		require.NoError(t, err)
		assert.Equal(t, constant, fullSlice)
	})
}
