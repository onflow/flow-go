package metrics

import (
	"math/rand"
	"sort"
)

func standardDistributionBuckets(min float64, max float64, buckets int) []float64 {
	// calculate the mean and standard deviation of the range
	rangeWidth := max - min
	mean := (max + min) / 2
	stdDev := rangeWidth / 4

	// generates normally distributed values with mean 0 and std dev 1
	values := make([]float64, buckets)
	for i := range values {
		values[i] = rand.NormFloat64()
	}

	// scale and shift the values to match the desired range
	for i := range values {
		values[i] = values[i]*stdDev + mean
	}

	// sort the resulting slice of sub-ranges
	sort.Float64s(values)

	return values
}
