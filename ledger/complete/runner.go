package complete

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	realWAL "github.com/onflow/flow-go/ledger/complete/wal"
)

type runner struct {
	logger            zerolog.Logger
	checkpointer      *realWAL.Checkpointer
	checkpointsToKeep int // the max number of checkpoint files to keep on disk in order to prevent too many checkpoint files filling disk.
}

func NewRunner(logger zerolog.Logger, checkpointer *realWAL.Checkpointer, checkpointsToKeep int) *runner {
	return &runner{
		logger:            logger,
		checkpointer:      checkpointer,
		checkpointsToKeep: checkpointsToKeep,
	}
}

func (r *runner) runCheckpoint(ctx context.Context, checkpointTries []*trie.MTrie, checkpointNum int) error {
	err := r.createCheckpoint(r.checkpointer, r.logger, checkpointTries, checkpointNum)
	if err != nil {
		return fmt.Errorf("cannot create checkpoints: %w", err)
	}

	err = r.cleanupCheckpoints()
	if err != nil {
		return fmt.Errorf("fail to cleanup old checkpoints: %w", err)
	}
	return nil
}

// createCheckpoint creates the checkpoint with the given tries with the given checkpointNum in the name of
// the generated checkpoint file
func (r *runner) createCheckpoint(checkpointer *realWAL.Checkpointer, logger zerolog.Logger, tries []*trie.MTrie, checkpointNum int) error {
	if len(tries) == 0 {
		return nil
	}

	lg := logger.With().
		Int("checkpoint_num", checkpointNum).
		Int("trie_count", len(tries)).
		Str("first_trie", tries[0].RootHash().String()).
		Str("last_trie", tries[len(tries)-1].RootHash().String()).
		Logger()

	lg.Info().Msgf("serializing checkpoint")

	startTime := time.Now()

	writer, err := checkpointer.CheckpointWriter(checkpointNum)
	if err != nil {
		return fmt.Errorf("cannot generate checkpoint writer: %w", err)
	}
	defer func() {
		closeErr := writer.Close()
		// Return close error if there isn'r any prior error to return.
		if err == nil {
			err = closeErr
		}
	}()

	err = realWAL.StoreCheckpoint(writer, tries...)
	if err != nil {
		return fmt.Errorf("error serializing checkpoint (%d): %w", checkpointNum, err)
	}

	duration := time.Since(startTime)
	lg.Info().Float64("total_time_s", duration.Seconds()).Msgf("created checkpoint %d", checkpointNum)

	return nil
}

func (r *runner) cleanupCheckpoints() error {
	checkpoints, err := r.checkpointer.Checkpoints()
	if err != nil {
		return fmt.Errorf("cannot list checkpoints: %w", err)
	}

	checkpointsToRemove := findCheckpointsToRemove(checkpoints, r.checkpointsToKeep)

	for _, checkpointNum := range checkpointsToRemove {
		err := r.checkpointer.RemoveCheckpoint(checkpointNum)
		if err != nil {
			return fmt.Errorf("cannot remove checkpoint %d: %w", checkpointNum, err)
		}
	}

	return nil
}

// findCheckpointsToRemove takes a list of checkpoint file nums, and count for how many checkpoint files to keep,
// it returns the checkpoint file nums to be removed, which are the oldest checkpoint files.
func findCheckpointsToRemove(checkpoints []int, checkpointToKeep int) []int {
	if len(checkpoints) <= checkpointToKeep {
		return nil
	}

	return checkpoints[:len(checkpoints)-checkpointToKeep]
}
