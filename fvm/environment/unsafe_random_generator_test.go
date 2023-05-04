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

// TODO: these functions are copied from flow-go/crypto/rand
// Once the new flow-go/crypto/ module version is tagged, flow-go would upgrade
// to the new version and import these functions
func BasicDistributionTest(t *testing.T, n uint64, classWidth uint64, randf func() (uint64, error)) {
	// sample size should ideally be a high number multiple of `n`
	// but if `n` is too small, we could use a small sample size so that the test
	// isn't too slow
	sampleSize := 1000 * n
	if n < 100 {
		sampleSize = (80000 / n) * n // highest multiple of n less than 80000
	}
	distribution := make([]float64, n)
	// populate the distribution
	for i := uint64(0); i < sampleSize; i++ {
		r, err := randf()
		require.NoError(t, err)
		if n*classWidth != 0 {
			require.Less(t, r, n*classWidth)
		}
		distribution[r/classWidth] += 1.0
	}
	EvaluateDistributionUniformity(t, distribution)
}

func EvaluateDistributionUniformity(t *testing.T, distribution []float64) {
	tolerance := 0.05
	stdev := stat.StdDev(distribution, nil)
	mean := stat.Mean(distribution, nil)
	assert.Greater(t, tolerance*mean, stdev, fmt.Sprintf("basic randomness test failed: n: %d, stdev: %v, mean: %v", len(distribution), stdev, mean))
}

func TestUnsafeRandomGenerator(t *testing.T) {
	bh := unittest.BlockHeaderFixtureOnChain(flow.Mainnet.Chain().ChainID())

	getRandoms := func(txnIndex uint32, N int) []uint64 {
		// seed the RG with the same block header
		urg := environment.NewUnsafeRandomGenerator(
			tracing.NewTracerSpan(),
			bh,
			txnIndex)
		numbers := make([]uint64, N)
		for i := 0; i < N; i++ {
			u, err := urg.UnsafeRandom()
			require.NoError(t, err)
			numbers[i] = u
		}
		return numbers
	}

	// basic randomness test to check outputs are "uniformly" spread over the
	// output space
	t.Run("randomness test", func(t *testing.T) {
		for txnIndex := uint32(0); txnIndex < 10; txnIndex++ {
			urg := environment.NewUnsafeRandomGenerator(
				tracing.NewTracerSpan(),
				bh,
				txnIndex)

			// make sure n is a power of 2 so that there is no bias in the last class
			// n is a random power of 2 (from 2 to 2^10)
			n := 1 << (1 + mrand.Intn(10))
			classWidth := (math.MaxUint64 / uint64(n)) + 1
			BasicDistributionTest(t, uint64(n), uint64(classWidth), urg.UnsafeRandom)
		}
	})

	// tests that unsafeRandom is PRG based and hence has deterministic outputs.
	t.Run("PRG-based UnsafeRandom", func(t *testing.T) {
		for txnIndex := uint32(0); txnIndex < 10; txnIndex++ {
			N := 100
			r1 := getRandoms(txnIndex, N)
			r2 := getRandoms(txnIndex, N)
			require.Equal(t, r1, r2)
		}
	})

	t.Run("transaction specific randomness", func(t *testing.T) {
		txns := [][]uint64{}
		for txnIndex := uint32(0); txnIndex < 10; txnIndex++ {
			N := 100
			txns = append(txns, getRandoms(txnIndex, N))
		}

		for i, txn := range txns {
			for _, otherTxn := range txns[i+1:] {
				require.NotEqual(t, txn, otherTxn)
			}
		}
	})
}
