package collection

import (
	"fmt"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/storage"
)

type Indexer struct {
	collections storage.Collections
	handler     state_synchronization.CollectionHandler
}

var _ state_synchronization.ExecutionDataIndexer = (*Indexer)(nil)

func NewIndexer(
	collections storage.Collections,
	handler state_synchronization.CollectionHandler,
) *Indexer {
	return &Indexer{
		collections: collections,
		handler:     handler,
	}
}

// IndexBlockData indexes the collections in the given block data, and
// notify the handler for each collection.
func (i *Indexer) IndexBlockData(data *execution_data.BlockExecutionDataEntity) error {
	for _, chunk := range data.BlockExecutionData.ChunkExecutionDatas {
		err := i.collections.Store(chunk.Collection)
		if err != nil {
			return fmt.Errorf("failed to store collection for block %v, coll ID %v: %w",
				data.ID(),
				chunk.Collection.ID(),
				err)
		}

		err = i.handler.HandleCollection(chunk.Collection)
		if err != nil {
			return fmt.Errorf("failed to handle collection for block %v, coll ID %v: %w",
				data.ID(),
				chunk.Collection.ID(),
				err)
		}
	}

	return nil
}
