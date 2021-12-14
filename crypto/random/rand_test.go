package random

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"math/rand"
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
)

// TODO: update the file name

// The tests are targeting the PRG implementations in the package.
// For now, the tests are only used for Chacha20 PRG, but can be ported
// to test another PRG implementation.

// Simple unit testing of Uint using a very basic randomness test.
// It doesn't evaluate randomness of the output and doesn't perform advanced statistical tests.
func TestUint(t *testing.T) {

	seed := make([]byte, Chacha20SeedLen)
	crand.Read(seed)
	customizer := make([]byte, Chacha20CustomizerMaxLen)
	crand.Read(customizer)

	rng, err := NewChacha20PRG(seed, customizer)
	require.NoError(t, err)

	t.Run("basic randomness", func(t *testing.T) {
		sampleSize := 100000
		tolerance := 0.05
		sampleSpace := uint64(10 + mrand.Intn(100))
		distribution := make([]float64, sampleSpace)

		for i := 0; i < sampleSize; i++ {
			r := rng.UintN(sampleSpace)
			require.Less(t, r, sampleSpace)
			distribution[r] += 1.0
		}
		stdev := stat.StdDev(distribution, nil)
		mean := stat.Mean(distribution, nil)
		assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic randomness test failed. stdev %v, mean %v", stdev, mean))
	})

	t.Run("zero n", func(t *testing.T) {
		assert.Panics(t, func() {
			rng.UintN(0)
		})
	})
}

// Simple unit testing of SubPermutation using a very basic randomness test.
// It doesn't evaluate randomness of the output and doesn't perform advanced statistical tests.
//
// SubPermutation tests cover Permutation as well.
func TestSubPermutation(t *testing.T) {

	seed := make([]byte, Chacha20SeedLen)
	crand.Read(seed)
	customizer := make([]byte, Chacha20CustomizerMaxLen)
	crand.Read(customizer)

	rng, err := NewChacha20PRG(seed, customizer)
	require.NoError(t, err)

	t.Run("basic randomness", func(t *testing.T) {

		listSize := 100
		subsetSize := 20

		// statictics parameters
		sampleSize := 100000
		tolerance := 0.05
		// tests the subset sampling randomness
		samplingDistribution := make([]float64, listSize)
		// tests the subset ordering randomness (using a particular element testElement)
		orderingDistribution := make([]float64, subsetSize)
		testElement := rand.Intn(listSize)

		for i := 0; i < sampleSize; i++ {
			shuffledlist, err := rng.SubPermutation(listSize, subsetSize)
			require.NoError(t, err)
			if len(shuffledlist) != subsetSize {
				t.Errorf("PermutateSubset returned a list with a wrong size")
			}
			has := make(map[int]struct{})
			for j, e := range shuffledlist {
				// check for repetition
				if _, ok := has[e]; ok {
					t.Errorf("dupplicated item in the results returned by PermutateSubset")
				}
				has[e] = struct{}{}
				// fill the distribution
				samplingDistribution[e] += 1.0
				if e == testElement {
					orderingDistribution[j] += 1.0
				}
			}
		}
		stdev := stat.StdDev(samplingDistribution, nil)
		mean := stat.Mean(samplingDistribution, nil)
		assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic subset randomness test failed. stdev %v, mean %v", stdev, mean))
		stdev = stat.StdDev(orderingDistribution, nil)
		mean = stat.Mean(orderingDistribution, nil)
		assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic ordering randomness test failed. stdev %v, mean %v", stdev, mean))
	})

	// Evaluate that
	//  - permuting an empty set returns an empty list
	//  - drawing a sample of size zero from a non-empty set returns an empty list
	t.Run("empty sets", func(t *testing.T) {

		// verify that permuting an empty set returns an empty list
		res, err := rng.SubPermutation(0, 0)
		require.NoError(t, err)
		assert.True(t, len(res) == 0)

		// verify that drawing a sample of size zero from a non-empty set returns an empty list
		res, err = rng.SubPermutation(10, 0)
		require.NoError(t, err)
		assert.True(t, len(res) == 0)
	})

	t.Run("negative inputs", func(t *testing.T) {
		res, err := rng.Permutation(-3)
		require.Error(t, err)
		assert.Nil(t, res)

		res, err = rng.SubPermutation(5, -3)
		require.Error(t, err)
		assert.Nil(t, res)

		res, err = rng.SubPermutation(-3, 5)
		require.Error(t, err)
		assert.Nil(t, res)
	})
}

// Simple unit testing of Shuffle using a very basic randomness test.
// It doesn't evaluate randomness of the output and doesn't perform advanced statistical tests.
func TestShuffle(t *testing.T) {

	seed := make([]byte, Chacha20SeedLen)
	crand.Read(seed)
	customizer := make([]byte, Chacha20CustomizerMaxLen)
	crand.Read(customizer)

	rng, err := NewChacha20PRG(seed, customizer)
	require.NoError(t, err)

	t.Run("basic randmoness", func(t *testing.T) {
		listSize := 100
		// test parameters
		sampleSize := 100000
		tolerance := 0.05
		// the distribution of a particular element of the list, testElement
		distribution := make([]float64, listSize)
		testElement := rand.Intn(listSize)
		// Slice to shuffle
		list := make([]int, 0, listSize)
		for i := 0; i < listSize; i++ {
			list = append(list, i)
		}

		for i := 0; i < sampleSize; i++ {
			err = rng.Shuffle(listSize, func(i, j int) {
				list[i], list[j] = list[j], list[i]
			})
			require.NoError(t, err)
			has := make(map[int]struct{})
			for j, e := range list {
				// check for repetition
				if _, ok := has[e]; ok {
					t.Errorf("dupplicated item in the results returned by PermutateSubset")
				}
				has[e] = struct{}{}
				// fill the distribution
				if e == testElement {
					distribution[j] += 1.0
				}
			}
		}
		stdev := stat.StdDev(distribution, nil)
		mean := stat.Mean(distribution, nil)
		assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic randomness test failed. stdev %v, mean %v", stdev, mean))
	})

	t.Run("empty slice", func(t *testing.T) {
		emptySlice := make([]float64, 0)
		err = rng.Shuffle(len(emptySlice), func(i, j int) {
			emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
		})
		require.NoError(t, err)
		assert.True(t, len(emptySlice) == 0)
	})

	t.Run("negative inputs", func(t *testing.T) {
		emptySlice := make([]float64, 5)
		err = rng.Shuffle(-3, func(i, j int) {
			emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
		})
		require.Error(t, err)
	})
}

func TestSamples(t *testing.T) {

	seed := make([]byte, Chacha20SeedLen)
	crand.Read(seed)
	customizer := make([]byte, Chacha20CustomizerMaxLen)
	crand.Read(customizer)

	rng, err := NewChacha20PRG(seed, customizer)
	require.NoError(t, err)

	t.Run("basic randmoness", func(t *testing.T) {

		listSize := 100
		samplesSize := 20

		// statictics parameters
		sampleSize := 100000
		tolerance := 0.05
		// tests the subset sampling randomness
		samplingDistribution := make([]float64, listSize)
		// tests the subset ordering randomness (using a particular element testElement)
		orderingDistribution := make([]float64, samplesSize)
		testElement := rand.Intn(listSize)
		// Slice to shuffle
		list := make([]int, 0, listSize)
		for i := 0; i < listSize; i++ {
			list = append(list, i)
		}

		for i := 0; i < sampleSize; i++ {
			err = rng.Samples(listSize, samplesSize, func(i, j int) {
				list[i], list[j] = list[j], list[i]
			})
			require.NoError(t, err)
			has := make(map[int]struct{})
			for j, e := range list[:samplesSize] {
				// check for repetition
				if _, ok := has[e]; ok {
					t.Errorf("dupplicated item in the results returned by PermutateSubset")
				}
				has[e] = struct{}{}
				// fill the distribution
				samplingDistribution[e] += 1.0
				if e == testElement {
					orderingDistribution[j] += 1.0
				}
			}
		}
		stdev := stat.StdDev(samplingDistribution, nil)
		mean := stat.Mean(samplingDistribution, nil)
		assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic subset randomness test failed. stdev %v, mean %v", stdev, mean))
		stdev = stat.StdDev(orderingDistribution, nil)
		mean = stat.Mean(orderingDistribution, nil)
		assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic ordering randomness test failed. stdev %v, mean %v", stdev, mean))
	})

	t.Run("zero edge cases", func(t *testing.T) {
		// Sampling from an empty set
		emptySlice := make([]float64, 0)
		err = rng.Samples(len(emptySlice), len(emptySlice), func(i, j int) {
			emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
		})
		require.NoError(t, err)
		assert.True(t, len(emptySlice) == 0)

		// drawing a sample of size zero from an non-empty list should leave the original list unmodified
		constant := []float64{0, 1, 2, 3, 4, 5}
		fullSlice := constant
		err = rng.Samples(len(fullSlice), 0, func(i, j int) { // modifies fullSlice in-place
			emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
		})
		require.NoError(t, err)
		assert.Equal(t, constant, fullSlice)
	})

	t.Run("negative inputs", func(t *testing.T) {
		emptySlice := make([]float64, 5)
		err = rng.Samples(-3, 5, func(i, j int) {
			emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
		})
		require.Error(t, err)

		err = rng.Samples(-5, 3, func(i, j int) {
			emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
		})
		require.Error(t, err)
	})
}

// TestStateRestore tests the serilaization and deserialization functions
// Store and Restore
func TestStateRestore(t *testing.T) {
	// generate a seed
	seed := make([]byte, Chacha20SeedLen)
	crand.Read(seed)
	customizer := make([]byte, Chacha20CustomizerMaxLen)
	crand.Read(customizer)
	t.Logf("seed is %x, customizer is %x\n", seed, customizer)

	// create an rng
	rng, err := NewChacha20PRG(seed, customizer)
	require.NoError(t, err)

	// evolve the internal state of the rng
	iterations := rand.Intn(1000)
	for i := 0; i < iterations; i++ {
		_ = rng.UintN(1024)
	}
	// get the internal state of the rng
	state := rng.Store()

	// check the state is deterministic
	state_clone := rng.Store()
	require.True(t, bytes.Equal(state, state_clone), "Store is not deterministic")

	// check Store is the Restore reverse function
	secondRng, err := RestoreChacha20PRG(state)
	require.NoError(t, err)
	require.True(t, bytes.Equal(state, secondRng.Store()), "Store o Restore is not identity")

	// check the 2 PRGs are generating identical outputs
	iterations = rand.Intn(1000)
	for i := 0; i < iterations; i++ {
		rand1 := rng.UintN(1024)
		rand2 := secondRng.UintN(1024)
		require.Equal(t, rand1, rand2, "the 2 rngs are not identical on round %d", i)
	}
}

// sanity check for the underlying implementation of Chacha20
// to make sure the implementation is following the RFC 7539
func TestChacha20Constants(t *testing.T) {
	assert.Equal(t, Chacha20SeedLen, 32)
	assert.Equal(t, Chacha20CustomizerMaxLen, 12)
}

// TODO: add test vectors
