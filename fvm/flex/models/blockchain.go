package models

import "github.com/ethereum/go-ethereum/common"

type BlockChain interface {
	// Appends a block to the block chain
	AppendBlock(block *FlexBlock) error

	// LatestBlock returns the latest appended block
	LatestBlock() (*FlexBlock, error)

	// returns the hash of the block at the given height
	BlockHash(height int) (common.Hash, error)
}
