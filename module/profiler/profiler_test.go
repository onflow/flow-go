package profiler_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/profiler"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProfiler(t *testing.T) {
	// profiler depends on the shared state, hence only one enabled=true test can run at a time.
	t.Run("profilerEnabled", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(tempDir string) {
			p, err := profiler.New(
				zerolog.Nop(),
				&profiler.NoopUploader{},
				profiler.ProfilerConfig{
					Enabled:  false,
					Dir:      tempDir,
					Interval: time.Hour,
					Duration: 100 * time.Millisecond,
				})
			require.NoError(t, err)

			unittest.AssertClosesBefore(t, p.Ready(), 5*time.Second)

			err = p.SetEnabled(true)
			require.NoError(t, err)

			require.Eventually(t, func() bool { return p.TriggerRun(time.Millisecond*100) == nil }, 1*time.Second, 10*time.Millisecond)

			// Fail if profiling is already running
			err = p.TriggerRun(0)
			require.ErrorContains(t, err, "profiling is already in progress")

			t.Logf("profiler ready %s", tempDir)

			require.Eventuallyf(t, func() bool {
				dirEnts, err := os.ReadDir(tempDir)
				require.NoError(t, err)

				foundPtypes := make(map[string]bool)
				for _, pType := range []string{"heap", "allocs", "goroutine", "cpu", "block"} {
					foundPtypes[pType] = false
				}

				for pName := range foundPtypes {
					for _, ent := range dirEnts {
						if strings.Contains(ent.Name(), pName) {
							foundPtypes[pName] = true
						}
					}
				}

				for pName, found := range foundPtypes {
					if !found {
						t.Logf("profiler %s not found", pName)
						return false
					}
				}
				return true
			}, time.Second*5, time.Millisecond*100, "profiler did not generate any profiles in %v", tempDir)

			unittest.AssertClosesBefore(t, p.Done(), 5*time.Second)
		})
	})
}
