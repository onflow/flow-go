package testutils

import (
	cryptoRand "crypto/rand"
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/onflow/flow-go/fvm/flex/models"
)

type TestEmulator struct {
	BalanceOfFunc      func(address models.FlexAddress) (*big.Int, error)
	CodeOfFunc         func(address models.FlexAddress) (models.Code, error)
	MintToFunc         func(address models.FlexAddress, amount *big.Int) (*models.Result, error)
	WithdrawFromFunc   func(address models.FlexAddress, amount *big.Int) (*models.Result, error)
	TransferFunc       func(from models.FlexAddress, to models.FlexAddress, value *big.Int) (*models.Result, error)
	DeployFunc         func(caller models.FlexAddress, code models.Code, gasLimit uint64, value *big.Int) (*models.Result, error)
	CallFunc           func(caller models.FlexAddress, to models.FlexAddress, data models.Data, gasLimit uint64, value *big.Int) (*models.Result, error)
	RunTransactionFunc func(tx *types.Transaction, coinbase models.FlexAddress) (*models.Result, error)
}

var _ models.Emulator = &TestEmulator{}

// BalanceOf returns the balance of this address
func (em *TestEmulator) BalanceOf(address models.FlexAddress) (*big.Int, error) {
	if em.BalanceOfFunc == nil {
		panic("method not set")
	}
	return em.BalanceOfFunc(address)
}

// CodeOf returns the code for this address (if smart contract is deployed at this address)
func (em *TestEmulator) CodeOf(address models.FlexAddress) (models.Code, error) {
	if em.CodeOfFunc == nil {
		panic("method not set")
	}
	return em.CodeOfFunc(address)
}

// MintTo mints new tokens to this address
func (em *TestEmulator) MintTo(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
	if em.MintToFunc == nil {
		panic("method not set")
	}
	return em.MintToFunc(address, amount)
}

// WithdrawFrom withdraws tokens from this address
func (em *TestEmulator) WithdrawFrom(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
	if em.WithdrawFromFunc == nil {
		panic("method not set")
	}
	return em.WithdrawFromFunc(address, amount)
}

// Transfer transfers token between addresses
func (em *TestEmulator) Transfer(from models.FlexAddress, to models.FlexAddress, value *big.Int) (*models.Result, error) {
	if em.TransferFunc == nil {
		panic("method not set")
	}
	return em.TransferFunc(from, to, value)
}

// Deploy deploys an smart contract
func (em *TestEmulator) Deploy(caller models.FlexAddress, code models.Code, gasLimit uint64, value *big.Int) (*models.Result, error) {
	if em.DeployFunc == nil {
		panic("method not set")
	}
	return em.DeployFunc(caller, code, gasLimit, value)
}

// Call makes a call to a smart contract
func (em *TestEmulator) Call(caller models.FlexAddress, to models.FlexAddress, data models.Data, gasLimit uint64, value *big.Int) (*models.Result, error) {
	if em.CallFunc == nil {
		panic("method not set")
	}
	return em.CallFunc(caller, to, data, gasLimit, value)
}

// RunTransaction runs a transaction and collect gas fees to the coinbase account
func (em *TestEmulator) RunTransaction(tx *types.Transaction, coinbase models.FlexAddress) (*models.Result, error) {
	if em.RunTransactionFunc == nil {
		panic("method not set")
	}
	return em.RunTransactionFunc(tx, coinbase)
}

func RandomCommonHash() common.Hash {
	ret := common.Hash{}
	cryptoRand.Read(ret[:common.HashLength])
	return ret
}

func RandomFlexAddress() models.FlexAddress {
	return models.NewFlexAddress(RandomAddress())
}

func RandomAddress() common.Address {
	ret := common.Address{}
	cryptoRand.Read(ret[:common.AddressLength])
	return ret
}

func RandomGas(limit int64) uint64 {
	return uint64(rand.Int63n(limit) + 1)
}

func RandomData() []byte {
	// byte size [1, 100]
	size := rand.Intn(100) + 1
	ret := make([]byte, size)
	cryptoRand.Read(ret[:])
	return ret
}

func GetRandomLogFixture() *types.Log {
	return &types.Log{
		Address: RandomAddress(),
		Topics: []common.Hash{
			RandomCommonHash(),
			RandomCommonHash(),
		},
		Data: RandomData(),
	}
}
