package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// ClusterBlocks implements a simple block storage around a badger DB.
type ClusterBlocks struct {
	db       *badger.DB
	chainID  flow.ChainID
	headers  *Headers
	payloads *ClusterPayloads
}

func NewClusterBlocks(db *badger.DB, chainID flow.ChainID, headers *Headers, payloads *ClusterPayloads) *ClusterBlocks {
	b := &ClusterBlocks{
		db:       db,
		chainID:  chainID,
		headers:  headers,
		payloads: payloads,
	}
	return b
}

func (b *ClusterBlocks) Store(block *cluster.Block) error {
	err := b.headers.Store(block.Header)
	if err != nil {
		return fmt.Errorf("could not store header: %w", err)
	}
	err = b.payloads.Store(block.ID(), block.Payload)
	if err != nil {
		return fmt.Errorf("could not store payload: %w", err)
	}
	return nil
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
	err := b.db.View(operation.LookupClusterBlockHeight(b.chainID, height, &blockID))
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}
