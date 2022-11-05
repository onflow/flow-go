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
			p, err := profiler.New(zerolog.Nop(), &profiler.NoopUploader{}, tempDir, time.Hour, time.Millisecond*100, true)
			require.NoError(t, err)

			err = p.TriggerRun()
			require.NoError(t, err)

			unittest.AssertClosesBefore(t, p.Ready(), 5*time.Second)

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
