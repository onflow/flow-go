package random

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
)

// TODO: update the file name

// math/random is only used to randomize test inputs

// The only purpose of this function is unit testing. It also implements a very basic randomness test.
// it doesn't evaluate randomness of the random function and doesn't perform advanced statistical tests
// just making sure code works on edge cases
func TestUint(t *testing.T) {
	sampleSize := 64768
	tolerance := 0.05
	sampleSpace := uint64(16) // this should be a power of 2 for a more uniform distribution
	distribution := make([]float64, sampleSpace)

	seed := make([]byte, 32)
	crand.Read(seed)
	streamID := make([]byte, 12)

	rng, err := NewChacha20(seed, streamID)
	require.NoError(t, err)
	for i := 0; i < sampleSize; i++ {
		r := rng.UintN(sampleSpace)
		require.Less(t, r, sampleSpace)
		distribution[r] += 1.0
	}
	stdev := stat.StdDev(distribution, nil)
	mean := stat.Mean(distribution, nil)
	assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic randomness test failed. stdev %v, mean %v", stdev, mean))
}

// The only purpose of this function is unit testing. It also implements a very basic tandomness test.
// it doesn't evaluate randomness of the random function and doesn't perform advanced statistical tests
// just making sure code works on edge cases
func TestRandomPermutationSubset(t *testing.T) {
	listSize := 100
	subsetSize := 20
	seed := make([]byte, 32)
	streamID := make([]byte, 12)
	// test a zero seed
	_, err := NewChacha20(seed, streamID)
	require.NoError(t, err)
	// fix thee seed
	seed[0] = 45
	rng, err := NewChacha20(seed, streamID)
	require.NoError(t, err)
	// statictics parameters
	sampleSize := 64768
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
}

// TestEmptyPermutationSubset evaluates that
//  * permuting an empty set returns an empty list
//  * drawing a sample of size zero from a non-empty set returns an empty list
func TestEmptyPermutationSubset(t *testing.T) {
	seed := make([]byte, 32)
	seed[0] = 45
	streamID := make([]byte, 12)

	rng, err := NewChacha20(seed, streamID)
	require.NoError(t, err)

	// verify that permuting an empty set returns an empty list
	res, err := rng.SubPermutation(0, 0)
	require.NoError(t, err)
	assert.True(t, len(res) == 0)

	// verify that drawing a sample of size zero from a non-empty set returns an empty list
	res, err = rng.SubPermutation(10, 0)
	require.NoError(t, err)
	assert.True(t, len(res) == 0)
}

func TestRandomShuffle(t *testing.T) {
	listSize := 100
	seed := make([]byte, 32)
	seed[0] = 45

	streamID := make([]byte, 12)
	rng, err := NewChacha20(seed, streamID)
	require.NoError(t, err)
	// statictics parameters
	sampleSize := 64768
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
}

func TestEmptyShuffle(t *testing.T) {
	seed := make([]byte, 32)
	seed[0] = 45
	streamID := make([]byte, 12)
	rng, err := NewChacha20(seed, streamID)
	require.NoError(t, err)
	emptySlice := make([]float64, 0)
	err = rng.Shuffle(len(emptySlice), func(i, j int) {
		emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
	})
	require.NoError(t, err)
	assert.True(t, len(emptySlice) == 0)
}

func TestRandomSamples(t *testing.T) {
	listSize := 100
	samplesSize := 20
	seed := make([]byte, 32)
	seed[0] = 45
	streamID := make([]byte, 12)

	rng, err := NewChacha20(seed, streamID)
	require.NoError(t, err)
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
}

// TestEmptySamples verifies that drawing a sample of size zero leaves the original list unchanged
func TestEmptySamples(t *testing.T) {
	seed := make([]byte, 32)
	seed[0] = 45
	streamID := make([]byte, 12)
	rng, err := NewChacha20(seed, streamID)
	require.NoError(t, err)

	// Sampling from an empty set
	emptySlice := make([]float64, 0)
	err = rng.Samples(len(emptySlice), len(emptySlice), func(i, j int) {
		emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
	})
	require.NoError(t, err)
	assert.True(t, len(emptySlice) == 0)

	// drawing a sample of size zero from an non-empty list should leave the original list unmodified
	fullSlice := []float64{0, 1, 2, 3, 4, 5}
	err = rng.Samples(len(fullSlice), 0, func(i, j int) { // modifies fullSlice IN-PLACE
		emptySlice[i], emptySlice[j] = emptySlice[j], emptySlice[i]
	})
	require.NoError(t, err)
	assert.Equal(t, []float64{0, 1, 2, 3, 4, 5}, fullSlice)
}

// TODO: use randomized data
// TODO: use chacha20 constants

// TestStateRestore tests the serilaization and deserialization functions
// State and Restore
func TestStateRestore(t *testing.T) {
	// generate a seed
	seed := make([]byte, Chacha20SeedLen)
	_, err := crand.Read(seed)
	require.NoError(t, err)

	streamID := make([]byte, Chacha20CustomizerMaxLen)
	_, err = crand.Read(streamID)
	require.NoError(t, err)
	t.Logf("seed is %x, stream_id is %x\n", seed, streamID)

	// create an rng
	rng, err := NewChacha20(seed, streamID)
	require.NoError(t, err)

	// evolve the internal state of the rng
	iterations := rand.Intn(1000)
	for i := 0; i < iterations; i++ {
		_ = rng.UintN(1024)
	}
	// get the internal state of the rng
	state := rng.State()

	// check the state is deterministic
	state_clone := rng.State()
	require.True(t, bytes.Equal(state, state_clone), "State is not deterministic")

	// check State is the Restore reverse function
	secondRng, err := Restore(state)
	require.NoError(t, err)
	require.True(t, bytes.Equal(state, secondRng.State()), "State o Restore is not identity")

	// check the 2 PRGs are generating identical outputs
	iterations = rand.Intn(1000)
	for i := 0; i < iterations; i++ {
		rand1 := rng.UintN(1024)
		rand2 := secondRng.UintN(1024)
		require.Equal(t, rand1, rand2, "the 2 rngs are not identical on round %d", i)
	}
}

// TODO : add test for chacha20 key size to be 256 bits
