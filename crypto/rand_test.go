package crypto

import (
	"testing"

	"gonum.org/v1/gonum/stat"
)

// The only purpose of this function is unit testing,
// it doesn't evaluate randomness of the random function and doesn't performe more advance tests
// just making sure code works on edge cases (not the quality)
func TestRand(t *testing.T) {
	sampleSize := 64768
	tolerance := 0.05
	sampleSpace := 16 // this should be 2^something
	distribution := make([]float64, sampleSpace)
	seed := []uint64{uint64(62534197802164589), uint64(121823123834)}
	rand, err := NewRand(seed)
	if err != nil {
		t.Errorf("something wrong when trying to create a the random generator.")
	}
	for i := 0; i < sampleSize; i++ {
		distribution[rand.IntN(sampleSpace)] += 1.0
	}
	stdev := stat.StdDev(distribution, nil)
	mean := stat.Mean(distribution, nil)
	if stdev > tolerance*mean {
		t.Errorf("something wrong with the random generator. stdev %v, mean %v", stdev, mean)
	}
}

func TestRandomPermutationSubset(t *testing.T) {
	listSize := 1000
	subsetSize := 20
	seed := []uint64{1, 2, 3, 4, 5, 6}
	shuffledlist, err := RandomPermutationSubset(listSize, subsetSize, seed)
	if err != nil {
		t.Errorf("RandomPermutationSubset returned an error")

	}
	if len(shuffledlist) != subsetSize {
		t.Errorf("RandomPermutationSubset returned a list with a wrong size")
	}
	// check for repetition
	has := make(map[int]bool)
	for i := range shuffledlist {
		if _, ok := has[shuffledlist[i]]; ok {
			t.Errorf("dupplicated item in the results returned by RandomPermutationSubset")
		}
		has[shuffledlist[i]] = true
	}
}
