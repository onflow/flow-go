package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

type Bootstrap struct {
	db   *pebble.DB
	done chan struct{}
}

func NewBootstrap(db pebble.DB) *Bootstrap {
	return &Bootstrap{
		db:   db,
		done: make(chan struct{}),
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

func IndexCheckpointFile(checkpointDir string) chan struct{} {

}
