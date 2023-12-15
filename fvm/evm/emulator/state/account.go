package state

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

type account struct {
	// address
	address gethCommon.Address
	// balance of the account
	balance *big.Int
	// nonce
	nonce uint64
	// hash of the code
	codeHash gethCommon.Hash
	// storageID of the map that holds account slots
	storageIDBytes []byte
}

func newAccount(
	address gethCommon.Address,
	balance *big.Int,
	nonce uint64,
	codeHash gethCommon.Hash,
	storageIDBytes []byte,
) *account {
	return &account{
		address:        address,
		balance:        balance,
		nonce:          nonce,
		codeHash:       codeHash,
		storageIDBytes: storageIDBytes,
	}
}

func (a *account) encode() ([]byte, error) {
	return rlp.EncodeToBytes(a)
}

func decodeAccount(inp []byte) (*account, error) {
	a := &account{}
	return a, rlp.DecodeBytes(inp, a)
}
