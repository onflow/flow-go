package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

type Bootstrap struct {
	db         *pebble.DB
	rootHeight uint64
	done       chan struct{}
}

func NewBootstrap(db *pebble.DB, rootHeight uint64) *Bootstrap {
	// check for pre-populated heights, fail if it is populated
	// i.e. the IndexCheckpointFile function has already run for the db in this directory
	_, _, err := db.Get(latestHeightKey())
	if err == nil {
		// key detected, attempt to run bootstrap on corrupt or already bootstrapped data
		panic("found latest key set on badger instance, cannot bootstrap populated DB")
	}
	return &Bootstrap{
		db:         db,
		done:       make(chan struct{}),
		rootHeight: rootHeight,
	}
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
func (b *Bootstrap) IndexCheckpointFile(checkpointDir string) <-chan struct{} {
	// index checkpoint

	// update heights atomically in case one gets populated and the other doesn't,
	// leaving it in a corrupted state
	bat := b.db.NewBatch()
	err := bat.Set(firstHeightKey(), EncodedUint64(b.rootHeight), nil)
	if err != nil {

	}
	err = bat.Set(latestHeightKey(), EncodedUint64(b.rootHeight), nil)
	if err != nil {

	}
	err = bat.Commit(pebble.Sync)
	if err != nil {

	}
}

// indexCheckpointFileWorker asynchronously indexes register entries from wal.OpenAndReadLeafNodesFromCheckpointV6
func (b *Bootstrap) indexCheckpointFileWorker() <-chan bool {

}
