package indexer

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

// ExecutionState indexes the execution state.
type ExecutionState struct {
	registers         storage.Registers
	headers           storage.Headers
	events            storage.Events
	commitments       map[uint64]flow.StateCommitment // todo persist
	startIndexHeight  uint64
	lastIndexedHeight counters.SequentialCounter
}

func New(registers storage.Registers, headers storage.Headers, startIndexHeight uint64) *ExecutionState {
	return &ExecutionState{
		registers:         registers,
		headers:           headers,
		commitments:       make(map[uint64]flow.StateCommitment),
		startIndexHeight:  startIndexHeight,
		lastIndexedHeight: counters.NewSequentialCounter(startIndexHeight),
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

// Commitment retrieves a commitment at the provided block height.
// If a commitment at height is not found expect a `storage.ErrNotFound` error.
func (i *ExecutionState) Commitment(height uint64) (flow.StateCommitment, error) {
	if height < i.startIndexHeight || height > i.lastIndexedHeight.Value() {
		return flow.DummyStateCommitment, fmt.Errorf(
			"state commitment out of indexed height bounds, current height range: [%d, %d], requested height: %d",
			i.startIndexHeight,
			i.lastIndexedHeight.Value(),
			height,
		)
	}

	val, ok := i.commitments[height]
	if !ok {
		return flow.DummyStateCommitment, fmt.Errorf("could not find commitment at height %d", height)
	}

	return val, nil
}

// RegisterValues retrieves register values by the register IDs at the provided block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// If the register was not found the storage.ErrNotFound error is returned.
func (i *ExecutionState) RegisterValues(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	if height < i.startIndexHeight || height > i.lastIndexedHeight.Value() {
		return nil, fmt.Errorf(
			"register out of indexed height bounds, current height range: [%d, %d], requested height: %d",
			i.startIndexHeight,
			i.lastIndexedHeight,
			height,
		)
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

	// todo note: should we silently drop already existing heights or should we fail - look into jobqueue worker failure handling, if nothing else we should log
	if !i.lastIndexedHeight.Set(block.Height) {
		return nil
	}

	// concurrently process indexing of block data
	g, ctx := errgroup.WithContext(ctx)
	for j, chunk := range data.ChunkExecutionDatas {
		g.Go(func() error {
			err := i.indexEvents(data.BlockID, chunk.Events)
			if err != nil {
				return fmt.Errorf("could not index events for chunk %d: %w", j, err)
			}
			return nil
		})
		g.Go(func() error {
			err = i.indexCommitment(flow.StateCommitment(chunk.TrieUpdate.RootHash), block.Height)
			if err != nil {
				return fmt.Errorf("could not index events for chunk %d: %w", j, err)
			}
			return nil
		})
		g.Go(func() error {
			err = i.indexRegisterPayload(chunk.TrieUpdate.Payloads, block.Height)
			if err != nil {
				return fmt.Errorf("could not index registers for chunk %d: %w", j, err)
			}
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return err
	}

	// progress height in storage before finishing
	return i.registers.SetLatestHeight(block.Height)
}

func (i *ExecutionState) indexCommitment(commitment flow.StateCommitment, height uint64) error {
	i.commitments[height] = commitment
	return nil
}

func (i *ExecutionState) indexEvents(blockID flow.Identifier, events flow.EventsList) error {
	// Note: service events are currently not included in execution data: https://github.com/onflow/flow-go/issues/4624
	return i.events.Store(blockID, []flow.EventsList{events})
}

func (i *ExecutionState) indexRegisterPayload(payloads []*ledger.Payload, height uint64) error {
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
