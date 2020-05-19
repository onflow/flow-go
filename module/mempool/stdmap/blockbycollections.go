// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
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

func (b *BlockByCollections) Add(block *entity.BlocksByCollection) bool {
	return b.Backend.Add(block)
}

func (b *BlockByCollections) Get(collID flow.Identifier) (*entity.BlocksByCollection, bool) {
	backdata := &BlockByCollectionBackdata{&b.Backdata}
	return backdata.ByID(collID)
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

func (b *BlockByCollectionBackdata) ByID(id flow.Identifier) (*entity.BlocksByCollection, bool) {
	e, exists := b.Backdata.ByID(id)
	if !exists {
		return nil, false
	}
	block := e.(*entity.BlocksByCollection)
	return block, true
}
