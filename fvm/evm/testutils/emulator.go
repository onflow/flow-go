package testutils

import (
	cryptoRand "crypto/rand"
	"math/big"
	"math/rand"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/types"
)

type TestEmulator struct {
	BalanceOfFunc      func(address types.Address) (*big.Int, error)
	NonceOfFunc        func(address types.Address) (uint64, error)
	CodeOfFunc         func(address types.Address) (types.Code, error)
	MintToFunc         func(address types.Address, amount *big.Int) (*types.Result, error)
	WithdrawFromFunc   func(address types.Address, amount *big.Int) (*types.Result, error)
	TransferFunc       func(from types.Address, to types.Address, value *big.Int) (*types.Result, error)
	DeployFunc         func(caller types.Address, code types.Code, gasLimit uint64, value *big.Int) (*types.Result, error)
	CallFunc           func(caller types.Address, to types.Address, data types.Data, gasLimit uint64, value *big.Int) (*types.Result, error)
	RunTransactionFunc func(tx *gethTypes.Transaction) (*types.Result, error)
}

var _ types.Emulator = &TestEmulator{}

// NewBlock returns a new block
func (em *TestEmulator) NewBlockView(_ types.BlockContext) (types.BlockView, error) {
	return em, nil
}

// NewBlock returns a new block view
func (em *TestEmulator) NewReadOnlyBlockView(_ types.BlockContext) (types.ReadOnlyBlockView, error) {
	return em, nil
}

// BalanceOf returns the balance of this address
func (em *TestEmulator) BalanceOf(address types.Address) (*big.Int, error) {
	if em.BalanceOfFunc == nil {
		panic("method not set")
	}
	return em.BalanceOfFunc(address)
}

// NonceOfFunc returns the nonce for this address
func (em *TestEmulator) NonceOf(address types.Address) (uint64, error) {
	if em.NonceOfFunc == nil {
		panic("method not set")
	}
	return em.NonceOfFunc(address)
}

// CodeOf returns the code for this address (if smart contract is deployed at this address)
func (em *TestEmulator) CodeOf(address types.Address) (types.Code, error) {
	if em.CodeOfFunc == nil {
		panic("method not set")
	}
	return em.CodeOfFunc(address)
}

// MintTo mints new tokens to this address
func (em *TestEmulator) MintTo(address types.Address, amount *big.Int) (*types.Result, error) {
	if em.MintToFunc == nil {
		panic("method not set")
	}
	return em.MintToFunc(address, amount)
}

// WithdrawFrom withdraws tokens from this address
func (em *TestEmulator) WithdrawFrom(address types.Address, amount *big.Int) (*types.Result, error) {
	if em.WithdrawFromFunc == nil {
		panic("method not set")
	}
	return em.WithdrawFromFunc(address, amount)
}

// Transfer transfers token between addresses
func (em *TestEmulator) Transfer(from types.Address, to types.Address, value *big.Int) (*types.Result, error) {
	if em.TransferFunc == nil {
		panic("method not set")
	}
	return em.TransferFunc(from, to, value)
}

// Deploy deploys an smart contract
func (em *TestEmulator) Deploy(caller types.Address, code types.Code, gasLimit uint64, value *big.Int) (*types.Result, error) {
	if em.DeployFunc == nil {
		panic("method not set")
	}
	return em.DeployFunc(caller, code, gasLimit, value)
}

// Call makes a call to a smart contract
func (em *TestEmulator) Call(caller types.Address, to types.Address, data types.Data, gasLimit uint64, value *big.Int) (*types.Result, error) {
	if em.CallFunc == nil {
		panic("method not set")
	}
	return em.CallFunc(caller, to, data, gasLimit, value)
}

// RunTransaction runs a transaction and collect gas fees to the coinbase account
func (em *TestEmulator) RunTransaction(tx *gethTypes.Transaction) (*types.Result, error) {
	if em.RunTransactionFunc == nil {
		panic("method not set")
	}
	return em.RunTransactionFunc(tx)
}

func RandomCommonHash(t testing.TB) gethCommon.Hash {
	ret := gethCommon.Hash{}
	_, err := cryptoRand.Read(ret[:gethCommon.HashLength])
	require.NoError(t, err)
	return ret
}

func RandomAddress(t testing.TB) types.Address {
	return types.NewAddress(RandomCommonAddress(t))
}

func RandomCommonAddress(t testing.TB) gethCommon.Address {
	ret := gethCommon.Address{}
	_, err := cryptoRand.Read(ret[:gethCommon.AddressLength])
	require.NoError(t, err)
	return ret
}

func RandomGas(limit int64) uint64 {
	return uint64(rand.Int63n(limit) + 1)
}

func RandomData(t testing.TB) []byte {
	// byte size [1, 100]
	size := rand.Intn(100) + 1
	ret := make([]byte, size)
	_, err := cryptoRand.Read(ret[:])
	require.NoError(t, err)
	return ret
}

func GetRandomLogFixture(t testing.TB) *gethTypes.Log {
	return &gethTypes.Log{
		Address: RandomCommonAddress(t),
		Topics: []gethCommon.Hash{
			RandomCommonHash(t),
			RandomCommonHash(t),
		},
		Data: RandomData(t),
	}
}
