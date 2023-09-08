package indexer

import (
	"context"
	"fmt"

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
	registers storage.RegisterIndex
	headers   storage.Headers
	events    storage.Events
}

func New(registers storage.RegisterIndex, headers storage.Headers, startIndexHeight uint64) *ExecutionState {
	return &ExecutionState{
		registers: registers,
		headers:   headers,
	}
}

// HeightByBlockID retrieves the height for block ID.
// If a block is not found expect a storage.ErrNotFound error.
func (i *ExecutionState) HeightByBlockID(ID flow.Identifier) (uint64, error) {
	header, err := i.headers.ByBlockID(ID)
	if err != nil {
		return 0, fmt.Errorf("could not find block by ID %: %w", ID, err)
	}

	return header.Height, nil
}

// RegisterValues retrieves register values by the register IDs at the provided block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// Expected errors:
// - storage.ErrNotFound if the register by the ID was never indexed
// - ErrHeightBoundary if the height is out of indexed height boundary
func (i *ExecutionState) RegisterValues(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	err := i.readBoundaryCheck(height)
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
// If the height was already indexed the operation will be ignored.
// This method shouldn't be used concurrently.
func (i *ExecutionState) IndexBlockData(ctx context.Context, data *execution_data.BlockExecutionDataEntity) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	block, err := i.headers.ByBlockID(data.BlockID)
	if err != nil {
		return fmt.Errorf("could not get the block by ID %s: %w", data.BlockID, err)
	}

	// concurrently process indexing of block data
	g, ctx := errgroup.WithContext(ctx)

	payloads := make(map[ledger.Path]*ledger.Payload)
	events := make([]flow.Event, 0)
	collections := make([]*flow.Collection, 0)

	for _, chunk := range data.ChunkExecutionDatas {
		// we are iterating all the registers and overwrite any existing register at the same path
		// this will make sure if we have multiple register changes only the last change will get persisted
		// if block has two chucks:
		// first chunk updates: { X: 1, Y: 2 }
		// second chunk updates: { X: 2 }
		// then we should persist only {X: 2: Y: 2}
		for i, path := range chunk.TrieUpdate.Paths {
			payloads[path] = chunk.TrieUpdate.Payloads[i] // todo should we use TrieUpdate.Paths or TrieUpdate.Payload.Key
		}

		events = append(events, chunk.Events...)
		collections = append(collections, chunk.Collection)
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

	return nil
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

var ErrHeightBoundary = fmt.Errorf("the height is not within the heights of the indexed interval")

func (i *ExecutionState) readBoundaryCheck(height uint64) error {
	first, err := i.registers.FirstHeight()
	if err != nil {
		return fmt.Errorf("failed to read first index height: %w", err)
	}

	last, err := i.registers.LatestHeight()
	if err != nil {
		return fmt.Errorf("failed to read last index height: %w", err)
	}

	if height < first || height > last {
		return fmt.Errorf("height %d is out of boundary [%d - %d]: %w", height, first, last, ErrHeightBoundary)
	}

	return nil
}
