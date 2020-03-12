package random

import (
	"math/rand"
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
	// tests the subset sampling randomness
	samplingDistribution := make([]float64, listSize)
	// tests the subset ordering randomness (using a particular element testElement)
	orderingDistribution := make([]float64, subsetSize)
	testElement := rand.Intn(listSize)

	for i := 0; i < sampleSize; i++ {
		shuffledlist, _ := rng.SubPermutation(listSize, subsetSize)
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
	if stdev > tolerance*mean {
		t.Errorf("basic subset randomness test failed. stdev %v, mean %v", stdev, mean)
	}
	stdev = stat.StdDev(orderingDistribution, nil)
	mean = stat.Mean(orderingDistribution, nil)
	if stdev > tolerance*mean {
		t.Errorf("basic ordering randomness test failed. stdev %v, mean %v", stdev, mean)
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
	// the distribution of a particular element of the list, testElement
	distribution := make([]float64, listSize)
	testElement := rand.Intn(listSize)
	// Slice to shuffle
	list := make([]int, 0, listSize)
	for i := 0; i < listSize; i++ {
		list = append(list, i)
	}

	for i := 0; i < sampleSize; i++ {
		rng.Shuffle(listSize, func(i, j int) {
			list[i], list[j] = list[j], list[i]
		})
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
	if stdev > tolerance*mean {
		t.Errorf("basic randomness test failed. stdev %v, mean %v", stdev, mean)
	}
}

func TestRandomSamples(t *testing.T) {
	listSize := 100
	samplesSize := 20
	seed := make([]byte, 16)
	seed[0] = 45
	rng, err := NewRand(seed)
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
		rng.Samples(listSize, samplesSize, func(i, j int) {
			list[i], list[j] = list[j], list[i]
		})
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
	if stdev > tolerance*mean {
		t.Errorf("basic subset randomness test failed. stdev %v, mean %v", stdev, mean)
	}
	stdev = stat.StdDev(orderingDistribution, nil)
	mean = stat.Mean(orderingDistribution, nil)
	if stdev > tolerance*mean {
		t.Errorf("basic ordering randomness test failed. stdev %v, mean %v", stdev, mean)
	}
}
