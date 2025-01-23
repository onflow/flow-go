package state

import (
	"github.com/holiman/uint256"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	"github.com/onflow/go-ethereum/rlp"
)

// Account holds the metadata of an address and provides (de)serialization functionality
//
// Note that code and storage slots of an address is not part of this data structure
type Account struct {
	// address
	Address gethCommon.Address
	// balance of the address
	Balance *uint256.Int
	// nonce of the address
	Nonce uint64
	// hash of the code
	// if no code the gethTypes.EmptyCodeHash is stored
	CodeHash gethCommon.Hash
	// the id of the collection holds storage slots for this account
	// this value is nil for EOA accounts
	CollectionID []byte
}

// NewAccount constructs a new account
func NewAccount(
	address gethCommon.Address,
	balance *uint256.Int,
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

// HasCode returns true if account has code
func (a *Account) HasCode() bool {
	return a.CodeHash != gethTypes.EmptyCodeHash
}

// HasStoredValues returns true if account has stored values
func (a *Account) HasStoredValues() bool {
	return len(a.CollectionID) != 0
}

// Encode encodes the account
func (a *Account) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(a)
}

// DecodeAccount constructs a new account from the encoded data
func DecodeAccount(inp []byte) (*Account, error) {
	if len(inp) == 0 {
		return nil, nil
	}
	a := &Account{}
	return a, rlp.DecodeBytes(inp, a)
}
