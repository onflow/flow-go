package indexer

import (
	"fmt"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

type ExecutionState struct {
	registers   storage.Registers
	headers     storage.Headers
	events      storage.Events
	commitments map[uint64]flow.StateCommitment // todo persist
}

func New(registers storage.Registers, headers storage.Headers) *ExecutionState {
	return &ExecutionState{
		registers:   registers,
		headers:     headers,
		commitments: make(map[uint64]flow.StateCommitment),
	}
}

func (i *ExecutionState) HeightByBlockID(ID flow.Identifier) (uint64, error) {
	header, err := i.headers.ByBlockID(ID)
	if err != nil {
		return 0, err
	}

	return header.Height, nil
}

func (i *ExecutionState) Commitment(height uint64) (flow.StateCommitment, error) {
	val, ok := i.commitments[height]
	if !ok {
		return flow.DummyStateCommitment, fmt.Errorf("could not find commitment at height %d", height)
	}

	return val, nil
}

func (i *ExecutionState) RegisterValues(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	values := make([]flow.RegisterValue, len(IDs))

	for j, id := range IDs {
		entry, err := i.registers.Get(id, height)
		if err != nil {
			return nil, err
		}

		values[j] = entry.Value
	}

	return values, nil
}

func (i *ExecutionState) IndexBlockData(data *execution_data.BlockExecutionDataEntity) error {
	block, err := i.headers.ByBlockID(data.BlockID)
	if err != nil {
		return fmt.Errorf("could not get the block by ID %s: %w", data.BlockID, err)
	}

	// TODO concurrently process
	for j, chunk := range data.ChunkExecutionDatas {
		err := i.IndexEvents(data.BlockID, chunk.Events)
		if err != nil {
			return fmt.Errorf("could not index events for chunk %d: %w", j, err)
		}

		err = i.IndexCommitment(flow.StateCommitment(chunk.TrieUpdate.RootHash), block.Height)
		if err != nil {
			return fmt.Errorf("could not index events for chunk %d: %w", j, err)
		}

		err = i.IndexRegisterPayloads(chunk.TrieUpdate.Payloads, block.Height)
		if err != nil {
			return fmt.Errorf("could not index registers for chunk %d: %w", j, err)
		}
	}

	return nil
}

func (i *ExecutionState) IndexCommitment(commitment flow.StateCommitment, height uint64) error {
	i.commitments[height] = commitment
	return nil
}

func (i *ExecutionState) IndexEvents(blockID flow.Identifier, events flow.EventsList) error {
	// Note: service events are currently not included in execution data: https://github.com/onflow/flow-go/issues/4624
	return i.events.Store(blockID, []flow.EventsList{events})
}

func (i *ExecutionState) IndexRegisterPayloads(payloads []*ledger.Payload, height uint64) error {
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
