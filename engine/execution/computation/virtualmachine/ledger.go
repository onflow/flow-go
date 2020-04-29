package virtualmachine

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/encoding/rlp"
	"github.com/dapperlabs/flow-go/model/flow"
)

// A Ledger is the storage interface used by the virtual machine to read and write register values.
type Ledger interface {
	Set(key flow.RegisterID, value flow.RegisterValue)
	Get(key flow.RegisterID) (flow.RegisterValue, error)
	Delete(key flow.RegisterID)
}

// A MapLedger is a naive ledger storage implementation backed by a simple map.
//
// This implementation is designed for testing purposes.
type MapLedger map[string]flow.RegisterValue

func (m MapLedger) Set(key flow.RegisterID, value flow.RegisterValue) {
	m[string(key)] = value
}

func (m MapLedger) Get(key flow.RegisterID) (flow.RegisterValue, error) {
	return m[string(key)], nil
}

func (m MapLedger) Delete(key flow.RegisterID) {
	delete(m, string(key))
}

const (
	keyLatestAccount  = "latest_account"
	keyExists         = "exists"
	keyBalance        = "balance"
	keyCode           = "code"
	keyPublicKeyCount = "public_key_count"
)

func fullKey(owner, controller, key string) string {
	return strings.Join([]string{owner, controller, key}, "__")
}
func fullKeyHash(owner, controller, key string) flow.RegisterID {
	h := sha256.New()
	_, _ = h.Write([]byte(fullKey(owner, controller, key)))
	return h.Sum(nil)
}

func keyPublicKey(index int) string {
	return fmt.Sprintf("public_key_%d", index)
}

// Set of functions to read/write Ledger
type LedgerAccess struct {
	Ledger Ledger
}

func (r *LedgerAccess) CheckAccountExists(accountID []byte) error {
	exists, err := r.Ledger.Get(fullKeyHash(string(accountID), "", keyExists))
	if err != nil {
		return err
	}

	bal, err := r.Ledger.Get(fullKeyHash(string(accountID), "", keyBalance))
	if err != nil {
		return err
	}

	if len(exists) != 0 || bal != nil {
		return nil
	}

	return fmt.Errorf("account with ID %x does not exist", accountID)
}

func (r *LedgerAccess) GetAccountPublicKeys(accountID []byte) (publicKeys [][]byte, err error) {
	countBytes, err := r.Ledger.Get(
		fullKeyHash(string(accountID), string(accountID), keyPublicKeyCount),
	)
	if err != nil {
		return nil, err
	}

	if countBytes == nil {
		return nil, fmt.Errorf("key count not set")
	}

	count := int(big.NewInt(0).SetBytes(countBytes).Int64())

	publicKeys = make([][]byte, count)

	for i := 0; i < count; i++ {
		publicKey, err := r.Ledger.Get(
			fullKeyHash(string(accountID), string(accountID), keyPublicKey(i)),
		)
		if err != nil {
			return nil, err
		}

		if publicKey == nil {
			return nil, fmt.Errorf("failed to retrieve key from account %s", accountID)
		}

		publicKeys[i] = publicKey
	}

	return publicKeys, nil
}

//
// The function returns nil if the specified account does not exist.
func (r *LedgerAccess) GetAccount(address flow.Address) *flow.Account {
	accountID := address.Bytes()

	err := r.CheckAccountExists(accountID)
	if err != nil {
		return nil
	}

	balanceBytes, _ := r.Ledger.Get(fullKeyHash(string(accountID), "", keyBalance))
	balanceInt := big.NewInt(0).SetBytes(balanceBytes)

	code, _ := r.Ledger.Get(fullKeyHash(string(accountID), string(accountID), keyCode))

	publicKeys, err := r.GetAccountPublicKeys(accountID)
	if err != nil {
		panic(err)
	}

	accountPublicKeys := make([]flow.AccountPublicKey, len(publicKeys))
	for i, publicKey := range publicKeys {
		accountPublicKey, err := decodePublicKey(publicKey)
		if err != nil {
			panic(err)
		}

		accountPublicKeys[i] = accountPublicKey
	}

	return &flow.Account{
		Address: address,
		Balance: balanceInt.Uint64(),
		Code:    code,
		Keys:    accountPublicKeys,
	}
}

// TODO: replace once public key format changes @psiemens
func decodePublicKey(b []byte) (a flow.AccountPublicKey, err error) {
	var temp struct {
		PublicKey []byte
		SignAlgo  uint
		HashAlgo  uint
		Weight    uint
	}

	encoder := rlp.NewEncoder()

	err = encoder.Decode(b, &temp)
	if err != nil {
		return a, err
	}

	signAlgo := crypto.SigningAlgorithm(temp.SignAlgo)
	hashAlgo := hash.HashingAlgorithm(temp.HashAlgo)

	publicKey, err := crypto.DecodePublicKey(signAlgo, temp.PublicKey)
	if err != nil {
		return a, err
	}

	return flow.AccountPublicKey{
		PublicKey: publicKey,
		SignAlgo:  signAlgo,
		HashAlgo:  hashAlgo,
		Weight:    int(temp.Weight),
	}, nil
}

func (r *LedgerAccess) GetLatestAccount() flow.Address {
	latestAccountID, _ := r.Ledger.Get(fullKeyHash("", "", keyLatestAccount))

	return flow.BytesToAddress(latestAccountID)
}

func (r *LedgerAccess) CreateAccountInLedger(publicKeys [][]byte) (flow.Address, error) {
	accountAddress := r.GetLatestAccount()

	accountID := accountAddress[:]

	accountIDInt := big.NewInt(0).SetBytes(accountID)
	newAccountBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	newAccountAddress := flow.BytesToAddress(newAccountBytes)
	newAccountID := newAccountAddress[:]

	// mark that account with this ID exists
	r.Ledger.Set(fullKeyHash(string(newAccountID), "", keyExists), []byte{1})

	// set account balance to 0
	r.Ledger.Set(fullKeyHash(string(newAccountID), "", keyBalance), big.NewInt(0).Bytes())

	err := r.SetAccountPublicKeys(newAccountID, publicKeys)
	if err != nil {
		return flow.Address{}, err
	}

	r.Ledger.Set(fullKeyHash("", "", keyLatestAccount), newAccountID)

	return flow.BytesToAddress(newAccountID), nil
}

func (r *LedgerAccess) SetAccountPublicKeys(accountID []byte, publicKeys [][]byte) error {
	var existingCount int

	countBytes, err := r.Ledger.Get(
		fullKeyHash(string(accountID), string(accountID), keyPublicKeyCount),
	)
	if err != nil {
		return err
	}

	if countBytes != nil {
		existingCount = int(big.NewInt(0).SetBytes(countBytes).Int64())
	} else {
		existingCount = 0
	}

	newCount := len(publicKeys)

	r.Ledger.Set(
		fullKeyHash(string(accountID), string(accountID), keyPublicKeyCount),
		big.NewInt(int64(newCount)).Bytes(),
	)

	for i, publicKey := range publicKeys {

		accountPublicKey, err := decodePublicKey(publicKey)
		if err != nil {
			return err
		}

		err = accountPublicKey.Validate()
		if err != nil {
			return err
		}

		r.Ledger.Set(
			fullKeyHash(string(accountID), string(accountID), keyPublicKey(i)),
			publicKey,
		)
	}

	// delete leftover keys
	for i := newCount; i < existingCount; i++ {
		r.Ledger.Delete(fullKeyHash(string(accountID), string(accountID), keyPublicKey(i)))
	}

	return nil
}
