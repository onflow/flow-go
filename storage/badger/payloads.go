package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type Payloads struct {
	db         *badger.DB
	index      *Index
	guarantees *Guarantees
	seals      *Seals
}

func NewPayloads(db *badger.DB, index *Index, guarantees *Guarantees, seals *Seals) *Payloads {

	p := &Payloads{
		db:         db,
		index:      index,
		guarantees: guarantees,
		seals:      seals,
	}

	return p
}

func (p *Payloads) storeTx(blockID flow.Identifier, payload *flow.Payload) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// make sure all payload guarantees are stored
		for _, guarantee := range payload.Guarantees {
			err := p.guarantees.storeTx(guarantee)(tx)
			if err != nil {
				return fmt.Errorf("could not store guarantee: %w", err)
			}
		}

		// make sure all payload seals are stored
		for _, seal := range payload.Seals {
			err := p.seals.storeTx(seal)(tx)
			if err != nil {
				return fmt.Errorf("could not store seal: %w", err)
			}
		}

		// store the index
		err := p.index.storeTx(blockID, payload.Index())(tx)
		if err != nil {
			return fmt.Errorf("could not store index: %w", err)
		}

		return nil
	}
}

func (p *Payloads) retrieveTx(blockID flow.Identifier) func(tx *badger.Txn) (*flow.Payload, error) {
	return func(tx *badger.Txn) (*flow.Payload, error) {

		// retrieve the index
		idx, err := p.index.retrieveTx(blockID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve index: %w", err)
		}

		// retrieve guarantees
		guarantees := make([]*flow.CollectionGuarantee, 0, len(idx.CollectionIDs))
		for _, collID := range idx.CollectionIDs {
			guarantee, err := p.guarantees.retrieveTx(collID)(tx)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve guarantee (%x): %w", collID, err)
			}
			guarantees = append(guarantees, guarantee)
		}

		// retrieve seals
		seals := make([]*flow.Seal, 0, len(idx.SealIDs))
		for _, sealID := range idx.SealIDs {
			seal, err := p.seals.retrieveTx(sealID)(tx)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve seal (%x): %w", sealID, err)
			}
			seals = append(seals, seal)
		}

		payload := &flow.Payload{
			Seals:      seals,
			Guarantees: guarantees,
		}

		return payload, nil
	}
}

func (p *Payloads) Store(blockID flow.Identifier, payload *flow.Payload) error {
	return operation.RetryOnConflict(p.db.Update, p.storeTx(blockID, payload))
}

func (p *Payloads) ByBlockID(blockID flow.Identifier) (*flow.Payload, error) {
	tx := p.db.NewTransaction(false)
	defer tx.Discard()
	return p.retrieveTx(blockID)(tx)
}
