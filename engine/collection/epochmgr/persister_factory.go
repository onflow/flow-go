package epochmgr

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/persister"
	"github.com/dapperlabs/flow-go/model/flow"
)

type PersisterFactory struct {
	db *badger.DB
}

func NewPersisterFactory(db *badger.DB) (*PersisterFactory, error) {
	factory := &PersisterFactory{db: db}
	return factory, nil
}

func (f *PersisterFactory) New(clusterID flow.ChainID) (*persister.Persister, error) {
	persist := persister.New(f.db, clusterID)
	return persist, nil
}
