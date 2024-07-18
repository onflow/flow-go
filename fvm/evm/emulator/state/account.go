package state

import (
	"math/big"

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

func (a *Account) HasCode() bool {
	return a.CodeHash != gethTypes.EmptyCodeHash
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
	// try decoding
	err := rlp.DecodeBytes(inp, a)
	if err != nil {
		// try legacy decoding (fall back)
		a = decodeLegacy(inp)
		// if no success return the initial error
		if a == nil {
			return nil, err
		}
	}
	return a, nil

}

// TODO: remove it when the Preview is shut down
// Legacy account type - used big.Int for balances
type accountV0 struct {
	// address
	Address gethCommon.Address
	// balance of the address
	Balance *big.Int
	// nonce of the address
	Nonce uint64
	// hash of the code
	// if no code the gethTypes.EmptyCodeHash is stored
	CodeHash gethCommon.Hash
	// the id of the collection holds storage slots for this account
	// this value is nil for EOA accounts
	CollectionID []byte
}

func decodeLegacy(encoded []byte) *Account {
	a0 := &accountV0{}
	if err := rlp.DecodeBytes(encoded, a0); err == nil {
		return &Account{
			Address:      a0.Address,
			Balance:      uint256.MustFromBig(a0.Balance),
			Nonce:        a0.Nonce,
			CodeHash:     a0.CodeHash,
			CollectionID: a0.CollectionID,
		}
	}
	return nil
}
