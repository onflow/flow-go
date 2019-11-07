package store

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Store interface {
	GetBlockByHash(crypto.Hash) (flow.Block, error)
	GetBlockByNumber(blockNumber uint64) (flow.Block, error)
	GetLatestBlock() (flow.Block, error)

	InsertBlock(flow.Block) error

	GetTransaction(crypto.Hash) (flow.Transaction, error)
	InsertTransaction(flow.Transaction) error

	GetRegistersView(blockNumber uint64) flow.RegistersView
	SetRegisters(blockNumber uint64, registers flow.Registers) error
}
