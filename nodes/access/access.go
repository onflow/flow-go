package access

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/nodes/access/collection_builder"
)

// Node simulates the behaviour of a Bamboo access node.
type Node struct {
	conf              *Config
	state             *data.WorldState
	transactionsIn    chan *data.Transaction
	collectionBuilder *collection_builder.CollectionBuilder
	log               *logrus.Logger
}

// Config hold the configuration options for an access node.
type Config struct {
	CollectionInterval time.Duration
}

// NewNode returns a new simulated access node.
func NewNode(conf *Config, state *data.WorldState, collectionsOut chan *data.Collection, log *logrus.Logger) *Node {
	transactionsIn := make(chan *data.Transaction, 16)

	collectionBuilder := collection_builder.NewCollectionBuilder(state, transactionsIn, collectionsOut, log)

	return &Node{
		conf:              conf,
		state:             state,
		transactionsIn:    transactionsIn,
		collectionBuilder: collectionBuilder,
		log:               log,
	}
}

func (n *Node) Start(ctx context.Context) {
	n.collectionBuilder.Start(ctx, n.conf.CollectionInterval)
}

func (n *Node) SendTransaction(tx *data.Transaction) error {
	txEntry := n.log.WithField("transactionHash", tx.Hash())
	txEntry.Info("Transaction submitted to network")

	err := n.state.InsertTransaction(tx)
	if err != nil {
		txEntry.Error("Transaction has already been submitted")
		return &DuplicateTransactionError{txHash: tx.Hash()}
	}

	for {
		select {
		case n.transactionsIn <- tx:
			txEntry.Debug("Transaction added to pending queue")
			return nil
		default:
			txEntry.Error("Transaction queue is full")
			return &TransactionQueueFullError{txHash: tx.Hash()}
		}
	}
}

func (n *Node) GetBlockByHash(hash crypto.Hash) (*data.Block, error) {
	blockEntry := n.log.WithField("blockHash", hash)
	blockEntry.Info("Fetching block by hash")

	block, err := n.state.GetBlockByHash(hash)
	if err != nil {
		switch err.(type) {
		case *data.ItemNotFoundError:
			blockEntry.Error("Block not found")
			return nil, &BlockNotFoundByHashError{blockHash: hash}
		default:
			return nil, err
		}
	}

	return block, nil
}

func (n *Node) GetBlockByNumber(number uint64) (*data.Block, error) {
	blockEntry := n.log.WithField("blockNumber", number)
	blockEntry.Info("Fetching block by number")

	block, err := n.state.GetBlockByNumber(number)
	if err != nil {
		switch err.(type) {
		case *data.InvalidBlockNumberError:
			blockEntry.Error("Block not found")
			return nil, &BlockNotFoundByNumberError{blockNumber: number}
		default:
			return nil, err
		}
	}

	return block, nil
}

func (n *Node) GetLatestBlock() *data.Block {
	n.log.Info("Fetching latest block")
	return n.state.GetLatestBlock()
}

func (n *Node) GetTransaction(hash crypto.Hash) (*data.Transaction, error) {
	txEntry := n.log.WithField("transactionHash", hash)
	txEntry.Info("Fetching transaction by hash")

	tx, err := n.state.GetTransaction(hash)
	if err != nil {
		switch err.(type) {
		case *data.ItemNotFoundError:
			txEntry.Error("Transaction not found")
			return nil, &TransactionNotFoundError{txHash: hash}
		default:
			return nil, err
		}
	}

	return tx, nil
}

func (n *Node) GetAccount(address crypto.Address) (*crypto.Account, error) {
	// TODO: implement GetAccount
	return nil, nil
}

func (n *Node) CallContract(script []byte) (interface{}, error) {
	// TODO: implement CallContract
	return nil, nil
}
