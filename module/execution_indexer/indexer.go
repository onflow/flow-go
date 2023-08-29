package execution_indexer

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/engine/execution/scripts"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

var _ module.ExecutionStateIndexer = &indexer{}
var _ scripts.ScriptExecutionState = &indexer{}

type indexer struct {
	registers   storage.Registers
	headers     storage.Headers
	last        uint64                          // todo persist
	commitments map[uint64]flow.StateCommitment // todo persist
}

func New(registers storage.Registers, headers storage.Headers) *indexer {
	return &indexer{
		registers:   registers,
		headers:     headers,
		last:        0,
		commitments: make(map[uint64]flow.StateCommitment),
	}
}

func (i *indexer) NewStorageSnapshot(commitment flow.StateCommitment) snapshot.StorageSnapshot {
	var height uint64
	for h, commit := range i.commitments {
		if commit == commitment {
			height = h
		}
	}

	if height == 0 {

	}

	reader := func(id flow.RegisterID) (flow.RegisterValue, error) {
		entry, err := i.registers.Get(id, height)
		if err != nil {
			return nil, err
		}

		return entry.Value, nil
	}

	return snapshot.NewReadFuncStorageSnapshot(reader)
}

func (i *indexer) StateCommitmentByBlockID(
	ctx context.Context,
	identifier flow.Identifier,
) (flow.StateCommitment, error) {
	h, err := i.headers.ByBlockID(identifier)
	if err != nil {
		return flow.DummyStateCommitment, fmt.Errorf("could not find block by ID %s for state commitment: %w", identifier.String(), err)
	}

	commit, ok := i.commitments[h.Height]
	if !ok {
		return flow.DummyStateCommitment, fmt.Errorf("could not find the state commitment by height %d", h.Height)
	}

	return commit, nil
}

func (i *indexer) HasState(commitment flow.StateCommitment) bool {
	for _, c := range i.commitments {
		if c == commitment {
			return true
		}
	}

	return false
}

func (i *indexer) Last() (uint64, error) {
	return i.last, nil
}

func (i *indexer) StoreLast(last uint64) error {
	i.last = last
	return nil
}

func (i *indexer) HeightByBlockID(ID flow.Identifier) (uint64, error) {
	header, err := i.headers.ByBlockID(ID)
	if err != nil {
		return 0, err
	}

	return header.Height, nil
}

func (i *indexer) Commitment(height uint64) (flow.StateCommitment, error) {
	val, ok := i.commitments[height]
	if !ok {
		return flow.DummyStateCommitment, fmt.Errorf("could not find commitment at height %d", height)
	}

	return val, nil
}

func (i *indexer) StoreCommitment(commitment flow.StateCommitment, height uint64) error {
	i.commitments[height] = commitment
	return nil
}

func (i *indexer) Values(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	values := make([]flow.RegisterValue, len(IDs))

	// TODO batch retrieve
	for j, id := range IDs {
		entry, err := i.registers.Get(id, height)
		if err != nil {
			return nil, err
		}

		values[j] = entry.Value
	}

	return values, nil
}

func (i *indexer) StorePayloads(payloads []*ledger.Payload, height uint64) error {

	// TODO add batch store
	for _, payload := range payloads {
		k, err := payload.Key()
		if err != nil {
			return err
		}

		// TODO make sure we can use the payload encKey instead of the paths,
		// is the key encoded in the payload key convertable the same way?
		id, err := migrations.KeyToRegisterID(k)
		if err != nil {
			return err
		}

		regEntries := flow.RegisterEntries{{
			Key:   id,
			Value: payload.Value(),
		}}

		err = i.registers.Store(regEntries, height)
		if err != nil {
			return err
		}
	}

	return nil
}
