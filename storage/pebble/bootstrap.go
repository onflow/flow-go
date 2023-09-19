package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

type Bootstrap struct {
	checkpointDir string
	db            *pebble.DB
	done          chan struct{}
	rootHeight    uint64
}

func NewBootstrap(db *pebble.DB, checkpointDir string, rootHeight uint64) (*Bootstrap, error) {
	// check for pre-populated heights, fail if it is populated
	// i.e. the IndexCheckpointFile function has already run for the db in this directory
	_, _, err := db.Get(latestHeightKey())
	if err == nil {
		// key detected, attempt to run bootstrap on corrupt or already bootstrapped data
		return nil, fmt.Errorf("found latest key set on badger instance, cannot bootstrap populated DB")
	}
	return &Bootstrap{
		checkpointDir: checkpointDir,
		db:            db,
		done:          make(chan struct{}),
		rootHeight:    rootHeight,
	}, nil
}

func (b *Bootstrap) batchIndexRegisters(height uint64, registers []*wal.LeafNode) error {
	// collect entries
	batch := b.db.NewBatch()
	defer batch.Close()
	for _, register := range registers {
		payload := register.Payload
		key, err := payload.Key()
		if err != nil {
			return fmt.Errorf("could not get key from register payload: %w", err)
		}

		registerID, err := registerIDFromPayloadKey(key)
		if err != nil {
			return fmt.Errorf("could not get register ID from key: %w", err)
		}

		encoded := newLookupKey(height, registerID).Bytes()
		err = batch.Set(encoded, payload.Value(), nil)
		if err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}
	}
	// batch insert to db
	err := batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}
	return nil
}

// IndexCheckpointFile indexes the checkpoint file in the Dir provided and returns a channel that closes when done
func (b *Bootstrap) IndexCheckpointFile() <-chan error {
	c := make(chan error, 1)
	go func() {
		bat := b.db.NewBatch()

		defer func() {
			close(c)
			err := bat.Close()
			if err != nil {

			}
		}()
		// index checkpoint file async
		doneIndex := b.indexCheckpointFileWorker()
		err := <-doneIndex
		if err != nil {
			c <- fmt.Errorf("failed to index checkpoint files: %w", err)
			return
		}
		// update heights atomically to prevent one getting populated without the other
		// leaving it in a corrupted state
		err = bat.Set(firstHeightKey(), EncodedUint64(b.rootHeight), nil)
		if err != nil {
			c <- fmt.Errorf("failed to set first height %w", err)
			return
		}
		err = bat.Set(latestHeightKey(), EncodedUint64(b.rootHeight), nil)
		if err != nil {
			c <- fmt.Errorf("failed to set latest height %w", err)
			return
		}
		err = bat.Commit(pebble.Sync)
		if err != nil {
			c <- fmt.Errorf("failed to commit height updates %w", err)
			return
		}
	}()
	return c
}

// indexCheckpointFileWorker asynchronously indexes register entries in b.checkpointDir
// with wal.OpenAndReadLeafNodesFromCheckpointV6
func (b *Bootstrap) indexCheckpointFileWorker() <-chan error {
	panic("not implemented")
}
