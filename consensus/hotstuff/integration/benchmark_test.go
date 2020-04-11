package integration

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkSingleInstance(b *testing.B) {

	// stop the timer for the setup
	b.StopTimer()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {

		// create a single instance to run for 16 blocks
		in := NewInstance(b,
			WithStopCondition(ViewReached(128)),
		)

		// time the startup up to stop condition
		b.StartTimer()
		err := in.loop.Start()
		require.True(b, errors.Is(err, errStopCondition))
		b.StopTimer()
	}
}
