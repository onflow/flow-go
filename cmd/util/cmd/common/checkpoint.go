package common

import (
	"fmt"
	"os"

	"github.com/onflow/flow-go/ledger/complete/wal"
)

// MoveCheckpointFiles moves a complete V6 checkpoint (1 header file + 16 subtrie files +
// 1 top-trie file = 18 files total) from (sourceDir, sourceName) to (destDir, destName),
// renaming all 18 files so that their base name changes from sourceName to destName while
// preserving the per-file suffixes (.000 through .016).
//
// This is the Go equivalent of tools/move-checkpoint.sh and is intended for use by
// compact-execution-state after a checkpoint extraction, when the checkpoint must be
// placed in the execution-state directory and named after a specific WAL segment number.
//
// The function validates that all 18 source files exist before moving any of them.
// If destDir does not exist it is created.
//
// No error returns are expected during normal operation.
func MoveCheckpointFiles(sourceDir, sourceName, destDir, destName string) error {
	sourcePaths := wal.CheckpointV6AllFilePaths(sourceDir, sourceName)
	destPaths := wal.CheckpointV6AllFilePaths(destDir, destName)

	for _, p := range sourcePaths {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("missing checkpoint file %s: %w", p, err)
		}
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("cannot create destination directory %s: %w", destDir, err)
	}

	for i, src := range sourcePaths {
		dst := destPaths[i]
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("cannot move %s to %s: %w", src, dst, err)
		}
	}

	return nil
}
