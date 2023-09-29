package pebble

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

// ErrAlreadyBootstrapped is the sentinel error for an already bootstrapped pebble instance
var ErrAlreadyBootstrapped = fmt.Errorf("found latest key set on badger instance, DB is already bootstrapped")

type RegisterBootstrap struct {
	checkpointDir      string
	checkpointFileName string
	log                zerolog.Logger
	db                 *pebble.DB
	leafNodeChan       chan *wal.LeafNode
	rootHeight         uint64
}

// NewRegisterBootstrap creates the bootstrap object for reading checkpoint data and the height tracker in pebble
// This object must be initialized and RegisterBootstrap.IndexCheckpointFile must be run to have the pebble db instance
// in the correct state to initialize a Registers store.
func NewRegisterBootstrap(
	db *pebble.DB,
	checkpointFile string,
	rootHeight uint64,
	log zerolog.Logger,
) (*RegisterBootstrap, error) {
	// check for pre-populated heights, fail if it is populated
	// i.e. the IndexCheckpointFile function has already run for the db in this directory
	isBootstrapped, err := IsBootstrapped(db)
	if err != nil {
		return nil, err
	}
	if isBootstrapped {
		// key detected, attempt to run bootstrap on corrupt or already bootstrapped data
		return nil, ErrAlreadyBootstrapped
	}
	checkpointDir, checkpointFileName := filepath.Split(checkpointFile)
	return &RegisterBootstrap{
		checkpointDir:      checkpointDir,
		checkpointFileName: checkpointFileName,
		log:                log.With().Str("module", "register_bootstrap").Logger(),
		db:                 db,
		leafNodeChan:       make(chan *wal.LeafNode, checkpointLeafNodeBufSize),
		rootHeight:         rootHeight,
	}, nil
}

func (b *RegisterBootstrap) batchIndexRegisters(leafNodes []*wal.LeafNode) error {
	b.log.Debug().Int("batch_size", len(leafNodes)).Msg("indexing batch of leaf nodes")
	batch := b.db.NewBatch()
	defer batch.Close()
	for _, register := range leafNodes {
		payload := register.Payload
		key, err := payload.Key()
		if err != nil {
			return fmt.Errorf("could not get key from register payload: %w", err)
		}

		registerID, err := convert.LedgerKeyToRegisterID(key)
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
func (b *RegisterBootstrap) indexCheckpointFileWorker(ctx context.Context) error {
	b.log.Info().Msg("started checkpoint index worker")
	// collect leaf nodes to batch index until the channel is closed
	batch := make([]*wal.LeafNode, 0, pebbleBootstrapRegisterBatchLen)
	for leafNode := range b.leafNodeChan {
		select {
		case <-ctx.Done():
			return nil
		default:
			batch = append(batch, leafNode)
			if len(batch) >= pebbleBootstrapRegisterBatchLen {
				err := b.batchIndexRegisters(batch)
				if err != nil {
					return fmt.Errorf("unable to index registers to pebble in batch: %w", err)
				}
				batch = make([]*wal.LeafNode, 0, pebbleBootstrapRegisterBatchLen)
			}
		}
	}
	// index the remaining registers if didn't reach a batch length.
	err := b.batchIndexRegisters(batch)
	if err != nil {
		return fmt.Errorf("unable to index remaining registers to pebble: %w", err)
	}
	return nil
}

// IndexCheckpointFile indexes the checkpoint file in the Dir provided
func (b *RegisterBootstrap) IndexCheckpointFile(ctx context.Context) error {
	cct, cancel := context.WithCancel(ctx)
	defer cancel()
	g, gCtx := errgroup.WithContext(cct)
	b.log.Info().Msg("indexing checkpoint file for pebble register store")
	for i := 0; i < pebbleBootstrapWorkerCount; i++ {
		g.Go(func() error {
			return b.indexCheckpointFileWorker(gCtx)
		})
	}
	err := wal.OpenAndReadLeafNodesFromCheckpointV6(b.leafNodeChan, b.checkpointDir, b.checkpointFileName, b.log)
	if err != nil {
		return fmt.Errorf("error reading leaf node: %w", err)
	}
	if err = g.Wait(); err != nil {
		return fmt.Errorf("failed to index checkpoint file: %w", err)
	}
	b.log.Info().Msg("checkpoint indexing complete")
	err = initHeights(b.db, b.rootHeight)
	if err != nil {
		return fmt.Errorf("could not index latest height: %w", err)
	}
	return nil
}
