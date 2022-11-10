package environment_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

func TestUnsafeRandomGenerator(t *testing.T) {
	t.Run("UnsafeRandom doesnt re-seed the random", func(t *testing.T) {
		bh := &flow.Header{}

		urg := environment.NewUnsafeRandomGenerator(&environment.Tracer{}, bh)

		// 10 random numbers. extremely unlikely to get the same number all the time and just fail the test by chance
		N := 10

		numbers := make([]uint64, N)

		for i := 0; i < N; i++ {
			u, err := urg.UnsafeRandom()
			require.NoError(t, err)
			numbers[i] = u
		}

		allEqual := true
		for i := 1; i < N; i++ {
			allEqual = allEqual && numbers[i] == numbers[0]
		}
		require.True(t, !allEqual)
	})
}
