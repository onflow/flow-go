package wal_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	realWAL "github.com/dapperlabs/flow-go/ledger/wal"
)

func Test_emptyDir(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, func(t *testing.T, wal *realWAL.LedgerWAL, checkpointer *realWAL.Checkpointer) {
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
	RunWithWALCheckpointerWithFiles(t, "00000000", "00000001", "00000002", func(t *testing.T, wal *realWAL.LedgerWAL, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, -1, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 0, from)
		require.Equal(t, 2, to)
	})
}

func Test_someCheckpoints(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "00000000", "00000001", "00000002", "00000003", "00000004", "00000005", "checkpoint.00000002", func(t *testing.T, wal *realWAL.LedgerWAL, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 2, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 3, from)
		require.Equal(t, 5, to)
	})
}

func Test_loneCheckpoint(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "checkpoint.00000005", func(t *testing.T, wal *realWAL.LedgerWAL, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 5, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, -1, from)
		require.Equal(t, -1, to)
	})
}

func Test_checkpointWithoutPrecedingSegments(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "checkpoint.00000005", "00000006", "00000007", func(t *testing.T, wal *realWAL.LedgerWAL, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 5, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 6, from)
		require.Equal(t, 7, to)
	})
}

func Test_checkpointWithSameSegment(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "checkpoint.00000005", "00000005", "00000006", "00000007", func(t *testing.T, wal *realWAL.LedgerWAL, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 5, latestCheckpoint)

		from, to, err := checkpointer.NotCheckpointedSegments()
		require.NoError(t, err)
		require.Equal(t, 6, from)
		require.Equal(t, 7, to)
	})
}

func Test_NoGapBetweenSegmentsAndLastCheckpoint(t *testing.T) {
	RunWithWALCheckpointerWithFiles(t, "checkpoint.00000004", "00000006", "00000007", func(t *testing.T, wal *realWAL.LedgerWAL, checkpointer *realWAL.Checkpointer) {
		latestCheckpoint, err := checkpointer.LatestCheckpoint()
		require.NoError(t, err)
		require.Equal(t, 4, latestCheckpoint)

		_, _, err = checkpointer.NotCheckpointedSegments()
		require.Error(t, err)
	})
}
