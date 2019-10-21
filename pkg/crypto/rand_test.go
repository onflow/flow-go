package crypto

import (
	"fmt"
	"testing"

	"gonum.org/v1/gonum/stat"
)

// The only purpose of this function is unit testing,
// it doesn't evaluate randomness of the random function and doesn't performe more advance tests
// just making sure code works on edge cases (not the quality)
func TestRand(t *testing.T) {
	sampleSize := 64768
	tolerance := 0.05
	sampleSpace := uint64(16) // this should be 2^something
	distribution := make([]float64, sampleSpace)
	seed := []uint64{uint64(62534197802164589), uint64(121823123834)}
	rand := NewRand(seed)
	for i := 0; i < sampleSize; i++ {
		distribution[rand.UIntN(sampleSpace)] += 1.0
	}
	stdev := stat.StdDev(distribution, nil)
	mean := stat.Mean(distribution, nil)
	if stdev > tolerance*mean {
		t.Error(fmt.Sprintf("something wrong with the random generator. stdev %v, mean %v", stdev, mean))
	}
}
