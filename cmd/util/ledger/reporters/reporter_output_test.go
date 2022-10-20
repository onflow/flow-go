package reporters_test

import (
	"os"
	"path"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
)

func TestReportFileWriter(t *testing.T) {
	dir := t.TempDir()

	filename := path.Join(dir, "test.json")
	log := zerolog.Logger{}

	requireFileContains := func(t *testing.T, expected string) {
		dat, err := os.ReadFile(filename)
		require.NoError(t, err)

		require.Equal(t, []byte(expected), dat)
	}

	type testData struct {
		TestField string
	}

	t.Run("Open & Close - empty json array", func(t *testing.T) {
		rw := reporters.NewReportFileWriter(filename, log)
		rw.Close()

		requireFileContains(t, "[]")
	})
	t.Run("Open & Write One & Close - json array with one element", func(t *testing.T) {
		rw := reporters.NewReportFileWriter(filename, log)
		rw.Write(testData{TestField: "something"})
		rw.Close()

		requireFileContains(t, "[{\"TestField\":\"something\"}]")
	})
	t.Run("Open & Write Many & Close - json array with many elements", func(t *testing.T) {
		rw := reporters.NewReportFileWriter(filename, log)
		rw.Write(testData{TestField: "something0"})
		rw.Write(testData{TestField: "something1"})
		rw.Write(testData{TestField: "something2"})

		rw.Close()

		requireFileContains(t,
			"[{\"TestField\":\"something0\"},{\"TestField\":\"something1\"},{\"TestField\":\"something2\"}]")
	})

	t.Run("Open & Write Many in threads & Close", func(t *testing.T) {
		rw := reporters.NewReportFileWriter(filename, log)

		wg := &sync.WaitGroup{}
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				rw.Write(testData{TestField: "something"})
				wg.Done()
			}()
		}

		wg.Wait()

		rw.Close()

		requireFileContains(t,
			"[{\"TestField\":\"something\"},{\"TestField\":\"something\"},{\"TestField\":\"something\"}]")
	})
}
