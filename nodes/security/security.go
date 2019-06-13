package security

import (
	"time"

	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
)

// Node simulates the behaviour of a Bamboo access node.
type Node struct {
	state *data.WorldState
}

// NewNode returns a new simulated security node.
func NewNode(state *data.WorldState) *Node {
	return &Node{
		state: state,
	}
}

func (n *Node) MintNoOpBlock() *data.Block {
	latestBlock := n.state.GetLatestBlock()

	newBlock := data.Block{
		Number:            latestBlock.Number,
		Timestamp:         time.Now(),
		PrevBlockHash:     latestBlock.Hash(),
		Status:            data.BlockPending,
		CollectionHashes:  []crypto.Hash{},
		TransactionHashes: []crypto.Hash{},
	}

	return &newBlock
}

func (n *Node) SealBlock(hash crypto.Hash) error {
	return n.state.SealBlock(hash)
}
