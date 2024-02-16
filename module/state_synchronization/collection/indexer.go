package collection

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

type Indexer struct {
	collections storage.Collections
	handler     func(*flow.Collection)
}

func NewIndexer(
	collections storage.Collections,
) *Indexer {
	return &Indexer{
		collections: collections,
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

		i.handler(chunk.Collection)
	}

	return nil
}

func (i *Indexer) WithHandler(handler func(*flow.Collection)) {
	i.handler = handler
}
