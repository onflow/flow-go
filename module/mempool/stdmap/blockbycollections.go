package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/entity"
	_ "github.com/onflow/flow-go/utils/binstat"
)

// Hold all the missing collections.
// Each entry is a missing collection, and all the blocks that contain
// this collection
type BlockByCollections struct {
	*Backend[flow.Identifier, *entity.BlocksByCollection]
}

// BlockByCollectionBackdata contains all the collections is being requested,
// for each collection it stores the blocks that contains the collection.
// the Backdata is essentially map<collectionID>map<blockID>*ExecutableBlock
type BlockByCollectionBackdata struct {
	mempool.BackData[flow.Identifier, *entity.BlocksByCollection]
}

func NewBlockByCollections() *BlockByCollections {
	return &BlockByCollections{NewBackend[flow.Identifier, *entity.BlocksByCollection](
		WithEject[flow.Identifier, *entity.BlocksByCollection](EjectPanic[flow.Identifier, *entity.BlocksByCollection]),
	)}
}

func (b *BlockByCollections) Add(block *entity.BlocksByCollection) bool {
	return b.Backend.Add(block.CollectionID, block)
}

func (b *BlockByCollections) Get(collID flow.Identifier) (*entity.BlocksByCollection, bool) {
	backdata := &BlockByCollectionBackdata{b.mutableBackData}
	return backdata.ByID(collID)
}

func (b *BlockByCollections) Run(f func(backdata *BlockByCollectionBackdata) error) error {
	//bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(BlockByCollections).Run")
	b.Lock()
	//binstat.Leave(bs1)

	//bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(BlockByCollections).Run)")
	defer b.Unlock()
	err := f(&BlockByCollectionBackdata{b.mutableBackData})
	//binstat.Leave(bs2)

	if err != nil {
		return err
	}
	return nil
}

func (b *BlockByCollectionBackdata) ByID(id flow.Identifier) (*entity.BlocksByCollection, bool) {
	block, exists := b.BackData.ByID(id)
	if !exists {
		return nil, false
	}
	return block, true
}
