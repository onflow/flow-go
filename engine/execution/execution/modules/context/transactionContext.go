package context

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/dapperlabs/flow-go/engine/execution/execution/modules/ledger"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

type CheckerFunc func([]byte, runtime.Location) error

type transactionContext struct {
	ledger          *ledger.View
	signingAccounts []values.Address
	checker         CheckerFunc
	logs            []string
	events          []values.Event
}

func (r *transactionContext) GetSigningAccounts() []values.Address {
	return r.signingAccounts
}

func (r *transactionContext) SetChecker(checker CheckerFunc) {
	r.checker = checker
}

func (r *transactionContext) Events() []values.Event {
	return r.events
}

func (r *transactionContext) Logs() []string {
	return r.logs
}

func (r *transactionContext) GetValue(owner, controller, key []byte) ([]byte, error) {
	v, _ := r.ledger.Get(fullKey(string(owner), string(controller), string(key)))
	return v, nil
}

func (r *transactionContext) SetValue(owner, controller, key, value []byte) error {
	r.ledger.Set(fullKey(string(owner), string(controller), string(key)), value)
	return nil
}

func (r *transactionContext) CreateAccount(publicKeys []values.Bytes) (values.Address, error) {
	latestAccountID, _ := r.ledger.Get(keyLatestAccount)

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := flow.BytesToAddress(accountIDBytes)

	accountID := accountAddress[:]

	r.ledger.Set(fullKey(string(accountID), "", keyBalance), big.NewInt(0).Bytes())

	err := r.setAccountPublicKeys(accountID, publicKeys)
	if err != nil {
		return values.Address{}, err
	}

	r.ledger.Set(keyLatestAccount, accountID)

	r.Log("Creating new account\n")
	r.Log(fmt.Sprintf("Address: %x", accountAddress))

	return values.NewAddress(accountAddress), nil
}

func (r *transactionContext) AddAccountKey(address values.Address, publicKey values.Bytes) error {
	accountID := address.Bytes()

	bal, err := r.ledger.Get(fullKey(string(accountID), "", keyBalance))
	if err != nil {
		return err
	}
	if bal == nil {
		return fmt.Errorf("account with ID %s does not exist", accountID)
	}

	publicKeys, err := r.getAccountPublicKeys(accountID)
	if err != nil {
		return err
	}

	publicKeys = append(publicKeys, publicKey)

	return r.setAccountPublicKeys(accountID, publicKeys)
}

func (r *transactionContext) RemoveAccountKey(address values.Address, index values.Int) (publicKey values.Bytes, err error) {
	accountID := address.Bytes()
	i := index.Int()

	bal, err := r.ledger.Get(fullKey(string(accountID), "", keyBalance))
	if err != nil {
		return nil, err
	}
	if bal == nil {
		return nil, fmt.Errorf("account with ID %s does not exist", accountID)
	}

	publicKeys, err := r.getAccountPublicKeys(accountID)
	if err != nil {
		return publicKey, err
	}

	if i < 0 || i > len(publicKeys)-1 {
		return publicKey, fmt.Errorf("invalid key index %d, account has %d keys", i, len(publicKeys))
	}

	removedKey := publicKeys[i]

	publicKeys = append(publicKeys[:i], publicKeys[i+1:]...)

	err = r.setAccountPublicKeys(accountID, publicKeys)
	if err != nil {
		return publicKey, err
	}

	return removedKey, nil
}

func (r *transactionContext) getAccountPublicKeys(accountID []byte) (publicKeys []values.Bytes, err error) {
	countBytes, err := r.ledger.Get(
		fullKey(string(accountID), string(accountID), keyPublicKeyCount),
	)
	if err != nil {
		return nil, err
	}

	if countBytes == nil {
		return nil, fmt.Errorf("key count not set")
	}

	count := int(big.NewInt(0).SetBytes(countBytes).Int64())

	publicKeys = make([]values.Bytes, count)

	for i := 0; i < count; i++ {
		publicKey, err := r.ledger.Get(
			fullKey(string(accountID), string(accountID), keyPublicKey(i)),
		)
		if err != nil {
			return nil, err
		}

		if publicKey == nil {
			return nil, fmt.Errorf("failed to retrieve key from account %s", accountID)
		}

		publicKeys[i] = values.NewBytes(publicKey)
	}

	return publicKeys, nil
}

func (r *transactionContext) setAccountPublicKeys(accountID []byte, publicKeys []values.Bytes) error {
	var existingCount int

	countBytes, err := r.ledger.Get(
		fullKey(string(accountID), string(accountID), keyPublicKeyCount),
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

	r.ledger.Set(
		fullKey(string(accountID), string(accountID), keyPublicKeyCount),
		big.NewInt(int64(newCount)).Bytes(),
	)

	for i, publicKey := range publicKeys {
		if err := keys.ValidateEncodedPublicKey(publicKey); err != nil {
			return err
		}

		r.ledger.Set(
			fullKey(string(accountID), string(accountID), keyPublicKey(i)),
			publicKey,
		)
	}

	// delete leftover keys
	for i := newCount; i < existingCount; i++ {
		r.ledger.Delete(fullKey(string(accountID), string(accountID), keyPublicKey(i)))
	}

	return nil
}

func (r *transactionContext) CheckCode(address values.Address, code values.Bytes) (err error) {
	return r.checkProgram(code, address)
}

func (r *transactionContext) UpdateAccountCode(address values.Address, code values.Bytes, checkPermission bool) (err error) {
	accountID := address[:]

	if checkPermission && !r.isValidSigningAccount(address) {
		return fmt.Errorf("not permitted to update account with ID %x", accountID)
	}

	bal, err := r.ledger.Get(fullKey(string(accountID), "", keyBalance))
	if err != nil {
		return err
	}
	if bal == nil {
		return fmt.Errorf("account with ID %s does not exist", accountID)
	}

	r.ledger.Set(fullKey(string(accountID), string(accountID), keyCode), code)

	return nil
}

// ResolveImport imports code for the provided import location.
//
// This function returns an error if the import location is not an account address,
// or if there is no code deployed at the specified address.
func (r *transactionContext) ResolveImport(location runtime.Location) ([]byte, error) {
	addressLocation, ok := location.(runtime.AddressLocation)
	if !ok {
		return nil, fmt.Errorf("import location must be an account address")
	}

	address := flow.BytesToAddress(addressLocation)

	accountID := address.Bytes()

	code, err := r.ledger.Get(fullKey(string(accountID), string(accountID), keyCode))
	if err != nil {
		return nil, err
	}

	if code == nil {
		return nil, fmt.Errorf("no code deployed at address %x", accountID)
	}

	return code, nil
}

func (r *transactionContext) Log(message string) {
	r.logs = append(r.logs, message)
}

func (r *transactionContext) EmitEvent(event values.Event) {
	r.events = append(r.events, event)
}

func (r *transactionContext) isValidSigningAccount(address values.Address) bool {
	for _, accountAddress := range r.GetSigningAccounts() {
		if accountAddress == address {
			return true
		}
	}

	return false
}

func (r *transactionContext) checkProgram(code []byte, address values.Address) error {
	if code == nil {
		return nil
	}

	location := runtime.AddressLocation(address.Bytes())

	return r.checker(code, location)
}

const (
	keyLatestAccount  = "latest_account"
	keyBalance        = "balance"
	keyCode           = "code"
	keyPublicKeyCount = "public_key_count"
)

func fullKey(owner, controller, key string) string {
	return strings.Join([]string{owner, controller, key}, "__")
}

func keyPublicKey(index int) string {
	return fmt.Sprintf("public_key_%d", index)
}
