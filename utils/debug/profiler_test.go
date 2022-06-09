package debug_test

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/debug"
)

func TestProfiler(t *testing.T) {
	t.Parallel()
	t.Run("profilerEnabled", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "profiles-")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		p, err := debug.NewAutoProfiler(log.Logger, tempDir, time.Millisecond*100, time.Millisecond*100, true)
		require.NoError(t, err)

		<-p.Ready()
		log.Info().Str("dir", tempDir).Msg("profiler ready")

		require.Eventuallyf(t, func() bool {
			dirEnts, err := os.ReadDir(tempDir)
			require.NoError(t, err)
			return len(dirEnts) > 4
		}, time.Second*5, time.Millisecond*100, "profiler did not generate any profiles in %v", tempDir)

		<-p.Done()
	})
}
