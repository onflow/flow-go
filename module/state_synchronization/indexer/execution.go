package indexer

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// ExecutionState indexes the execution state.
type ExecutionState struct {
	registers storage.RegisterIndex
	headers   storage.Headers
	events    storage.Events
	log       zerolog.Logger
}

// New execution state indexer used to ingest block execution data and index it by height.
// The passed RegisterIndex storage must be populated to include the first and last height otherwise the indexer
// won't be initialized to ensure we have bootstrapped the storage first.
func New(
	log zerolog.Logger,
	registers storage.RegisterIndex,
	headers storage.Headers,
	events storage.Events,
) (*ExecutionState, error) {
	return &ExecutionState{
		registers: registers,
		headers:   headers,
		events:    events,
		log:       log.With().Str("component", "execution_indexer").Logger(),
	}, nil
}

// RegisterValues retrieves register values by the register IDs at the provided block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// Expected errors:
// - storage.ErrNotFound if the register by the ID was never indexed
// - ErrIndexBoundary if the height is out of indexed height boundary
func (i *ExecutionState) RegisterValues(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	values := make([]flow.RegisterValue, len(IDs))

	for j, id := range IDs {
		value, err := i.registers.Get(id, height)
		if err != nil {
			return nil, err
		}

		values[j] = value
	}

	return values, nil
}

// IndexBlockData indexes all execution block data by height.
// This method shouldn't be used concurrently.
// Expected errors:
// - ErrIndexValue if height was not incremented in sequence
// - storage.ErrNotFound if the block for execution data was not found
func (i *ExecutionState) IndexBlockData(ctx context.Context, data *execution_data.BlockExecutionDataEntity) error {
	block, err := i.headers.ByBlockID(data.BlockID)
	if err != nil {
		return fmt.Errorf("could not get the block by ID %s: %w", data.BlockID, err)
	}

	lg := i.log.With().
		Hex("block_id", logging.ID(data.BlockID)).
		Uint64("height", block.Height).
		Logger()

	lg.Debug().Msgf("indexing new block")

	latest, err := i.registers.LatestHeight()
	if err != nil {
		return err
	}

	if block.Height == latest {
		lg.Warn().Uint64("height", block.Height).Msg("indexing already indexed block")
		return nil
	}

	if block.Height != latest+1 {
		return fmt.Errorf("must store registers with the next height %d, but got %d", latest+1, block.Height)
	}

	// concurrently process indexing of block data
	g := errgroup.Group{}

	updates := make([]*ledger.TrieUpdate, 0)
	events := make([]flow.Event, 0)

	for _, chunk := range data.ChunkExecutionDatas {
		if chunk.TrieUpdate != nil {
			updates = append(updates, chunk.TrieUpdate)
		}

		events = append(events, chunk.Events...)
	}

	if len(events) > 0 {
		g.Go(func() error {
			err := i.indexEvents(data.BlockID, events)
			if err != nil {
				return fmt.Errorf("could not index events at height %d: %w", block.Height, err)
			}
			return nil
		})
	}

	g.Go(func() error {
		// we are iterating all the registers and overwrite any existing register at the same path
		// this will make sure if we have multiple register changes only the last change will get persisted
		// if block has two chucks:
		// first chunk updates: { X: 1, Y: 2 }
		// second chunk updates: { X: 2 }
		// then we should persist only {X: 2: Y: 2}

		payloads := make(map[ledger.Path]*ledger.Payload)
		for _, update := range updates {
			for i, path := range update.Paths {
				payloads[path] = update.Payloads[i] // todo should we use TrieUpdate.Paths or TrieUpdate.Payload.Key?
			}
		}

		err = i.indexRegisterPayloads(maps.Values(payloads), block.Height)
		if err != nil {
			return fmt.Errorf("could not index register payloads at height %d: %w", block.Height, err)
		}

		lg.Debug().
			Int("register_count", len(payloads)).
			Msg("indexed registers")

		return nil
	})

	err = g.Wait()
	if err != nil {
		return fmt.Errorf("failed to index block data at height %d: %w", block.Height, err)
	}
	return nil
}

func (i *ExecutionState) indexEvents(blockID flow.Identifier, events flow.EventsList) error {
	// Note: the last chunk in an execution data is the system chunk. All events in that ChunkExecutionData are service events.
	return i.events.Store(blockID, []flow.EventsList{events})
}

func (i *ExecutionState) indexRegisterPayloads(payloads []*ledger.Payload, height uint64) error {
	regEntries := make(flow.RegisterEntries, len(payloads))

	for j, payload := range payloads {
		k, err := payload.Key()
		if err != nil {
			return err
		}

		id, err := migrations.KeyToRegisterID(k)
		if err != nil {
			return err
		}

		regEntries[j] = flow.RegisterEntry{
			Key:   id,
			Value: payload.Value(),
		}
	}

	return i.registers.Store(regEntries, height)
}
