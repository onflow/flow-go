package models

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Result struct {
	RootHash                common.Hash
	DeployedContractAddress FlexAddress
	ReturnedValue           []byte
	GasConsumed             uint64
	Logs                    []*types.Log
}

type Emulator interface {
	BalanceOf(address FlexAddress) (*big.Int, error)
	MintTo(address FlexAddress, amount *big.Int) (*Result, error)
	WithdrawFrom(address FlexAddress, amount *big.Int) (*Result, error)
	Transfer(from FlexAddress, to FlexAddress, value *big.Int) (*Result, error)
	Deploy(caller FlexAddress, code Code, gasLimit uint64, value *big.Int) (*Result, error)
	Call(caller FlexAddress, to FlexAddress, data Data, gasLimit uint64, value *big.Int) (*Result, error)
	RunTransaction(tx *types.Transaction) (*Result, error)
}
