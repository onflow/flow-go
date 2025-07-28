package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ClusterBlocks implements a simple block storage around a badger DB.
type ClusterBlocks struct {
	db       *badger.DB
	chainID  flow.ChainID
	headers  *Headers
	payloads *ClusterPayloads
}

var _ storage.ClusterBlocks = (*ClusterBlocks)(nil)

func NewClusterBlocks(db *badger.DB, chainID flow.ChainID, headers *Headers, payloads *ClusterPayloads) *ClusterBlocks {
	b := &ClusterBlocks{
		db:       db,
		chainID:  chainID,
		headers:  headers,
		payloads: payloads,
	}
	return b
}

func (b *ClusterBlocks) Store(proposal *cluster.Proposal) error {
	return operation.RetryOnConflictTx(b.db, transaction.Update, b.storeTx(proposal))
}

func (b *ClusterBlocks) storeTx(proposal *cluster.Proposal) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		blockID := proposal.Block.ID()
		err := b.headers.storeTx(blockID, proposal.Block.ToHeader(), proposal.ProposerSigData)(tx)
		if err != nil {
			return fmt.Errorf("could not store header: %w", err)
		}
		err = b.payloads.storeTx(blockID, &proposal.Block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not store payload: %w", err)
		}
		return nil
	}
}

func (b *ClusterBlocks) ProposalByID(blockID flow.Identifier) (*cluster.Proposal, error) {
	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get header: %w", err)
	}
	payload, err := b.payloads.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve payload: %w", err)
	}
	sig, err := b.headers.sigs.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve proposer signature: %w", err)
	}
	block, err := cluster.NewBlock(
		cluster.UnsignedUntrustedBlock{
			HeaderBody: header.HeaderBody,
			Payload:    *payload,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not build cluster block: %w", err)
	}
	proposal := &cluster.Proposal{
		Block:           *block,
		ProposerSigData: sig,
	}
	return proposal, nil
}

func (b *ClusterBlocks) ProposalByHeight(height uint64) (*cluster.Proposal, error) {
	var blockID flow.Identifier
	err := b.db.View(operation.LookupClusterBlockHeight(b.chainID, height, &blockID))
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ProposalByID(blockID)
}
