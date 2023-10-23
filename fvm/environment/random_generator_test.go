package environment_test

import (
	"encoding/binary"
	"math"
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto/random"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRandomGenerator(t *testing.T) {
	randomSourceHistoryProvider := &mock.EntropyProvider{}
	randomSourceHistoryProvider.On("RandomSource").Return(unittest.RandomBytes(48), nil)

	getRandoms := func(txId []byte, N int) []uint64 {
		// seed the RG with the same block header
		urg := environment.NewRandomGenerator(
			tracing.NewTracerSpan(),
			randomSourceHistoryProvider,
			txId)
		numbers := make([]uint64, N)
		for i := 0; i < N; i++ {
			var buffer [8]byte
			err := urg.ReadRandom(buffer[:])
			require.NoError(t, err)
			numbers[i] = binary.LittleEndian.Uint64(buffer[:])
		}
		return numbers
	}

	// basic randomness test to check outputs are "uniformly" spread over the
	// output space
	t.Run("randomness test", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			txId := unittest.TransactionFixture().ID()
			urg := environment.NewRandomGenerator(
				tracing.NewTracerSpan(),
				randomSourceHistoryProvider,
				txId[:])

			// make sure n is a power of 2 so that there is no bias in the last class
			// n is a random power of 2 (from 2 to 2^10)
			n := 1 << (1 + mrand.Intn(10))
			classWidth := (math.MaxUint64 / uint64(n)) + 1
			random.BasicDistributionTest(t, uint64(n), uint64(classWidth), func() (uint64, error) {
				var buffer [8]byte
				err := urg.ReadRandom(buffer[:])
				if err != nil {
					return 0, err
				}
				return binary.LittleEndian.Uint64(buffer[:]), nil
			})
		}
	})

	// tests that has deterministic outputs.
	t.Run("PRG-based Random", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			txId := unittest.TransactionFixture().ID()
			N := 100
			r1 := getRandoms(txId[:], N)
			r2 := getRandoms(txId[:], N)
			require.Equal(t, r1, r2)
		}
	})

	t.Run("transaction specific randomness", func(t *testing.T) {
		txns := [][]uint64{}
		for i := 0; i < 10; i++ {
			txId := unittest.TransactionFixture().ID()
			N := 2
			txns = append(txns, getRandoms(txId[:], N))
		}

		for i, txn := range txns {
			for _, otherTxn := range txns[i+1:] {
				require.NotEqual(t, txn, otherTxn)
			}
		}
	})
}
