package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// ClusterBlocks implements a simple block storage around a badger DB.
type ClusterBlocks struct {
	db       storage.DB
	chainID  flow.ChainID
	headers  *Headers
	payloads *ClusterPayloads
}

func NewClusterBlocks(db storage.DB, chainID flow.ChainID, headers *Headers, payloads *ClusterPayloads) *ClusterBlocks {
	b := &ClusterBlocks{
		db:       db,
		chainID:  chainID,
		headers:  headers,
		payloads: payloads,
	}
	return b
}

func (b *ClusterBlocks) ByID(blockID flow.Identifier) (*cluster.Block, error) {
	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get header: %w", err)
	}
	payload, err := b.payloads.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve payload: %w", err)
	}
	block := cluster.Block{
		Header:  header,
		Payload: payload,
	}
	return &block, nil
}

func (b *ClusterBlocks) ByHeight(height uint64) (*cluster.Block, error) {
	var blockID flow.Identifier
	err := operation.LookupClusterBlockHeight(b.db.Reader(), b.chainID, height, &blockID)
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}
