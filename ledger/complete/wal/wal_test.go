package wal

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

var (
	pathByteSize = 32
	segmentSize  = 32 * 1024
)

func RunWithWALCheckpointerWithFiles(t *testing.T, names ...interface{}) {
	f := names[len(names)-1].(func(*testing.T, *DiskWAL, *Checkpointer))

	fileNames := make([]string, len(names)-1)

	for i := 0; i <= len(names)-2; i++ {
		fileNames[i] = names[i].(string)
	}

	unittest.RunWithTempDir(t, func(dir string) {
		util.CreateFiles(t, dir, fileNames...)

		wal, err := NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, 10, pathByteSize, segmentSize)
		require.NoError(t, err)

		checkpointer, err := wal.NewCheckpointer()
		require.NoError(t, err)

		f(t, wal, checkpointer)

		<-wal.Done()
	})
}

func Test_emptyDir(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, func(t *testing.T, wal *DiskWAL, checkpointer *Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, -1, latestCheckpoint)

		// here when starting LedgerWAL, it creates empty file for writing, hence directory isn't really empty
		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 0, from)
		require.Equal(t, 0, to)
	})
}

// Prometheus WAL require files to be 8 characters, otherwise it gets confused
func Test_noCheckpoints(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "00000000", "00000001", "00000002", func(t *testing.T, wal *DiskWAL, checkpointer *Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, -1, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 0, from)
		require.Equal(t, 3, to) //extra one because WAL now creates empty file on start
	})
}

func Test_someCheckpoints(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "00000000", "00000001", "00000002", "00000003", "00000004", "00000005", "checkpoint.00000002", func(t *testing.T, wal *DiskWAL, checkpointer *Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 2, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 3, from)
		require.Equal(t, 6, to) //extra one because WAL now creates empty file on start
	})
}

func Test_loneCheckpoint(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "checkpoint.00000005", func(t *testing.T, wal *DiskWAL, checkpointer *Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 5, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, -1, from)
		require.Equal(t, -1, to)
	})
}

func Test_lastCheckpointIsFoundByNumericValue(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "checkpoint.00000005", "checkpoint.00000004", "checkpoint.00000006", "checkpoint.00000002", "checkpoint.00000001", func(t *testing.T, wal *DiskWAL, checkpointer *Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 6, latestCheckpoint)
	})
}

func Test_checkpointWithoutPrecedingSegments(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "checkpoint.00000005", "00000006", "00000007", func(t *testing.T, wal *DiskWAL, checkpointer *Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 5, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 6, from)
		require.Equal(t, 8, to) //extra one because WAL now creates empty file on start
	})
}

func Test_checkpointWithSameSegment(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "checkpoint.00000005", "00000005", "00000006", "00000007", func(t *testing.T, wal *DiskWAL, checkpointer *Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 5, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 6, from)
		require.Equal(t, 8, to) //extra one because WAL now creates empty file on start
	})
}

func Test_listingCheckpoints(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "checkpoint.00000005", "checkpoint.00000002", "00000003", "checkpoint.00000000", func(t *testing.T, wal *DiskWAL, checkpointer *Checkpointer) {
		listCheckpoints, err := checkpointer.Checkpoints()
		require.NoError(t, err)
		require.Len(t, listCheckpoints, 3)
		require.Equal(t, []int{0, 2, 5}, listCheckpoints)
	})
}

func Test_NoGapBetweenSegmentsAndLastCheckpoint(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "checkpoint.00000004", "00000006", "00000007", func(t *testing.T, wal *DiskWAL, checkpointer *Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 4, latestCheckpoint)

		_, _, err = checkpointer.NotCheckpointedSegments()
		require.Error(t, err)
	})
}

func Test_LatestPossibleCheckpoints(t *testing.T) {

	require.Equal(t, []int(nil), getPossibleCheckpoints([]int{1, 2, 5}, 0, 0))

	require.Equal(t, []int{1}, getPossibleCheckpoints([]int{1, 2, 5}, 0, 1))
	require.Equal(t, []int{1, 2}, getPossibleCheckpoints([]int{1, 2, 5}, 0, 2))
	require.Equal(t, []int{1, 2}, getPossibleCheckpoints([]int{1, 2, 5}, 0, 3))
	require.Equal(t, []int{1, 2}, getPossibleCheckpoints([]int{1, 2, 5}, 0, 4))

	require.Equal(t, []int{1, 2, 5}, getPossibleCheckpoints([]int{1, 2, 5}, 0, 5))
	require.Equal(t, []int{1, 2, 5}, getPossibleCheckpoints([]int{1, 2, 5}, 0, 6))
	require.Equal(t, []int{1, 2, 5}, getPossibleCheckpoints([]int{1, 2, 5}, 1, 6))

	require.Equal(t, []int{2, 5}, getPossibleCheckpoints([]int{1, 2, 5}, 2, 6))
	require.Equal(t, []int{2}, getPossibleCheckpoints([]int{1, 2, 5}, 2, 3))
	require.Equal(t, []int{2}, getPossibleCheckpoints([]int{1, 2, 5}, 2, 4))
	require.Equal(t, []int{2, 5}, getPossibleCheckpoints([]int{1, 2, 5}, 2, 5))

	require.Equal(t, []int{5}, getPossibleCheckpoints([]int{1, 2, 5}, 3, 5))
	require.Equal(t, []int{5}, getPossibleCheckpoints([]int{1, 2, 5}, 3, 6))

	require.Equal(t, []int{5}, getPossibleCheckpoints([]int{1, 2, 5}, 5, 5))
	require.Equal(t, []int{}, getPossibleCheckpoints([]int{1, 2, 5}, 6, 6))

}
