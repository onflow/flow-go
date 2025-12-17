package storehouse

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// ErrMismatch represents a register value mismatch error with details about the mismatch.
type ErrMismatch struct {
	RegisterID     flow.RegisterID
	Height         uint64
	StoredLength   int
	ExpectedLength int
	StoredData     []byte
	ExpectedData   []byte
	Message        string
}

func (e *ErrMismatch) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("register value mismatch: owner=%s, key=%s, height=%d, stored_length=%d, expected_length=%d",
		e.RegisterID.Owner, e.RegisterID.Key, e.Height, e.StoredLength, e.ExpectedLength)
}

// IsErrMismatch returns true if the given error is an ErrMismatch or wraps an ErrMismatch.
func IsErrMismatch(err error) (*ErrMismatch, bool) {
	var mismatchErr *ErrMismatch
	isErr := errors.As(err, &mismatchErr)
	return mismatchErr, isErr
}

// ValidateWithCheckpoint validates the registers in the given store against the leaf nodes read from the checkpoint file.
// Limitation: the validation can not cover if there are extra non-empty registers in the store that are not in the checkpoint file.
func ValidateWithCheckpoint(
	log zerolog.Logger,
	ctx context.Context,
	store execution.OnDiskRegisterStore,
	results storage.ExecutionResults,
	headers storage.Headers,
	checkpointDir string, // checkpointDir must have a root.checkpoint file that contains only a single trie
	blockHeight uint64,
	workerCount int,
) error {
	// used by the wal reader to send leaf nodes read from checkpoint file
	// used by N workers to validate registers in store
	leafNodeChan := make(chan *wal.LeafNode, 1000)

	// get rootHash before creating goroutines since we need a valid rootHash to validate registers
	rootHash, err := rootHashByHeight(results, headers, blockHeight)
	if err != nil {
		return err
	}

	// create N workers to validate registers in store
	cct, cancel := context.WithCancel(ctx)
	defer cancel()

	g, gCtx := errgroup.WithContext(cct)

	// track total number of mismatch errors across all workers
	var mismatchErrorCount atomic.Int64

	start := time.Now()
	log.Info().Msgf("validation registers from checkpoint with %v worker", workerCount)
	for i := 0; i < workerCount; i++ {
		g.Go(func() error {
			return validatingRegisterInStore(gCtx, log, store, leafNodeChan, blockHeight, &mismatchErrorCount)
		})
	}

	// read leaf nodes from checkpoint file and send to leafNodeChan
	err = wal.OpenAndReadLeafNodesFromCheckpointV6(leafNodeChan, checkpointDir, "root.checkpoint", rootHash, log)
	if err != nil {
		return fmt.Errorf("error reading leaf node from checkpoint: %w", err)
	}

	if err = g.Wait(); err != nil {
		return fmt.Errorf("failed to validate registers from checkpoint file: %w", err)
	}

	totalMismatches := mismatchErrorCount.Load()
	if totalMismatches > 0 {
		return fmt.Errorf("validation failed: found %d register value mismatches", totalMismatches)
	}

	log.Info().Msgf("finished validating registers from checkpoint in %s, no mismatch found", time.Since(start))
	return nil
}

func rootHashByHeight(results storage.ExecutionResults, headers storage.Headers, height uint64) (ledger.RootHash, error) {
	blockID, err := headers.BlockIDByHeight(height)
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("could not get block ID at height %d: %w", height, err)
	}

	result, err := results.ByBlockID(blockID)
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("could not get execution result for block ID %s: %w", blockID, err)
	}

	commit, err := result.FinalStateCommitment()
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("could not get final state commitment for block ID %s: %w", blockID, err)
	}

	return ledger.RootHash(commit), nil
}

func validatingRegisterInStore(ctx context.Context, log zerolog.Logger, store execution.OnDiskRegisterStore, leafNodeChan chan *wal.LeafNode, height uint64, mismatchErrorCount *atomic.Int64) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case leafNode, ok := <-leafNodeChan:
			if !ok {
				return nil
			}
			err := validateRegister(store, leafNode, height)
			if err != nil {
				mismatchErr, ok := IsErrMismatch(err)
				if ok {
					// mismatch error: log and continue, increment counter
					log.Error().Msg(mismatchErr.Error())
					mismatchErrorCount.Add(1)
				} else {
					// non-mismatch error: this is an exception, crash the process
					log.Fatal().Err(err).Msg("unexpected error during validation")
				}
			}
		}
	}
}

// validateRegister checks if the register store has the same register as the leaf node.
// It follows the same pattern as batchIndexRegisters but validates instead of indexing.
// Returns ErrMismatch for value mismatches, or other errors for exceptions.
func validateRegister(store execution.OnDiskRegisterStore, leafNode *wal.LeafNode, height uint64) error {
	payload := leafNode.Payload
	key, err := payload.Key()
	if err != nil {
		return fmt.Errorf("could not get key from register payload: %w", err)
	}

	registerID, err := convert.LedgerKeyToRegisterID(key)
	if err != nil {
		return fmt.Errorf("could not get register ID from key: %w", err)
	}

	// Get the expected value from the leaf node payload
	expectedValue := payload.Value()

	// Get the register value from the store at the given height
	storedValue, err := store.Get(registerID, height)
	if err != nil {
		if err == storage.ErrNotFound {
			// register not found is a mismatch error (expected register missing)
			return &ErrMismatch{
				RegisterID:     registerID,
				Height:         height,
				StoredLength:   0,
				ExpectedLength: len(expectedValue),
				StoredData:     nil,
				ExpectedData:   expectedValue,
				Message:        fmt.Sprintf("register not found in store: owner=%s, key=%s, height=%d", registerID.Owner, registerID.Key, height),
			}
		}
		// other store errors are exceptions
		return fmt.Errorf("failed to get register from store: owner=%s, key=%s, height=%d: %w", registerID.Owner, registerID.Key, height, err)
	}

	// Compare the stored value with the expected value
	if !bytes.Equal(storedValue, expectedValue) {
		// value mismatch is a mismatch error
		return &ErrMismatch{
			RegisterID:     registerID,
			Height:         height,
			StoredLength:   len(storedValue),
			ExpectedLength: len(expectedValue),
			StoredData:     storedValue,
			ExpectedData:   expectedValue,
		}
	}

	return nil
}
