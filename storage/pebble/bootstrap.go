package pebble

import (
	"fmt"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/rs/zerolog"
)

type Bootstrap struct {
	checkpointDir      string
	checkpointFileName string
	log                zerolog.Logger
	db                 *pebble.DB
	leafNodeChan       chan *wal.LeafNode
	rootHeight         uint64
}

func NewBootstrap(db *pebble.DB, checkpointFile string, rootHeight uint64, log zerolog.Logger) (*Bootstrap, error) {
	// check for pre-populated heights, fail if it is populated
	// i.e. the IndexCheckpointFile function has already run for the db in this directory
	checkpointDir, checkpointFileName := filepath.Split(checkpointFile)
	_, _, err := db.Get(latestHeightKey())
	if err == nil {
		// key detected, attempt to run bootstrap on corrupt or already bootstrapped data
		return nil, fmt.Errorf("found latest key set on badger instance, cannot bootstrap populated DB")
	}
	return &Bootstrap{
		checkpointDir:      checkpointDir,
		checkpointFileName: checkpointFileName,
		log:                log,
		db:                 db,
		leafNodeChan:       make(chan *wal.LeafNode, checkpointLeafNodeBufSize),
		rootHeight:         rootHeight,
	}, nil
}

func (b *Bootstrap) batchIndexRegisters(leafNodes []*wal.LeafNode) error {
	batch := b.db.NewBatch()
	defer batch.Close()
	for _, register := range leafNodes {
		payload := register.Payload
		key, err := payload.Key()
		if err != nil {
			return fmt.Errorf("could not get key from register payload: %w", err)
		}

		registerID, err := keyToRegisterID(key)
		if err != nil {
			return fmt.Errorf("could not get register ID from key: %w", err)
		}

		encoded := newLookupKey(b.rootHeight, registerID).Bytes()
		err = batch.Set(encoded, payload.Value(), nil)
		if err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}
	}
	err := batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}
	return nil
}

// indexCheckpointFileWorker asynchronously indexes register entries in b.checkpointDir
// with wal.OpenAndReadLeafNodesFromCheckpointV6
func (b *Bootstrap) indexCheckpointFileWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	select {
	case <-ctx.Done():
		return
	default:
	}
	// collect leaf nodes to batch index until the channel is closed
	batch := make([]*wal.LeafNode, 0, pebbleBootstrapRegisterBatchLen)
	for leafNode := range b.leafNodeChan {
		batch = append(batch, leafNode)
		if len(batch) >= pebbleBootstrapRegisterBatchLen {
			err := b.batchIndexRegisters(batch)
			if err != nil {
				ctx.Throw(fmt.Errorf("unable to index registers to pebble: %w", err))
			}
			batch = make([]*wal.LeafNode, 0, pebbleBootstrapRegisterBatchLen)
		}
	}
}

// IndexCheckpointFile indexes the checkpoint file in the Dir provided as a component
func (b *Bootstrap) IndexCheckpointFile(parentCtx irrecoverable.SignalerContext) error {
	// index checkpoint file async
	cmb := component.NewComponentManagerBuilder()
	for i := 0; i < pebbleBootstrapWorkerCount; i++ {
		// create workers to read and index registers
		cmb.AddWorker(b.indexCheckpointFileWorker)
	}
	err := wal.OpenAndReadLeafNodesFromCheckpointV6(b.leafNodeChan, b.checkpointDir, b.checkpointFileName, b.log)
	if err != nil {
		// error in reading a leaf node
		return fmt.Errorf("error reading leaf node: %w", err)
	}
	c := cmb.Build()
	c.Start(parentCtx)
	// wait for the indexing to finish before populating heights
	<-c.Done()
	bat := b.db.NewBatch()
	defer bat.Close()
	// update heights atomically to prevent one getting populated without the other
	// leaving it in a corrupted state
	err = bat.Set(firstHeightKey(), encodedUint64(b.rootHeight), nil)
	if err != nil {
		return fmt.Errorf("unable to add first height to batch: %w", err)
	}
	err = bat.Set(latestHeightKey(), encodedUint64(b.rootHeight), nil)
	if err != nil {
		return fmt.Errorf("unable to add latest height to batch: %w", err)
	}
	err = bat.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("unable to index first and latest heights: %w", err)
	}
	return nil
}
