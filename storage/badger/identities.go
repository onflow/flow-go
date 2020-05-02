package badger

import (
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Identities struct {
	sync.Mutex
	db         *badger.DB
	identities flow.IdentityList
}

func NewIdentities(db *badger.DB) *Identities {
	i := &Identities{
		db:         db,
		identities: nil,
	}
	return i
}

func (i *Identities) ByBlockID(blockID flow.Identifier) (flow.IdentityList, error) {
	i.Lock()
	defer i.Unlock()

	// check if identities were already loaded
	if i.identities != nil {
		return i.identities, nil
	}

	// load identities
	var identities flow.IdentityList
	err := i.db.View(operation.RetrieveIdentities(&identities))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve identities: %w", err)
	}

	// cache identities
	i.identities = identities
	return identities, nil
}
