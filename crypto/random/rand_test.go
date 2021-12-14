package random

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20"
	"gonum.org/v1/gonum/stat"
)

// sanity check for the underlying implementation of Chacha20
// to make sure the implementation is compliant the RFC 7539.
func TestChacha20Compliance(t *testing.T) {

	t.Run("key and nonce length", func(t *testing.T) {

		assert.Equal(t, Chacha20SeedLen, 32)
		assert.Equal(t, Chacha20CustomizerMaxLen, 12)
	})

	t.Run("RFC test vector", func(t *testing.T) {

		key := []byte{
			0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
			0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
		}
		nonce := []byte{0, 0, 0, 0, 0, 0, 0, 0x4a, 0, 0, 0, 0}
		counter := uint32(1)
		plaintext := []byte{
			0x4c, 0x61, 0x64, 0x69, 0x65, 0x73, 0x20, 0x61, 0x6e, 0x64, 0x20, 0x47, 0x65, 0x6e, 0x74, 0x6c,
			0x65, 0x6d, 0x65, 0x6e, 0x20, 0x6f, 0x66, 0x20, 0x74, 0x68, 0x65, 0x20, 0x63, 0x6c, 0x61, 0x73,
			0x73, 0x20, 0x6f, 0x66, 0x20, 0x27, 0x39, 0x39, 0x3a, 0x20, 0x49, 0x66, 0x20, 0x49, 0x20, 0x63,
			0x6f, 0x75, 0x6c, 0x64, 0x20, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x20, 0x79, 0x6f, 0x75, 0x20, 0x6f,
			0x6e, 0x6c, 0x79, 0x20, 0x6f, 0x6e, 0x65, 0x20, 0x74, 0x69, 0x70, 0x20, 0x66, 0x6f, 0x72, 0x20,
			0x74, 0x68, 0x65, 0x20, 0x66, 0x75, 0x74, 0x75, 0x72, 0x65, 0x2c, 0x20, 0x73, 0x75, 0x6e, 0x73,
			0x63, 0x72, 0x65, 0x65, 0x6e, 0x20, 0x77, 0x6f, 0x75, 0x6c, 0x64, 0x20, 0x62, 0x65, 0x20, 0x69,
		}
		ciphertext := []byte{
			0x6e, 0x2e, 0x35, 0x9a, 0x25, 0x68, 0xf9, 0x80, 0x41, 0xba, 0x07, 0x28, 0xdd, 0x0d, 0x69, 0x81,
			0xe9, 0x7e, 0x7a, 0xec, 0x1d, 0x43, 0x60, 0xc2, 0x0a, 0x27, 0xaf, 0xcc, 0xfd, 0x9f, 0xae, 0x0b,
			0xf9, 0x1b, 0x65, 0xc5, 0x52, 0x47, 0x33, 0xab, 0x8f, 0x59, 0x3d, 0xab, 0xcd, 0x62, 0xb3, 0x57,
			0x16, 0x39, 0xd6, 0x24, 0xe6, 0x51, 0x52, 0xab, 0x8f, 0x53, 0x0c, 0x35, 0x9f, 0x08, 0x61, 0xd8,
			0x07, 0xca, 0x0d, 0xbf, 0x50, 0x0d, 0x6a, 0x61, 0x56, 0xa3, 0x8e, 0x08, 0x8a, 0x22, 0xb6, 0x5e,
			0x52, 0xbc, 0x51, 0x4d, 0x16, 0xcc, 0xf8, 0x06, 0x81, 0x8c, 0xe9, 0x1a, 0xb7, 0x79, 0x37, 0x36,
			0x5a, 0xf9, 0x0b, 0xbf, 0x74, 0xa3, 0x5b, 0xe6, 0xb4, 0x0b, 0x8e, 0xed, 0xf2, 0x78, 0x5e, 0x42,
		}

		chacha, err := chacha20.NewUnauthenticatedCipher(key, nonce)
		require.NoError(t, err)
		chacha.SetCounter(counter)
		chacha.XORKeyStream(plaintext, plaintext)
		assert.Equal(t, plaintext, ciphertext)

	})

	t.Run("invalid constructor inputs", func(t *testing.T) {
		seed := make([]byte, Chacha20SeedLen+1)
		customizer := make([]byte, Chacha20CustomizerMaxLen+1)

		// long seed
		_, err := NewChacha20PRG(seed, customizer[:Chacha20CustomizerMaxLen])
		assert.Error(t, err)
		// long nonce
		_, err = NewChacha20PRG(seed[:Chacha20SeedLen], customizer)
		assert.Error(t, err)
	})
}

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
		testElement := mrand.Intn(listSize)

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
		testElement := mrand.Intn(listSize)
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
		testElement := mrand.Intn(listSize)
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
	iterations := mrand.Intn(1000)
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
	iterations = mrand.Intn(1000)
	for i := 0; i < iterations; i++ {
		rand1 := rng.UintN(1024)
		rand2 := secondRng.UintN(1024)
		require.Equal(t, rand1, rand2, "the 2 rngs are not identical on round %d", i)
	}
}
