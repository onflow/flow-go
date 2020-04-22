package virtualmachine

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"

	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/encoding/rlp"
	"github.com/dapperlabs/flow-go/model/flow"
)

type CheckerFunc func([]byte, runtime.Location) error

type TransactionContext struct {
	ledger            Ledger
	signingAccounts   []runtime.Address
	checker           CheckerFunc
	logs              []string
	events            []runtime.Event
	OnSetValueHandler func(owner, controller, key, value []byte)
}

type TransactionContextOption func(*TransactionContext)

// GetSigningAccounts gets the signing accounts for this context.
//
// Signing accounts are the accounts that signed the transaction executing
// inside this context.
func (r *TransactionContext) GetSigningAccounts() []runtime.Address {
	return r.signingAccounts
}

// SetChecker sets the semantic checker function for this context.
func (r *TransactionContext) SetChecker(checker CheckerFunc) {
	r.checker = checker
}

// Events returns all events emitted by the runtime to this context.
func (r *TransactionContext) Events() []runtime.Event {
	return r.events
}

// Logs returns all logs emitted by the runtime to this context.
func (r *TransactionContext) Logs() []string {
	return r.logs
}

// GetValue gets a register value from the world state.
func (r *TransactionContext) GetValue(owner, controller, key []byte) ([]byte, error) {
	v, _ := r.ledger.Get(fullKeyHash(string(owner), string(controller), string(key)))
	return v, nil
}

// SetValue sets a register value in the world state.
func (r *TransactionContext) SetValue(owner, controller, key, value []byte) error {
	r.ledger.Set(fullKeyHash(string(owner), string(controller), string(key)), value)
	if r.OnSetValueHandler != nil {
		r.OnSetValueHandler(owner, controller, key, value)
	}
	return nil
}

func CreateAccountInLedger(ledger Ledger, publicKeys [][]byte) (runtime.Address, error) {
	latestAccountID, _ := ledger.Get(fullKeyHash("", "", keyLatestAccount))

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := flow.BytesToAddress(accountIDBytes)

	accountID := accountAddress[:]

	// mark that account with this ID exists
	ledger.Set(fullKeyHash(string(accountID), "", keyExists), []byte{1})

	// set account balance to 0
	ledger.Set(fullKeyHash(string(accountID), "", keyBalance), big.NewInt(0).Bytes())

	err := setAccountPublicKeys(ledger, accountID, publicKeys)
	if err != nil {
		return runtime.Address{}, err
	}

	ledger.Set(fullKeyHash("", "", keyLatestAccount), accountID)

	return runtime.Address(accountAddress), nil
}

// CreateAccount creates a new account and inserts it into the world state.
//
// This function returns an error if the input is invalid.
//
// After creating the account, this function calls the onAccountCreated callback registered
// with this context.
func (r *TransactionContext) CreateAccount(publicKeys [][]byte) (runtime.Address, error) {
	accountAddress, err := CreateAccountInLedger(r.ledger, publicKeys)
	r.Log("Creating new account\n")
	r.Log(fmt.Sprintf("Address: %x", accountAddress))

	return accountAddress, err
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (r *TransactionContext) AddAccountKey(address runtime.Address, publicKey []byte) error {
	accountID := address[:]

	err := r.checkAccountExists(accountID)
	if err != nil {
		return err
	}

	publicKeys, err := r.getAccountPublicKeys(accountID)
	if err != nil {
		return err
	}

	publicKeys = append(publicKeys, publicKey)

	return setAccountPublicKeys(r.ledger, accountID, publicKeys)
}

// RemoveAccountKey removes a public key by index from an existing account.
//
// This function returns an error if the specified account does not exist, the
// provided key is invalid, or if key deletion fails.
func (r *TransactionContext) RemoveAccountKey(address runtime.Address, index int) (publicKey []byte, err error) {
	accountID := address[:]

	err = r.checkAccountExists(accountID)
	if err != nil {
		return nil, err
	}

	publicKeys, err := r.getAccountPublicKeys(accountID)
	if err != nil {
		return publicKey, err
	}

	if index < 0 || index > len(publicKeys)-1 {
		return publicKey, fmt.Errorf("invalid key index %d, account has %d keys", index, len(publicKeys))
	}

	removedKey := publicKeys[index]

	publicKeys = append(publicKeys[:index], publicKeys[index+1:]...)

	err = setAccountPublicKeys(r.ledger, accountID, publicKeys)
	if err != nil {
		return publicKey, err
	}

	return removedKey, nil
}

func (r *TransactionContext) getAccountPublicKeys(accountID []byte) (publicKeys [][]byte, err error) {
	countBytes, err := r.ledger.Get(
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
		publicKey, err := r.ledger.Get(
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

func setAccountPublicKeys(ledger Ledger, accountID []byte, publicKeys [][]byte) error {
	var existingCount int

	countBytes, err := ledger.Get(
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

	ledger.Set(
		fullKeyHash(string(accountID), string(accountID), keyPublicKeyCount),
		big.NewInt(int64(newCount)).Bytes(),
	)

	for i, publicKey := range publicKeys {
		ledger.Set(
			fullKeyHash(string(accountID), string(accountID), keyPublicKey(i)),
			publicKey,
		)
	}

	// delete leftover keys
	for i := newCount; i < existingCount; i++ {
		ledger.Delete(fullKeyHash(string(accountID), string(accountID), keyPublicKey(i)))
	}

	return nil
}

// CheckCode checks the code for its validity.
func (r *TransactionContext) CheckCode(address runtime.Address, code []byte) (err error) {
	return r.checkProgram(code, address)
}

// UpdateAccountCode updates the deployed code on an existing account.
//
// This function returns an error if the specified account does not exist or is
// not a valid signing account.
func (r *TransactionContext) UpdateAccountCode(address runtime.Address, code []byte, checkPermission bool) (err error) {
	accountID := address[:]

	if checkPermission && !r.isValidSigningAccount(address) {
		return fmt.Errorf("not permitted to update account with ID %s", address)
	}

	err = r.checkAccountExists(accountID)
	if err != nil {
		return err
	}

	r.ledger.Set(fullKeyHash(string(accountID), string(accountID), keyCode), code)

	return nil
}

// ResolveImport imports code for the provided import location.
//
// This function returns an error if the import location is not an account address,
// or if there is no code deployed at the specified address.
func (r *TransactionContext) ResolveImport(location runtime.Location) ([]byte, error) {
	addressLocation, ok := location.(runtime.AddressLocation)
	if !ok {
		return nil, fmt.Errorf("import location must be an account address")
	}

	address := flow.BytesToAddress(addressLocation)

	accountID := address.Bytes()

	code, err := r.ledger.Get(fullKeyHash(string(accountID), string(accountID), keyCode))
	if err != nil {
		return nil, err
	}

	if code == nil {
		return nil, fmt.Errorf("no code deployed at address %x", accountID)
	}

	return code, nil
}

// Log captures a log message from the runtime.
func (r *TransactionContext) Log(message string) {
	r.logs = append(r.logs, message)
}

// EmitEvent is called when an event is emitted by the runtime.
func (r *TransactionContext) EmitEvent(event runtime.Event) {
	r.events = append(r.events, event)
}

// GetAccount gets an account by address.
//
// The function returns nil if the specified account does not exist.
func (r *TransactionContext) GetAccount(address flow.Address) *flow.Account {
	accountID := address.Bytes()

	err := r.checkAccountExists(accountID)
	if err != nil {
		return nil
	}

	balanceBytes, _ := r.ledger.Get(fullKeyHash(string(accountID), "", keyBalance))
	balanceInt := big.NewInt(0).SetBytes(balanceBytes)

	code, _ := r.ledger.Get(fullKeyHash(string(accountID), string(accountID), keyCode))

	publicKeys, err := r.getAccountPublicKeys(accountID)
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

func (r *TransactionContext) isValidSigningAccount(address runtime.Address) bool {
	for _, accountAddress := range r.GetSigningAccounts() {
		if accountAddress == address {
			return true
		}
	}

	return false
}

// checkProgram checks the given code for syntactic and semantic correctness.
func (r *TransactionContext) checkProgram(code []byte, address runtime.Address) error {
	if code == nil {
		return nil
	}

	location := runtime.AddressLocation(address[:])

	return r.checker(code, location)
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

func (r *TransactionContext) checkAccountExists(accountID []byte) error {
	exists, err := r.ledger.Get(fullKeyHash(string(accountID), "", keyExists))
	if err != nil {
		return err
	}

	bal, err := r.ledger.Get(fullKeyHash(string(accountID), "", keyBalance))
	if err != nil {
		return err
	}

	if len(exists) != 0 || bal != nil {
		return nil
	}

	return fmt.Errorf("account with ID %s does not exist", accountID)
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
