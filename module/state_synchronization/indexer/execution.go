package indexer

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

// ExecutionState indexes the execution state.
type ExecutionState struct {
	registers  storage.RegisterIndex
	headers    storage.Headers
	events     storage.Events
	indexRange *SequentialIndexRange
	log        zerolog.Logger
}

// New execution state indexer used to ingest block execution data and index it by height.
// The passed RegisterIndex storage must be populated to include the first and last height otherwise the indexer
// won't be initialized to ensure we have bootstrapped the storage first.
func New(registers storage.RegisterIndex, headers storage.Headers, events storage.Events, log zerolog.Logger) (*ExecutionState, error) {
	// we must have the first height set in the storage otherwise error out
	first, err := registers.FirstHeight()
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize the indexer with missing first height")
	}

	// we must have the latest height set in the storage otherwise error out
	last, err := registers.LatestHeight()
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize the indexer with missing latest height")
	}

	indexRange, err := NewSequentialIndexRange(first, last)
	if err != nil {
		return nil, err
	}

	log.Debug().Msgf("initialized indexer with range first: %d, last: %d", first, last)

	return &ExecutionState{
		registers:  registers,
		headers:    headers,
		events:     events,
		indexRange: indexRange,
		log:        log.With().Str("component", "execution_indexer").Logger(),
	}, nil
}

// RegisterValues retrieves register values by the register IDs at the provided block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// Expected errors:
// - storage.ErrNotFound if the register by the ID was never indexed
// - ErrIndexBoundary if the height is out of indexed height boundary
func (i *ExecutionState) RegisterValues(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	_, err := i.indexRange.Contained(height)
	if err != nil {
		return nil, err
	}

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
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	block, err := i.headers.ByBlockID(data.BlockID)
	if err != nil {
		// todo should we wrap sentinel error or should we use irrecoverable.exception as per design guidelines https://github.com/onflow/flow-go/blob/master/CodingConventions.md
		return fmt.Errorf("could not get the block by ID %s: %w", data.BlockID, err)
	}

	i.log.Debug().Msgf("indexing new block ID %s at height %d", data.BlockID.String(), block.Height)

	if _, err := i.indexRange.CanIncrease(block.Height); err != nil {
		return err
	}

	// concurrently process indexing of block data
	g, _ := errgroup.WithContext(ctx)

	payloads := make(map[ledger.Path]*ledger.Payload)
	events := make([]flow.Event, 0)

	for _, chunk := range data.ChunkExecutionDatas {
		if chunk.TrieUpdate != nil {
			// we are iterating all the registers and overwrite any existing register at the same path
			// this will make sure if we have multiple register changes only the last change will get persisted
			// if block has two chucks:
			// first chunk updates: { X: 1, Y: 2 }
			// second chunk updates: { X: 2 }
			// then we should persist only {X: 2: Y: 2}
			for i, path := range chunk.TrieUpdate.Paths {
				payloads[path] = chunk.TrieUpdate.Payloads[i]
			}
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
		err = i.indexRegisterPayloads(maps.Values(payloads), block.Height)
		if err != nil {
			return fmt.Errorf("could not index register payloads at height %d: %w", block.Height, err)
		}
		return nil
	})

	err = g.Wait()
	if err != nil {
		return fmt.Errorf("failed to index block data at height %d: %w", block.Height, err)
	}

	i.log.Debug().Uint64("height", block.Height).Int("register count", len(payloads)).Msgf("indexed block data")

	return i.indexRange.Increase(block.Height)
}

func (i *ExecutionState) indexEvents(blockID flow.Identifier, events flow.EventsList) error {
	// Note: service events are currently not included in execution data: https://github.com/onflow/flow-go/issues/4624
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
