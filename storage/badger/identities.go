package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Identities struct {
	db         *badger.DB
	identities flow.IdentityList
}

func NewIdentities(db *badger.DB) (*Identities, error) {
	var identities flow.IdentityList
	err := db.View(operation.RetrieveIdentities(&identities))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve identities")
	}
	i := &Identities{
		db:         db,
		identities: identities,
	}
	return i, nil
}

func (i *Identities) ByBlockID(blockID flow.Identifier) (flow.IdentityList, error) {
	return i.identities, nil
}
