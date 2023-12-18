package state

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// Account holds the metadata of an address and provides (de)serialization
//
// note that code and slots of an address is not part of this data structure
type Account struct {
	// address
	Address gethCommon.Address
	// balance of the account
	Balance *big.Int
	// nonce
	Nonce uint64
	// hash of the code
	CodeHash gethCommon.Hash
	// CollectionID of the id of the collection holds slots of this account
	CollectionID []byte
}

// NewAccount constructs a new account
func NewAccount(
	address gethCommon.Address,
	balance *big.Int,
	nonce uint64,
	codeHash gethCommon.Hash,
	collectionID []byte,
) *Account {
	return &Account{
		Address:      address,
		Balance:      balance,
		Nonce:        nonce,
		CodeHash:     codeHash,
		CollectionID: collectionID,
	}
}

// Encode encodes the account
func (a *Account) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(a)
}

// DecodeAccount constructs a new account from encoded data
func DecodeAccount(inp []byte) (*Account, error) {
	if len(inp) == 0 {
		return nil, nil
	}
	a := &Account{}
	return a, rlp.DecodeBytes(inp, a)
}
