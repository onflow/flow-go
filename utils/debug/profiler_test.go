package debug_test

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/debug"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProfiler(t *testing.T) {
	t.Parallel()
	t.Run("profilerEnabled", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(tempDir string) {
			p, err := debug.NewAutoProfiler(zerolog.Nop(), tempDir, time.Millisecond*100, time.Millisecond*100, true)
			require.NoError(t, err)

			unittest.AssertClosesBefore(t, p.Ready(), 5*time.Second)

			t.Logf("profiler ready %s", tempDir)

			require.Eventuallyf(t, func() bool {
				dirEnts, err := os.ReadDir(tempDir)
				require.NoError(t, err)
				return len(dirEnts) > 4
			}, time.Second*5, time.Millisecond*100, "profiler did not generate any profiles in %v", tempDir)

			unittest.AssertClosesBefore(t, p.Done(), 5*time.Second)
		})
	})
}
