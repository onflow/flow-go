package pebble

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

// ErrAlreadyBootstrapped is the sentinel error for an already bootstrapped pebble instance
var ErrAlreadyBootstrapped = errors.New("found latest key set on badger instance, DB is already bootstrapped")

type RegisterBootstrap struct {
	log                zerolog.Logger
	db                 *pebble.DB
	checkpointDir      string
	checkpointFileName string
	leafNodeChan       chan *wal.LeafNode
	rootHeight         uint64
	registerCount      *atomic.Uint64
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
		log:                log.With().Str("module", "register_bootstrap").Logger(),
		db:                 db,
		checkpointDir:      checkpointDir,
		checkpointFileName: checkpointFileName,
		leafNodeChan:       make(chan *wal.LeafNode, checkpointLeafNodeBufSize),
		rootHeight:         rootHeight,
		registerCount:      atomic.NewUint64(0),
	}, nil
}

func (b *RegisterBootstrap) batchIndexRegisters(leafNodes []*wal.LeafNode) error {
	batch := b.db.NewBatch()
	defer batch.Close()

	b.log.Trace().Int("batch_size", len(leafNodes)).Msg("indexing batch of leaf nodes")
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

	b.registerCount.Add(uint64(len(leafNodes)))

	return nil
}

// indexCheckpointFileWorker asynchronously indexes register entries in b.checkpointDir
// with wal.OpenAndReadLeafNodesFromCheckpointV6
func (b *RegisterBootstrap) indexCheckpointFileWorker(ctx context.Context) error {
	b.log.Debug().Msg("started checkpoint index worker")

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
func (b *RegisterBootstrap) IndexCheckpointFile(ctx context.Context, workerCount int) error {
	cct, cancel := context.WithCancel(ctx)
	defer cancel()

	g, gCtx := errgroup.WithContext(cct)

	start := time.Now()
	b.log.Info().Msgf("indexing registers from checkpoint with %v worker", workerCount)
	for i := 0; i < workerCount; i++ {
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

	err = initHeights(b.db, b.rootHeight)
	if err != nil {
		return fmt.Errorf("could not index latest height: %w", err)
	}

	b.log.Info().
		Uint64("root_height", b.rootHeight).
		Uint64("register_count", b.registerCount.Load()).
		// note: not using Dur() since default units are ms and this duration is long
		Str("duration", fmt.Sprintf("%v", time.Since(start))).
		Msg("checkpoint indexing complete")

	return nil
}
