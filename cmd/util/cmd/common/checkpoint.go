package common

import (
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/irrecoverable"
	utilsio "github.com/onflow/flow-go/utils/io"
)

// ErrCheckpointFileMissing indicates that one or more expected checkpoint source
// files do not exist. This is a benign error returned during normal operation when
// the caller requests a move for a checkpoint that has not been fully written.
var ErrCheckpointFileMissing = errors.New("checkpoint source file missing")

// MoveCheckpointFiles moves a complete V6 checkpoint (1 header file + 16 subtrie files +
// 1 top-trie file = 18 files total) from (sourceDir, sourceName) to (destDir, destName),
// renaming all 18 files so that their base name changes from sourceName to destName while
// preserving the per-file suffixes (.000 through .016).
//
// This is the Go equivalent of tools/move-checkpoint.sh and is intended for use by
// compact-execution-state after a checkpoint extraction, when the checkpoint must be
// placed in the execution-state directory and named after a specific WAL segment number.
//
// The function validates that all 18 source files exist and that none of the 18
// destination files exist before moving any file. If destDir does not exist it is created.
// When sourceDir and destDir are on different file systems, the atomic rename falls back
// to copying the file contents and removing the source file.
//
// Expected error returns during normal operation:
//   - [ErrCheckpointFileMissing]: if one or more source checkpoint files do not exist
//
// No other error returns are expected during normal operation.
func MoveCheckpointFiles(sourceDir, sourceName, destDir, destName string) error {
	sourcePaths := wal.CheckpointV6AllFilePaths(sourceDir, sourceName)
	destPaths := wal.CheckpointV6AllFilePaths(destDir, destName)

	for _, p := range sourcePaths {
		if _, err := os.Stat(p); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("missing checkpoint file %s: %w", p, ErrCheckpointFileMissing)
			}
			return irrecoverable.NewExceptionf("cannot stat checkpoint file %s: %w", p, err)
		}
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return irrecoverable.NewExceptionf("cannot create destination directory %s: %w", destDir, err)
	}

	for _, p := range destPaths {
		if _, err := os.Stat(p); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return irrecoverable.NewExceptionf("cannot stat destination file %s: %w", p, err)
		}
		return irrecoverable.NewExceptionf("destination checkpoint file already exists: %s", p)
	}

	for i, src := range sourcePaths {
		dst := destPaths[i]
		if err := utilsio.MoveFile(src, dst); err != nil {
			return irrecoverable.NewExceptionf("cannot move checkpoint file from %s to %s: %w", src, dst, err)
		}
	}

	return nil
}

// BackupCheckpointsFrom moves all checkpoint files in dir whose checkpoint number is
// greater than or equal to minNumber into backupDir, preserving their file names.
//
// A checkpoint number identifies the highest WAL segment the checkpoint was created up
// to. After the WAL has been trimmed so that segment minNumber is its last segment, any
// checkpoint at or beyond minNumber references state newer than the trim target and is
// inconsistent with the remaining WAL; moving those checkpoint files to backupDir keeps
// the execution-state directory free of checkpoints the node could load over the trimmed
// WAL, while preserving the files for possible restore.
//
// Each checkpoint is moved via [MoveCheckpointFiles]. A partial checkpoint (a subset of
// the 18 V6 checkpoint files) cannot be moved and is skipped with a warning, since the
// node ignores checkpoints that fail to load. If backupDir does not exist it is created.
//
// No error returns are expected during normal operation.
func BackupCheckpointsFrom(lg zerolog.Logger, dir, backupDir string, minNumber int) error {
	checkpoints, err := wal.Checkpoints(dir)
	if err != nil {
		return fmt.Errorf("cannot list checkpoints in %s: %w", dir, err)
	}

	for _, n := range checkpoints {
		if n < minNumber {
			continue
		}

		name := wal.NumberToFilename(n)
		if err := MoveCheckpointFiles(dir, name, backupDir, name); err != nil {
			if errors.Is(err, ErrCheckpointFileMissing) {
				lg.Warn().Int("checkpoint", n).
					Msg("skipping partial checkpoint in execution state dir")
				continue
			}
			return fmt.Errorf("cannot move checkpoint %d to backup dir: %w", n, err)
		}

		lg.Info().Int("checkpoint", n).Str("backup-dir", backupDir).
			Msg("moved checkpoint to backup dir")
	}

	return nil
}
