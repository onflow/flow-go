package environment_test

import (
	"fmt"
	"math"
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestUnsafeRandomGenerator(t *testing.T) {
	// basic randomness test to check outputs are "uniformly" spread over the
	// output space
	t.Run("randomness test", func(t *testing.T) {
		bh := unittest.BlockHeaderFixtureOnChain(flow.Mainnet.Chain().ChainID())
		urg := environment.NewUnsafeRandomGenerator(tracing.NewTracerSpan(), bh)

		sampleSize := 80000
		tolerance := 0.05
		n := 10 + mrand.Intn(100)
		distribution := make([]float64, n)

		// partition all outputs into `n` classes and compute the distribution
		// over the partition. Each class is `classWidth`-big
		classWidth := math.MaxUint64 / uint64(n)
		// populate the distribution
		for i := 0; i < sampleSize; i++ {
			r, err := urg.UnsafeRandom()
			require.NoError(t, err)
			distribution[r/classWidth] += 1.0
		}
		stdev := stat.StdDev(distribution, nil)
		mean := stat.Mean(distribution, nil)
		assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic randomness test failed. stdev %v, mean %v", stdev, mean))
	})

	// tests that unsafeRandom is PRG based and hence has deterministic outputs.
	t.Run("PRG-based UnsafeRandom", func(t *testing.T) {
		bh := unittest.BlockHeaderFixtureOnChain(flow.Mainnet.Chain().ChainID())
		N := 100
		getRandoms := func() []uint64 {
			// seed the RG with the same block header
			urg := environment.NewUnsafeRandomGenerator(tracing.NewTracerSpan(), bh)
			numbers := make([]uint64, N)
			for i := 0; i < N; i++ {
				u, err := urg.UnsafeRandom()
				require.NoError(t, err)
				numbers[i] = u
			}
			return numbers
		}
		r1 := getRandoms()
		r2 := getRandoms()
		require.Equal(t, r1, r2)
	})
}
