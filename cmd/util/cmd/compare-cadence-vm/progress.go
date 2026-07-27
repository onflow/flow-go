package compare_cadence_vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/onflow/flow-go/model/flow"
)

// progressVersion is the schema version of the progress file.
// It is increased whenever the format changes in a way that makes older files unusable.
const progressVersion = 1

// runConfig captures the flags that determine which blocks a run compares, and how it compares them.
// A run may only resume from a progress file that was written by a run with an equal configuration,
// because the recorded number of compared blocks is otherwise meaningless.
//
// The batch size is deliberately not part of the configuration: the progress is recorded as a
// number of compared blocks, which a run with a different batch size continues from just as well.
type runConfig struct {
	Chain        string   `json:"chain"`
	BlockIDs     []string `json:"block_ids"`
	BlockCount   int      `json:"block_count"`
	ComputeLimit uint64   `json:"compute_limit"`
}

// equals returns true if both configurations compare the same blocks in the same way.
// The configurations can not be compared with the equality operator, because they contain a slice.
func (c runConfig) equals(other runConfig) bool {
	return c.Chain == other.Chain &&
		c.BlockCount == other.BlockCount &&
		c.ComputeLimit == other.ComputeLimit &&
		slices.Equal(c.BlockIDs, other.BlockIDs)
}

// runStats are the comparison results accumulated over all completed batches of a run.
type runStats struct {
	BlocksMatched          int64 `json:"blocks_matched"`
	BlocksMismatched       int64 `json:"blocks_mismatched"`
	TransactionsMatched    int64 `json:"transactions_matched"`
	TransactionsMismatched int64 `json:"transactions_mismatched"`
}

// add adds the results of another batch or run.
func (s *runStats) add(other runStats) {
	s.BlocksMatched += other.BlocksMatched
	s.BlocksMismatched += other.BlocksMismatched
	s.TransactionsMatched += other.TransactionsMatched
	s.TransactionsMismatched += other.TransactionsMismatched
}

// runProgress is the persisted state of a batched comparison run.
//
// It is only written once all blocks of a batch have been compared. A run that is interrupted
// while comparing a batch therefore resumes at the first block of that batch, and never skips
// a block that has not been compared.
type runProgress struct {
	Version int       `json:"version"`
	Config  runConfig `json:"config"`

	// CompletedBlockCount is the number of blocks that all completed batches compared.
	CompletedBlockCount int `json:"completed_block_count"`

	// NextBlockID is the ID of the first block of the next batch.
	// In follow-parents mode, it is the next ancestor only while the run has blocks remaining;
	// it is empty once all blocks of the run have been compared.
	NextBlockID string `json:"next_block_id"`

	Stats runStats `json:"stats"`
}

// newRunProgress returns the progress of a run that has not compared any block yet.
func newRunProgress(config runConfig, firstBlockID flow.Identifier) runProgress {
	return runProgress{
		Version:     progressVersion,
		Config:      config,
		NextBlockID: blockIDString(firstBlockID),
	}
}

// validate checks that the progress was written by an equivalent run and describes a state
// that the current run can continue from.
//
// No error returns are expected during normal operation.
func (p runProgress) validate(config runConfig, totalBlockCount int) error {
	if p.Version != progressVersion {
		return fmt.Errorf(
			"progress file has version %d, but this version of the tool writes version %d",
			p.Version,
			progressVersion,
		)
	}

	if !p.Config.equals(config) {
		return fmt.Errorf("progress file was written by a run with a different configuration")
	}

	if p.CompletedBlockCount < 0 || p.CompletedBlockCount > totalBlockCount {
		return fmt.Errorf(
			"progress file reports %d compared blocks, which is outside the %d blocks of this run",
			p.CompletedBlockCount,
			totalBlockCount,
		)
	}

	if p.CompletedBlockCount == totalBlockCount {
		return nil
	}

	if _, err := p.nextBlockID(); err != nil {
		return err
	}

	return nil
}

// nextBlockID returns the ID of the first block of the next batch.
//
// No error returns are expected during normal operation.
func (p runProgress) nextBlockID() (flow.Identifier, error) {
	if p.NextBlockID == "" {
		return flow.ZeroID, fmt.Errorf("progress file does not contain the ID of the next block")
	}

	blockID, err := flow.HexStringToIdentifier(p.NextBlockID)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("progress file contains an invalid next block ID: %w", err)
	}

	return blockID, nil
}

// blockIDString returns the hexadecimal representation of the block ID,
// and the empty string for the zero identifier, which indicates that no block is left to compare.
func blockIDString(blockID flow.Identifier) string {
	if blockID == flow.ZeroID {
		return ""
	}
	return blockID.String()
}

// runProgressExists returns true if a progress file exists at the given path.
//
// No error returns are expected during normal operation.
func runProgressExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check progress file %s: %w", path, err)
}

// readRunProgress reads the progress file.
//
// No error returns are expected during normal operation.
func readRunProgress(path string) (progress runProgress, err error) {
	file, err := os.Open(path)
	if err != nil {
		return runProgress{}, fmt.Errorf("failed to open progress file: %w", err)
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&progress); err != nil {
		return runProgress{}, fmt.Errorf("failed to decode progress file: %w", err)
	}

	// A progress file that contains more than one JSON value was not written by this tool,
	// so the decoded value can not be trusted.
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return runProgress{}, fmt.Errorf("progress file contains more than the expected progress")
	}

	return progress, nil
}

// writeRunProgress replaces the progress file with the given progress.
//
// The progress is first written to a temporary file in the same directory, which is then renamed,
// so that an interrupted write can not truncate or corrupt previously recorded progress.
//
// No error returns are expected during normal operation.
func writeRunProgress(path string, progress runProgress) (err error) {
	directoryPath := filepath.Dir(path)

	file, err := os.CreateTemp(directoryPath, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("failed to create temporary progress file: %w", err)
	}
	temporaryPath := file.Name()

	fileClosed := false
	defer func() {
		if !fileClosed {
			err = errors.Join(err, file.Close())
		}
		// The temporary file only exists if it was not renamed, i.e. if writing failed.
		removeErr := os.Remove(temporaryPath)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()

	if err := file.Chmod(0644); err != nil {
		return fmt.Errorf("failed to set permissions of temporary progress file: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(progress); err != nil {
		return fmt.Errorf("failed to encode progress: %w", err)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to flush temporary progress file: %w", err)
	}

	closeErr := file.Close()
	fileClosed = true
	if closeErr != nil {
		return fmt.Errorf("failed to close temporary progress file: %w", closeErr)
	}

	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("failed to replace progress file: %w", err)
	}

	// Flush the rename, so that the progress survives a crash of the machine.
	directory, err := os.Open(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to open directory of progress file: %w", err)
	}
	defer func() {
		err = errors.Join(err, directory.Close())
	}()

	if err := directory.Sync(); err != nil {
		return fmt.Errorf("failed to flush directory of progress file: %w", err)
	}

	return nil
}
