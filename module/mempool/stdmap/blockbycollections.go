// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
)

type BlockByCollections struct {
	*Backend
}

type BlockByCollectionBackdata struct {
	*Backdata
}

func NewBlockByCollections() *BlockByCollections {
	return &BlockByCollections{NewBackend(WithEject(EjectPanic))}
}

func (b *BlockByCollections) Add(block *entity.BlockByCollection) error {
	return b.Backend.Add(block)
}

func (b *BlockByCollections) Get(id flow.Identifier) (*entity.BlockByCollection, error) {
	backdata := &BlockByCollectionBackdata{&b.Backdata}
	return backdata.ByID(id)
}

func (b *BlockByCollections) Run(f func(backdata *BlockByCollectionBackdata) error) error {
	b.Lock()
	defer b.Unlock()

	err := f(&BlockByCollectionBackdata{&b.Backdata})
	if err != nil {
		return err
	}
	return nil
}

func (b *BlockByCollectionBackdata) ByID(id flow.Identifier) (*entity.BlockByCollection, error) {
	e, err := b.Backdata.ByID(id)
	if err != nil {
		return nil, err
	}
	block, ok := e.(*entity.BlockByCollection)
	if !ok {
		panic(fmt.Sprintf("invalid entity in complete block mempool (%T)", e))
	}
	return block, nil
}
