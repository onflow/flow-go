package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/storage"
)

type Registers struct {
	db *badger.DB
}

var _ storage.Registers = &Registers{}

func NewRegisters(db *badger.DB) *Registers {
	return &Registers{db: db}
}

func (r Registers) Store(path ledger.Path, payload *ledger.Payload) error {
	//TODO implement me
	panic("implement me")
}

func (r Registers) ByPath(path ledger.Path) (*ledger.Payload, error) {
	//TODO implement me
	panic("implement me")
}
