package state

import (
	"math/big"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
)

// Registers is a map of register values.
type Registers map[crypto.Hash][]byte

func (r Registers) MergeWith(registers Registers) {
	for key, value := range registers {
		r[key] = value
	}
}

func (r Registers) NewView() *RegistersView {
	return &RegistersView{
		new: make(Registers),
		old: r,
	}
}

// RegistersView provides a read-only view into an existing registers state.
//
// Values are written to a temporary register cache that can later be
// committed to the world state.
type RegistersView struct {
	new Registers
	old Registers
}

func (r *RegistersView) UpdatedRegisters() Registers {
	return r.new
}

func (r *RegistersView) Set(controller, owner, key, value []byte) {
	r.set(getFullKey(controller, owner, key), value)
}

func (r *RegistersView) Get(controller, owner, key []byte) (value []byte, exists bool) {
	return r.get(getFullKey(controller, owner, key))
}

func (r *RegistersView) CreateAccount(publicKey, code []byte) crypto.Address {
	latestAccountID, _ := r.get(keyLatestAccount())

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := crypto.BytesToAddress(accountIDBytes)

	accountID := accountAddress.Bytes()

	r.Set(accountID, accountID, []byte("balance"), big.NewInt(0).Bytes())
	r.Set(accountID, accountID, []byte("public_key"), publicKey)
	r.Set(accountID, accountID, []byte("code"), code)

	r.set(keyLatestAccount(), accountID)

	address := crypto.BytesToAddress(accountID)

	return address
}

func (r *RegistersView) GetAccount(address crypto.Address) *crypto.Account {
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

func (r *RegistersView) get(key crypto.Hash) (value []byte, exists bool) {
	value, exists = r.new[key]
	if exists {
		return value, exists
	}

	value, exists = r.old[key]
	return value, exists
}

func (r *RegistersView) set(key crypto.Hash, value []byte) {
	r.new[key] = value
}

func keyLatestAccount() crypto.Hash {
	return crypto.NewHash([]byte("latestAccount"))
}

func getFullKey(controller, owner, key []byte) crypto.Hash {
	fullKey := append(controller, owner...)
	fullKey = append(fullKey, key...)

	return crypto.NewHash(fullKey)
}
