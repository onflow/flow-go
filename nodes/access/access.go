package access

import (
	"context"
	"math/big"

	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
)

// Node simulates the behaviour of a Bamboo access node.
type Node struct {
	state             *data.WorldState
	transactionsIn    chan *data.Transaction
	collectionBuilder *CollectionBuilder
}

// NewNode returns a new simulated access node.
func NewNode(state *data.WorldState, collectionsOut chan *data.Collection) *Node {
	transactionsIn := make(chan *data.Transaction, 16)

	collectionBuilder := NewCollectionBuilder(state, transactionsIn, collectionsOut)

	return &Node{
		state:             state,
		transactionsIn:    transactionsIn,
		collectionBuilder: collectionBuilder,
	}
}

func (n *Node) Start(ctx context.Context) {
	n.collectionBuilder.Start(ctx)
}

func (n *Node) SendTransaction(tx *data.Transaction) error {
	err := n.state.InsertTransaction(tx)
	if err != nil {
		return &DuplicateTransactionError{txHash: tx.Hash()}
	}

	n.transactionsIn <- tx

	return nil
}

func (n *Node) GetBlockByHash(hash crypto.Hash) (*data.Block, error) {
	block, err := n.state.GetBlockByHash(hash)
	if err != nil {
		switch err.(type) {
		case *data.ItemNotFoundError:
			return nil, &BlockNotFoundByHashError{blockHash: hash}
		default:
			return nil, err
		}
	}

	return block, nil
}

func (n *Node) GetBlockByNumber(number uint64) (*data.Block, error) {
	block, err := n.state.GetBlockByNumber(number)
	if err != nil {
		switch err.(type) {
		case *data.InvalidBlockNumberError:
			return nil, &BlockNotFoundByNumberError{blockNumber: number}
		default:
			return nil, err
		}
	}

	return block, nil
}

func (n *Node) GetLatestBlock() *data.Block {
	return n.state.GetLatestBlock()
}

func (n *Node) GetTransaction(hash crypto.Hash) (*data.Transaction, error) {
	tx, err := n.state.GetTransaction(hash)
	if err != nil {
		switch err.(type) {
		case *data.ItemNotFoundError:
			return nil, &TransactionNotFoundError{txHash: hash}
		default:
			return nil, err
		}
	}

	return tx, nil
}

func (n *Node) GetBalance(address crypto.Address) (*big.Int, error) {
	// TODO: implement GetBalance
	return nil, nil
}

func (n *Node) CallContract(script []byte) (interface{}, error) {
	// TODO: implement CallContract
	return nil, nil
}
