package execution_indexer

import (
	"fmt"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

var _ module.ExecutionStateIndexer = &ExecutionIndexer{}

type ExecutionIndexer struct {
	registers   storage.Registers
	headers     storage.Headers
	last        uint64                          // todo persist
	commitments map[uint64]flow.StateCommitment // todo persist
}

func New(registers storage.Registers, headers storage.Headers) *ExecutionIndexer {
	return &ExecutionIndexer{
		registers:   registers,
		headers:     headers,
		last:        0,
		commitments: make(map[uint64]flow.StateCommitment),
	}
}

func (i *ExecutionIndexer) Last() (uint64, error) {
	return i.last, nil
}

func (i *ExecutionIndexer) StoreLast(last uint64) error {
	i.last = last
	return nil
}

func (i *ExecutionIndexer) HeightByBlockID(ID flow.Identifier) (uint64, error) {
	header, err := i.headers.ByBlockID(ID)
	if err != nil {
		return 0, err
	}

	return header.Height, nil
}

func (i *ExecutionIndexer) Commitment(height uint64) (flow.StateCommitment, error) {
	val, ok := i.commitments[height]
	if !ok {
		return flow.DummyStateCommitment, fmt.Errorf("could not find commitment at height %d", height)
	}

	return val, nil
}

func (i *ExecutionIndexer) StoreCommitment(commitment flow.StateCommitment, height uint64) error {
	i.commitments[height] = commitment
	return nil
}

func (i *ExecutionIndexer) Values(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
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

func (i *ExecutionIndexer) StorePayloads(payloads []*ledger.Payload, height uint64) error {
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
