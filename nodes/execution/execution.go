package execution

import (
	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/runtime"
)

// Node simulates the behaviour of a Bamboo execution node.
type Node struct {
	state    *data.WorldState
	computer *Computer
	log      *logrus.Logger
}

// NewNode returns a new simulated execution node.
func NewNode(state *data.WorldState, log *logrus.Logger) *Node {
	runtime := runtime.NewMockRuntime()
	computer := NewComputer(runtime, state.GetTransaction, state.GetRegister)

	return &Node{
		state:    state,
		computer: computer,
		log:      log,
	}
}

// ExecuteBlock executes a block and saves the results to the world state.
func (n *Node) ExecuteBlock(block *data.Block) error {
	blockEntry := n.log.WithFields(logrus.Fields{
		"blockHash", block.Hash(),
		"blockNumber", block.Number,
	})

	blockEntry.Info("Executing block at height %d", block.Number)

	registers, results, err := n.computer.ExecuteBlock(block)
	if err != nil {
		blockEntry.
			WithError(err).
			Error("Failed to execute block at height %d", block.Number)
		return err
	}

	n.state.CommitRegisters(registers)

	err = n.state.SealBlock(block.Hash())
	if err != nil {
		return err
	}

	for hash, succeeded := range results {
		if succeeded {
			err = n.state.UpdateTransactionStatus(hash, data.TxSealed)
			if err != nil {
				return err
			}
		} else {
			err = n.state.UpdateTransactionStatus(hash, data.TxReverted)
			if err != nil {
				return err
			}
		}
	}

	blockEntry.Info("Sealed block at height %d", block.Number)

	return nil
}
