package state

import (
	"math/big"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
)

const (
	keyLatestAccount = "latestAccount"
)

// Registers is a map of register values.
type Registers map[crypto.Hash][]byte

func fullKey(controller, owner, key []byte) crypto.Hash {
	fullKey := append(controller, owner...)
	fullKey = append(fullKey, key...)

	return crypto.NewHash(fullKey)
}

func (r Registers) Set(controller, owner, key, value []byte) {
	r[fullKey(controller, owner, key)] = value
}

func (r Registers) Get(controller, owner, key []byte) (value []byte, exists bool) {
	value, exists = r[fullKey(controller, owner, key)]
	return value, exists
}

func (r Registers) CreateAccount(publicKey, code []byte) crypto.Address {
	latestAccountID := r[r.keyLatestAccount()]

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := crypto.BytesToAddress(accountIDBytes)

	accountID := accountAddress.Bytes()

	r.Set(accountID, accountID, []byte("balance"), big.NewInt(0).Bytes())
	r.Set(accountID, accountID, []byte("public_key"), publicKey)
	r.Set(accountID, accountID, []byte("code"), code)

	r[r.keyLatestAccount()] = accountID

	address := crypto.BytesToAddress(accountID)

	return address
}

func (r Registers) GetAccount(address crypto.Address) *crypto.Account {
	accountID := address.Bytes()

	balanceBytes, exists := r.Get(accountID, accountID, []byte("balance"))
	if !exists {
		return nil
	}

	balanceInt := big.NewInt(0).SetBytes(balanceBytes)

	publicKey, _ := r.Get(accountID, accountID, []byte("public_key"))
	code, _ := r.Get(accountID, accountID, []byte("code"))

	return &crypto.Account{
		Address:    address,
		Balance:    balanceInt.Uint64(),
		Code:       code,
		PublicKeys: [][]byte{publicKey},
	}
}

func (r Registers) MergeWith(registers Registers) {
	for key, value := range registers {
		r[key] = value
	}
}

func (r Registers) keyLatestAccount() crypto.Hash {
	return crypto.NewHash([]byte(keyLatestAccount))
}
