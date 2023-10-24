package testutils

import (
	cryptoRand "crypto/rand"
	"math/big"
	"math/rand"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/onflow/flow-go/fvm/evm/types"
)

type TestEmulator struct {
	BalanceOfFunc func(address types.Address) (*big.Int, error)
	CodeOfFunc    func(address types.Address) (types.Code, error)

	DirectCallFunc     func(call *types.DirectCall) (*types.Result, error)
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

// CodeOf returns the code for this address (if smart contract is deployed at this address)
func (em *TestEmulator) CodeOf(address types.Address) (types.Code, error) {
	if em.CodeOfFunc == nil {
		panic("method not set")
	}
	return em.CodeOfFunc(address)
}

// DirectCall executes a direct call
func (em *TestEmulator) DirectCall(call *types.DirectCall) (*types.Result, error) {
	if em.DirectCallFunc == nil {
		panic("method not set")
	}
	return em.DirectCallFunc(call)
}

// RunTransaction runs a transaction and collect gas fees to the coinbase account
func (em *TestEmulator) RunTransaction(tx *gethTypes.Transaction) (*types.Result, error) {
	if em.RunTransactionFunc == nil {
		panic("method not set")
	}
	return em.RunTransactionFunc(tx)
}

func RandomCommonHash() gethCommon.Hash {
	ret := gethCommon.Hash{}
	cryptoRand.Read(ret[:gethCommon.HashLength])
	return ret
}

func RandomAddress() types.Address {
	return types.NewAddress(RandomCommonAddress())
}

func RandomCommonAddress() gethCommon.Address {
	ret := gethCommon.Address{}
	cryptoRand.Read(ret[:gethCommon.AddressLength])
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

func GetRandomLogFixture() *gethTypes.Log {
	return &gethTypes.Log{
		Address: RandomCommonAddress(),
		Topics: []gethCommon.Hash{
			RandomCommonHash(),
			RandomCommonHash(),
		},
		Data: RandomData(),
	}
}
