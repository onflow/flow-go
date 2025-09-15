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

var _ storage.ClusterBlocks = (*ClusterBlocks)(nil)

func NewClusterBlocks(db storage.DB, chainID flow.ChainID, headers *Headers, payloads *ClusterPayloads) *ClusterBlocks {
	b := &ClusterBlocks{
		db:       db,
		chainID:  chainID,
		headers:  headers,
		payloads: payloads,
	}
	return b
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
	untrustedBlock := cluster.UntrustedBlock{
		HeaderBody: header.HeaderBody,
		Payload:    *payload,
	}
	var block *cluster.Block
	if header.ContainsParentQC() {
		block, err = cluster.NewBlock(untrustedBlock)
		if err != nil {
			return nil, fmt.Errorf("could not build cluster block: %w", err)
		}

	} else {
		block, err = cluster.NewRootBlock(untrustedBlock)
		if err != nil {
			return nil, fmt.Errorf("could not build cluster root block: %w", err)
		}
	}

	untrustedProposal := cluster.UntrustedProposal{
		Block:           *block,
		ProposerSigData: sig,
	}
	var proposal *cluster.Proposal
	if header.ContainsParentQC() {
		proposal, err = cluster.NewProposal(untrustedProposal)
		if err != nil {
			return nil, fmt.Errorf("could not build cluster proposal: %w", err)
		}
	} else {
		proposal, err = cluster.NewRootProposal(untrustedProposal)
		if err != nil {
			return nil, fmt.Errorf("could not build root cluster proposal: %w", err)
		}
	}

	return proposal, nil
}

func (b *ClusterBlocks) ProposalByHeight(height uint64) (*cluster.Proposal, error) {
	var blockID flow.Identifier
	err := operation.LookupClusterBlockHeight(b.db.Reader(), b.chainID, height, &blockID)
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ProposalByID(blockID)
}
