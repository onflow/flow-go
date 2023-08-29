package execution_indexer

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/engine/execution/scripts"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type ExecutionStateIndexer interface {
	ExecutionStateIndexReader
	ExecutionStateIndexWriter
}

type ExecutionStateIndexReader interface {
	// Last returns the last block height that was indexed.
	//
	// Expect an error if no value exists.
	Last() (uint64, error)
	// HeightByBlockID returns the block height by block ID.
	//
	// Expect an error if block by the ID was not indexed.
	HeightByBlockID(ID flow.Identifier) (uint64, error)
	// Commitment by the block height.
	//
	// Expect an error if commitment by block height was not indexed.
	Commitment(height uint64) (flow.StateCommitment, error)
	// Values retrieve register values for register IDs at given block height or by
	// the block height that is highest but still lower than the provided height.
	//
	// Expect an error if register was not indexed at all.
	Values(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error)
}

type ExecutionStateIndexWriter interface {
	// StorePayloads at the provided block height.
	//
	// Expect an error if we are trying to overwrite existing payloads at the provided height.
	StorePayloads(payloads []*ledger.Payload, height uint64) error
	// StoreCommitment at the provided block height.
	//
	// Expect an error if we are trying to overwrite existing commitments at provided height.
	StoreCommitment(commitment flow.StateCommitment, height uint64) error
	// StoreLast block height that was indexed.
	// Normal operation shouldn't produce an error.
	StoreLast(uint64) error
}

var _ ExecutionStateIndexer = &Indexer{}
var _ scripts.ScriptExecutionState = &Indexer{}

type Indexer struct {
	registers   storage.Registers
	headers     storage.Headers
	last        uint64                          // todo persist
	commitments map[uint64]flow.StateCommitment // todo persist
}

func New(registers storage.Registers, headers storage.Headers) *Indexer {
	return &Indexer{
		registers:   registers,
		headers:     headers,
		last:        0,
		commitments: make(map[uint64]flow.StateCommitment),
	}
}

func (i *Indexer) NewStorageSnapshot(commitment flow.StateCommitment) snapshot.StorageSnapshot {
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

func (i *Indexer) StateCommitmentByBlockID(
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

func (i *Indexer) HasState(commitment flow.StateCommitment) bool {
	for _, c := range i.commitments {
		if c == commitment {
			return true
		}
	}

	return false
}

func (i *Indexer) Last() (uint64, error) {
	return i.last, nil
}

func (i *Indexer) StoreLast(last uint64) error {
	i.last = last
	return nil
}

func (i *Indexer) HeightByBlockID(ID flow.Identifier) (uint64, error) {
	header, err := i.headers.ByBlockID(ID)
	if err != nil {
		return 0, err
	}

	return header.Height, nil
}

func (i *Indexer) Commitment(height uint64) (flow.StateCommitment, error) {
	val, ok := i.commitments[height]
	if !ok {
		return flow.DummyStateCommitment, fmt.Errorf("could not find commitment at height %d", height)
	}

	return val, nil
}

func (i *Indexer) StoreCommitment(commitment flow.StateCommitment, height uint64) error {
	i.commitments[height] = commitment
	return nil
}

func (i *Indexer) Values(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
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

func (i *Indexer) StorePayloads(payloads []*ledger.Payload, height uint64) error {

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
