// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

type Seals struct {
	db *badger.DB
}

func NewSeals(db *badger.DB) *Seals {
	s := &Seals{db: db}
	return s
}

func (s *Seals) Store(seal *flow.Seal) error {
	return s.db.Update(func(tx *badger.Txn) error {
		err := operation.InsertSeal(seal)(tx)
		if err != nil {
			return fmt.Errorf("could not insert seal: %w", err)
		}
		return nil
	})
}

func (s *Seals) ByID(sealID flow.Identifier) (*flow.Seal, error) {
	var seal flow.Seal
	err := s.db.View(func(tx *badger.Txn) error {
		return operation.RetrieveSeal(sealID, &seal)(tx)
	})
	if err != nil {
		return nil, fmt.Errorf("could not retrieve seal: %w", err)
	}
	return &seal, nil
}

func (s *Seals) ByBlockID(blockID flow.Identifier) ([]*flow.Seal, error) {
	var seals []*flow.Seal
	err := s.db.View(func(tx *badger.Txn) error {
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not get header: %w", err)
		}
		err = procedure.RetrieveSeals(header.PayloadHash, &seals)(tx)
		if err != nil {
			return fmt.Errorf("could not get seals: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not retrieve seals for block: %w", err)
	}
	return seals, nil
}
