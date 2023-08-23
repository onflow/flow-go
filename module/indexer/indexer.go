package indexer

import (
	"fmt"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

var _ module.Indexer = &Indexer{}

type Indexer struct {
	registers   storage.Registers
	headers     storage.Headers
	last        uint64                          // todo persist
	commitments map[uint64]flow.StateCommitment // todo persist
}

func (i *Indexer) Last() (uint64, error) {
	return i.last, nil
}

func (i *Indexer) StoreLast(last uint64) error {
	i.last = last
	return nil
}

func (i *Indexer) HeightForBlock(ID flow.Identifier) (uint64, error) {
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

	for j, id := range IDs {
		val, err := i.registers.Get(id, height)
		if err != nil {
			return nil, err
		}

		values[j] = val
	}

	return values, nil
}

func (i *Indexer) StorePayloads(payloads []*ledger.Payload, height uint64) error {

	for _, payload := range payloads {
		k, err := payload.Key()
		if err != nil {
			return err
		}
		id, err := migrations.KeyToRegisterID(k)
		if err != nil {
			return err
		}

		regEntry := flow.RegisterEntry{
			Key:   id,
			Value: payload.Value(),
		}

		err = i.registers.Store(regEntry, height) // TODO add batch store
		if err != nil {
			return err
		}
	}

	return nil
}
