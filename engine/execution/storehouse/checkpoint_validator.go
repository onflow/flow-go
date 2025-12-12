package storehouse

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

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

	// create N workers to validate registers in store
	cct, cancel := context.WithCancel(ctx)
	defer cancel()

	g, gCtx := errgroup.WithContext(cct)

	start := time.Now()
	log.Info().Msgf("validation registers from checkpoint with %v worker", workerCount)
	for i := 0; i < workerCount; i++ {
		g.Go(func() error {
			return validatingRegisterInStore(gCtx, store, leafNodeChan, blockHeight)
		})
	}

	rootHash, err := rootHashByHeight(results, headers, blockHeight)
	if err != nil {
		return err
	}

	// read leaf nodes from checkpoint file and send to leafNodeChan
	err = wal.OpenAndReadLeafNodesFromCheckpointV6(leafNodeChan, checkpointDir, "root.checkpoint", rootHash, log)
	if err != nil {
		return fmt.Errorf("error reading leaf node from checkpoint: %w", err)
	}

	if err = g.Wait(); err != nil {
		return fmt.Errorf("failed to validate registers from checkpoint file: %w", err)
	}

	log.Info().Msgf("finished validating registers from checkpoint in %s", time.Since(start))
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

func validatingRegisterInStore(ctx context.Context, store execution.OnDiskRegisterStore, leafNodeChan chan *wal.LeafNode, height uint64) error {
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
				return err
			}
		}
	}
}

// validateRegister checks if the register store has the same register as the leaf node.
// It follows the same pattern as batchIndexRegisters but validates instead of indexing.
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

	// Get the register value from the store at the given height
	storedValue, err := store.Get(registerID, height)
	if err != nil {
		if err == storage.ErrNotFound {
			return fmt.Errorf("register not found in store: owner=%s, key=%s, height=%d", registerID.Owner, registerID.Key, height)
		}
		return fmt.Errorf("failed to get register from store: owner=%s, key=%s, height=%d: %w", registerID.Owner, registerID.Key, height, err)
	}

	// Get the expected value from the leaf node payload
	expectedValue := payload.Value()

	// Compare the stored value with the expected value
	if !bytes.Equal(storedValue, expectedValue) {
		return fmt.Errorf("register value mismatch: owner=%s, key=%s, height=%d, stored_length=%d, expected_length=%d",
			registerID.Owner, registerID.Key, height, len(storedValue), len(expectedValue))
	}

	return nil
}
