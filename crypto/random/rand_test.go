package random

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
)

// The only purpose of this function is unit testing. It also implements a very basic tandomness test.
// it doesn't evaluate randomness of the random function and doesn't perform advanced statistical tests
// just making sure code works on edge cases
func TestRandInt(t *testing.T) {
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
		r, _ := rand.IntN(sampleSpace)
		distribution[r] += 1.0
	}
	stdev := stat.StdDev(distribution, nil)
	mean := stat.Mean(distribution, nil)
	if stdev > tolerance*mean {
		t.Errorf("basic randomness test failed. stdev %v, mean %v", stdev, mean)
	}
}

// The only purpose of this function is unit testing. It also implements a very basic tandomness test.
// it doesn't evaluate randomness of the random function and doesn't perform advanced statistical tests
// just making sure code works on edge cases
func TestRandomPermutationSubset(t *testing.T) {
	listSize := 100
	subsetSize := 20
	seed := make([]byte, 16)
	// test a zero seed
	rng, err := NewRand(seed)
	require.Error(t, err)
	// fix thee seed
	seed[0] = 45
	rng, err = NewRand(seed)
	require.NoError(t, err)
	// statictics parameters
	sampleSize := 64768
	tolerance := 0.05
	distribution := make([]float64, listSize)

	for i := 0; i < sampleSize; i++ {
		shuffledlist, _ := rng.SubPermutation(listSize, subsetSize)
		if len(shuffledlist) != subsetSize {
			t.Errorf("PermutateSubset returned a list with a wrong size")
		}
		has := make(map[int]struct{})
		for i := range shuffledlist {
			// check for repetition
			if _, ok := has[shuffledlist[i]]; ok {
				t.Errorf("dupplicated item in the results returned by PermutateSubset")
			}
			has[shuffledlist[i]] = struct{}{}
			// fill the distribution
			distribution[shuffledlist[i]] += 1.0
		}
	}
	stdev := stat.StdDev(distribution, nil)
	mean := stat.Mean(distribution, nil)
	if stdev > tolerance*mean {
		t.Errorf("basic randomness test failed. stdev %v, mean %v", stdev, mean)
	}
}

func TestRandomShuffle(t *testing.T) {
	listSize := 100
	seed := make([]byte, 16)
	seed[0] = 45
	rng, err := NewRand(seed)
	require.NoError(t, err)
	// statictics parameters
	sampleSize := 64768
	tolerance := 0.05
	distribution := make([]float64, listSize)
	// Slice to shuffle
	list := make([]int, 0, listSize)
	for i := 0; i < listSize; i++ {
		list = append(list, i)
	}

	for i := 0; i < sampleSize; i++ {
		rng.Shuffle(len(list), func(i, j int) {
			list[i], list[j] = list[j], list[i]
		})
		has := make(map[int]struct{})
		for _, e := range list {
			// check for repetition
			if _, ok := has[e]; ok {
				t.Errorf("dupplicated item in the results returned by PermutateSubset")
			}
			has[e] = struct{}{}
			// fill the distribution
			distribution[e] += 1.0
		}
	}
	stdev := stat.StdDev(distribution, nil)
	mean := stat.Mean(distribution, nil)
	if stdev > tolerance*mean {
		t.Errorf("basic randomness test failed. stdev %v, mean %v", stdev, mean)
	}
}
