package state

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

type Account struct {
	// address
	Address gethCommon.Address
	// balance of the account
	Balance *big.Int
	// nonce
	Nonce uint64
	// hash of the code
	CodeHash gethCommon.Hash
	// storageID of the map that holds account slots
	StorageIDBytes []byte
}

func NewAccount(
	address gethCommon.Address,
	balance *big.Int,
	nonce uint64,
	codeHash gethCommon.Hash,
	storageIDBytes []byte,
) *Account {
	return &Account{
		Address:        address,
		Balance:        balance,
		Nonce:          nonce,
		CodeHash:       codeHash,
		StorageIDBytes: storageIDBytes,
	}
}

func (a *Account) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(a)
}

func DecodeAccount(inp []byte) (*Account, error) {
	a := &Account{}
	return a, rlp.DecodeBytes(inp, a)
}
