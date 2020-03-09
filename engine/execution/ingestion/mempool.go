package ingestion

//revive:disable:unexported-return

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
)

type Mempool struct {
	*stdmap.Backend
}

type blockByCollection struct {
	CollectionID flow.Identifier
	Block        *execution.CompleteBlock
}

func (b *blockByCollection) ID() flow.Identifier {

	return b.CollectionID
}

func (b *blockByCollection) Checksum() flow.Identifier {
	return b.CollectionID
}

type Backdata struct {
	*stdmap.Backdata
}

func (a *Backdata) ByID(id flow.Identifier) (*blockByCollection, error) {
	entity, err := a.Backdata.ByID(id)
	if err != nil {
		return nil, err
	}
	block, ok := entity.(*blockByCollection)
	if !ok {
		panic(fmt.Sprintf("invalid entity in complete block mempool  (%T)", entity))
	}
	return block, nil
}

func newMempool() (*Mempool, error) {
	a := &Mempool{
		Backend: stdmap.NewBackend(),
	}
	return a, nil
}

func (b *Mempool) Add(block *blockByCollection) error {
	return b.Backend.Add(block)
}

func (b *Mempool) Get(id flow.Identifier) (*blockByCollection, error) {
	backdata := &Backdata{&b.Backdata}
	return backdata.ByID(id)
}

func (b *Mempool) Run(f func(backdata *Backdata) error) error {
	b.RLock()
	defer b.RUnlock()

	err := f(&Backdata{&b.Backdata})
	if err != nil {
		return err
	}
	return nil
}
