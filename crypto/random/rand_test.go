package random

import (
	"bytes"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20"
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

	t.Run("short nonce", func(t *testing.T) {
		seed := make([]byte, Chacha20SeedLen)
		customizer := make([]byte, Chacha20CustomizerMaxLen)

		// short nonces should be accepted
		_, err := NewChacha20PRG(seed, customizer[:Chacha20CustomizerMaxLen-1])
		assert.NoError(t, err)
		_, err = NewChacha20PRG(seed, customizer[:0])
		assert.NoError(t, err)
	})
}

func getPRG(t *testing.T) *mrand.Rand {
	random := time.Now().UnixNano()
	t.Logf("rng seed is %d", random)
	rng := mrand.New(mrand.NewSource(random))
	return rng
}

// The tests are targeting the PRG implementations in the package.
// For now, the tests are only used for Chacha20 PRG, but can be ported
// to test another PRG implementation.

// Simple unit testing of UintN using a basic randomness test.
// It doesn't perform advanced statistical tests.
func TestUintN(t *testing.T) {
	rand := getPRG(t)
	seed := make([]byte, Chacha20SeedLen)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	customizer := make([]byte, Chacha20CustomizerMaxLen)
	_, err = rand.Read(customizer)
	require.NoError(t, err)

	rng, err := NewChacha20PRG(seed, customizer)
	require.NoError(t, err)

	t.Run("basic uniformity", func(t *testing.T) {
		maxN := uint64(1000)
		mod := mrand.Uint64()
		var n, classWidth uint64
		if mod < maxN { // `mod` is too small so that we can consider `mod` classes
			n = mod
			classWidth = 1
		} else { // `mod` is big enough so that we can partition [0,mod-1] into `maxN` classes
			n = maxN
			mod = (mod / n) * n // adjust `mod` to make sure it is a multiple of n for a more accurate test
			classWidth = mod / n
		}

		uintNf := func() (uint64, error) {
			return uint64(rng.UintN(mod)), nil
		}
		BasicDistributionTest(t, n, classWidth, uintNf)
	})

	t.Run("zero n", func(t *testing.T) {
		assert.Panics(t, func() {
			rng.UintN(0)
		})
	})
}

// Simple unit testing of SubPermutation using a basic randomness test.
// It doesn't perform advanced statistical tests.
//
// SubPermutation tests cover Permutation as well.
func TestSubPermutation(t *testing.T) {
	rand := getPRG(t)

	seed := make([]byte, Chacha20SeedLen)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	customizer := make([]byte, Chacha20CustomizerMaxLen)
	_, err = rand.Read(customizer)
	require.NoError(t, err)

	rng, err := NewChacha20PRG(seed, customizer)
	require.NoError(t, err)

	t.Run("basic randomness", func(t *testing.T) {
		listSize := 100
		subsetSize := 20
		sampleSize := 85000
		// tests the subset sampling randomness
		samplingDistribution := make([]float64, listSize)
		// tests the subset ordering randomness (using a particular element testElement)
		orderingDistribution := make([]float64, subsetSize)
		testElement := rand.Intn(listSize)

		for i := 0; i < sampleSize; i++ {
			shuffledlist, err := rng.SubPermutation(listSize, subsetSize)
			require.NoError(t, err)
			require.Equal(t, len(shuffledlist), subsetSize)
			has := make(map[int]struct{})
			for j, e := range shuffledlist {
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
		EvaluateDistributionUniformity(t, samplingDistribution)
		EvaluateDistributionUniformity(t, orderingDistribution)
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

// Simple unit testing of Shuffle using a basic randomness test.
// It doesn't perform advanced statistical tests.
func TestShuffle(t *testing.T) {
	rand := getPRG(t)

	seed := make([]byte, Chacha20SeedLen)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	customizer := make([]byte, Chacha20CustomizerMaxLen)
	_, err = rand.Read(customizer)
	require.NoError(t, err)

	rng, err := NewChacha20PRG(seed, customizer)
	require.NoError(t, err)

	t.Run("basic uniformity", func(t *testing.T) {
		// compute n!
		fact := func(n int) int {
			f := 1
			for i := 1; i <= n; i++ {
				f *= i
			}
			return f
		}

		for listSize := 2; listSize <= 6; listSize++ {
			factN := uint64(fact(listSize))
			t.Logf("permutation size is %d (factorial is %d)", listSize, factN)
			t.Run("shuffle a random permutation", func(t *testing.T) {
				list := make([]int, 0, listSize)
				for i := 0; i < listSize; i++ {
					list = append(list, i)
				}
				permEncoding := func() (uint64, error) {
					err = rng.Shuffle(listSize, func(i, j int) {
						list[i], list[j] = list[j], list[i]
					})
					return uint64(EncodePermutation(list)), err
				}
				BasicDistributionTest(t, factN, 1, permEncoding)
			})

			t.Run("shuffle a same permutation", func(t *testing.T) {
				list := make([]int, listSize)
				permEncoding := func() (uint64, error) {
					// reinit the permutation to the same value
					for i := 0; i < listSize; i++ {
						list[i] = i
					}
					err = rng.Shuffle(listSize, func(i, j int) {
						list[i], list[j] = list[j], list[i]
					})
					return uint64(EncodePermutation(list)), err
				}
				BasicDistributionTest(t, factN, 1, permEncoding)
			})
		}
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
	rand := getPRG(t)

	seed := make([]byte, Chacha20SeedLen)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	customizer := make([]byte, Chacha20CustomizerMaxLen)
	_, err = rand.Read(customizer)
	require.NoError(t, err)

	rng, err := NewChacha20PRG(seed, customizer)
	require.NoError(t, err)

	t.Run("basic uniformity", func(t *testing.T) {
		listSize := 100
		samplesSize := 20
		sampleSize := 100000
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
		EvaluateDistributionUniformity(t, samplingDistribution)
		EvaluateDistributionUniformity(t, orderingDistribution)
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
	rand := getPRG(t)

	// generate a seed
	seed := make([]byte, Chacha20SeedLen)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	customizer := make([]byte, Chacha20CustomizerMaxLen)
	_, err = rand.Read(customizer)
	require.NoError(t, err)
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
	assert.True(t, bytes.Equal(state, state_clone), "Store is not deterministic")

	// check Store is the Restore reverse function
	secondRng, err := RestoreChacha20PRG(state)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(state, secondRng.Store()), "Store o Restore is not identity")

	// check the 2 PRGs are generating identical outputs
	iterations = rand.Intn(1000)
	for i := 0; i < iterations; i++ {
		rand1 := rng.UintN(1024)
		rand2 := secondRng.UintN(1024)
		assert.Equal(t, rand1, rand2, "the 2 rngs are not identical on round %d", i)
	}
}
