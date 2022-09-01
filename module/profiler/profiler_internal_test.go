package profiler

import (
	"bytes"
	"runtime"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestGoHeapProfile(t *testing.T) {
	t.Parallel()
	t.Run("goHeapProfile", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(tempDir string) {
			p, err := New(zerolog.Nop(), &NoopUploader{}, tempDir, time.Millisecond*100, time.Millisecond*100, false)
			require.NoError(t, err)
			unittest.AssertClosesBefore(t, p.Ready(), 5*time.Second)
			t.Logf("profiler ready %s", tempDir)

			prof, err := p.goHeapProfile("inuse_objects", "alloc_space")
			require.NoError(t, err)
			require.NotEmpty(t, prof)

			require.Equal(t, "inuse_objects", prof.DefaultSampleType)
			require.Equal(t, 2, len(prof.SampleType))
			require.Equal(t, "inuse_objects", prof.SampleType[0].Type)
			require.Equal(t, "alloc_space", prof.SampleType[1].Type)
			require.NotZero(t, len(prof.Sample))
			require.Equal(t, 2, len(prof.Sample[0].Value))
			require.NotZero(t, prof.Sample[0].Value[0]+prof.Sample[0].Value[1])

			unittest.AssertClosesBefore(t, p.Done(), 5*time.Second)
		})
	})
}

func TestGoAllocsProfile(t *testing.T) {
	t.Parallel()
	t.Run("pprofAllocs", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(tempDir string) {
			p, err := New(zerolog.Nop(), &NoopUploader{}, tempDir, time.Hour, time.Second*1, false)
			require.NoError(t, err)
			unittest.AssertClosesBefore(t, p.Ready(), 5*time.Second)
			t.Logf("profiler ready %s", tempDir)

			ticker := time.NewTicker(time.Millisecond * 10)
			defer ticker.Stop()

			// do some allocations in the background
			go func() {
				for range ticker.C {
					var m runtime.MemStats
					runtime.ReadMemStats(&m)
				}
			}()

			buf := &bytes.Buffer{}
			err = p.pprofAllocs(buf)
			require.NoError(t, err)

			prof, err := profile.Parse(buf)
			require.NoError(t, err)

			require.Equal(t, "alloc_objects", prof.DefaultSampleType)
			require.Equal(t, 2, len(prof.SampleType))
			require.Equal(t, "alloc_objects", prof.SampleType[0].Type)
			require.Equal(t, "alloc_space", prof.SampleType[1].Type)
			require.NotZero(t, len(prof.Sample))
			require.Equal(t, 2, len(prof.Sample[0].Value))
			require.NotZero(t, prof.Sample[0].Value[0]+prof.Sample[0].Value[1])

			unittest.AssertClosesBefore(t, p.Done(), 5*time.Second)
		})
	})
}
