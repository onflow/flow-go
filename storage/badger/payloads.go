// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Payloads implements a simple read-only payload storage around a badger DB.
type Payloads struct {
	db *badger.DB
}

func NewPayloads(db *badger.DB) *Payloads {
	p := &Payloads{
		db: db,
	}
	return p
}

func (p *Payloads) Store(header *flow.Header, payload *flow.Payload) error {
	return p.db.Update(func(tx *badger.Txn) error {
		if header.PayloadHash != payload.Hash() {
			return fmt.Errorf("payload integrity check failed")
		}
		err := procedure.InsertPayload(payload)(tx)
		if err != nil {
			return fmt.Errorf("could not insert payload: %w", err)
		}
		err = procedure.IndexPayload(header, payload)(tx)
		if err != nil {
			return fmt.Errorf("could not index payload: %w", err)
		}
		return nil
	})
}

func (p *Payloads) ByBlockID(blockID flow.Identifier) (*flow.Payload, error) {
	var payload flow.Payload
	err := p.db.View(procedure.RetrievePayload(blockID, &payload))
	if err != nil {
		return nil, err
	}

	return &payload, nil
}
