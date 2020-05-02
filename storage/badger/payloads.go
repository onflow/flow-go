// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Payloads implements a simple read-only payload storage around a badger DB.
type Payloads struct {
	db    *badger.DB
	cache *Cache
}

func NewPayloads(db *badger.DB) *Payloads {

	store := func(blockID flow.Identifier, payload interface{}) error {
		return db.Update(procedure.InsertPayload(blockID, payload.(*flow.Payload)))
	}

	retrieve := func(blockID flow.Identifier) (interface{}, error) {
		var payload flow.Payload
		err := db.View(procedure.RetrievePayload(blockID, &payload))
		return &payload, err
	}

	p := &Payloads{
		db:    db,
		cache: newCache(withStore(store), withRetrieve(retrieve)),
	}

	return p
}

func (p *Payloads) Store(blockID flow.Identifier, payload *flow.Payload) error {
	return p.cache.Put(blockID, payload)
}

func (p *Payloads) ByBlockID(blockID flow.Identifier) (*flow.Payload, error) {
	payload, err := p.cache.Get(blockID)
	return payload.(*flow.Payload), err
}
