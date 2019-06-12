package execution

import (
	"github.com/dapperlabs/bamboo-emulator/data"
)

// Node simulates the behaviour of a Bamboo execution node.
type Node interface {
	ExecuteBlock(block *data.Block)
}

type node struct {
	state    data.WorldState
	computer Computer
}

// NewNode returns a new simulated execution node.
func NewNode(state data.WorldState) Node {
	computer := NewComputer(state.GetRegister)

	return &node{
		state:    state,
		computer: computer,
	}
}

func (n *node) ExecuteBlock(block *data.Block) {
	registers, results := n.computer.ExecuteBlock(block)

	n.state.CommitRegisters(registers)

	n.state.SealBlock(block.Hash())

	for hash, succeeded := range results {
		if succeeded {
			n.state.UpdateTransactionStatus(hash, data.TxSucceeded)
		} else {
			n.state.UpdateTransactionStatus(hash, data.TxSealed)
		}
	}
}
