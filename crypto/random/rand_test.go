package random

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
)

// The only purpose of this function is unit testing,
// it doesn't evaluate randomness of the random function and doesn't perform advanced statistical tests
// just making sure code works on edge cases (not the quality)
func TestXorshift(t *testing.T) {
	sampleSize := 64768
	tolerance := 0.05
	sampleSpace := 16 // this should be a power of 2 for a more uniform distribution
	distribution := make([]float64, sampleSpace)
	seed := []uint8{0x6A, 0x23, 0x41, 0xB7, 0x80, 0xE1, 0x64, 0x59,
		0x6A, 0x53, 0x40, 0xB7, 0x80, 0xE4, 0x64, 0x5C,
		0x66, 0x53, 0x41, 0xB7, 0x80, 0xE1, 0x64, 0x51,
		0xAA, 0x53, 0x40, 0xB7, 0x80, 0xE4, 0x64, 0x50}
	rand, err := NewRand(seed)
	require.NoError(t, err)
	for i := 0; i < sampleSize; i++ {
		r := rand.IntN(sampleSpace)
		distribution[r] += 1.0
	}
	stdev := stat.StdDev(distribution, nil)
	mean := stat.Mean(distribution, nil)
	if stdev > tolerance*mean {
		t.Errorf("basic randomness test failed. stdev %v, mean %v", stdev, mean)
	}
}

func TestRandomPermutationSubset(t *testing.T) {
	listSize := 1000
	subsetSize := 20
	seed := make([]byte, 16)
	// test a zero seed
	rng, err := NewRand(seed)
	require.Error(t, err)
	// fix thee seed
	seed[0] = 45
	rng, err = NewRand(seed)
	require.NoError(t, err)

	shuffledlist := rng.SubPermutation(listSize, subsetSize)
	if len(shuffledlist) != subsetSize {
		t.Errorf("PermutateSubset returned a list with a wrong size")
	}
	// check for repetition
	has := make(map[int]bool)
	for i := range shuffledlist {
		if _, ok := has[shuffledlist[i]]; ok {
			t.Errorf("dupplicated item in the results returned by PermutateSubset")
		}
		has[shuffledlist[i]] = true
	}
}
