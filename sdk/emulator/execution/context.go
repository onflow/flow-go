package execution

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

type CheckerFunc func([]byte, runtime.Location) error

// RuntimeContext implements host functionality required by the Cadence runtime.
//
// A context is short-lived and is intended to be used when executing a single transaction.
//
// The logic in this runtime context is specific to the emulator and is designed to be
// used with a Blockchain instance.
type RuntimeContext struct {
	ledger          *types.LedgerView
	signingAccounts []values.Address
	checker         CheckerFunc
	logs            []string
	events          []values.Event
}

// NewRuntimeContext returns a new RuntimeContext instance.
func NewRuntimeContext(ledger *types.LedgerView) *RuntimeContext {
	return &RuntimeContext{
		ledger:  ledger,
		checker: func([]byte, runtime.Location) error { return nil },
		events:  make([]values.Event, 0),
	}
}

// SetSigningAccounts sets the signing accounts for this context.
//
// Signing accounts are the accounts that signed the transaction executing
// inside this context.
func (r *RuntimeContext) SetSigningAccounts(accounts []flow.Address) {
	signingAccounts := make([]values.Address, len(accounts))

	for i, account := range accounts {
		signingAccounts[i] = values.Address(account)
	}

	r.signingAccounts = signingAccounts
}

// GetSigningAccounts gets the signing accounts for this context.
//
// Signing accounts are the accounts that signed the transaction executing
// inside this context.
func (r *RuntimeContext) GetSigningAccounts() []values.Address {
	return r.signingAccounts
}

// SetChecker sets the semantic checker function for this context.
func (r *RuntimeContext) SetChecker(checker CheckerFunc) {
	r.checker = checker
}

// Events returns all events emitted by the runtime to this context.
func (r *RuntimeContext) Events() []values.Event {
	return r.events
}

// Logs returns all logs emitted by the runtime to this context.
func (r *RuntimeContext) Logs() []string {
	return r.logs
}

// GetValue gets a register value from the world state.
func (r *RuntimeContext) GetValue(owner, controller, key values.Bytes) (values.Bytes, error) {
	v, _ := r.ledger.Get(fullKey(string(owner), string(controller), string(key)))
	return v, nil
}

// SetValue sets a register value in the world state.
func (r *RuntimeContext) SetValue(owner, controller, key, value values.Bytes) error {
	r.ledger.Set(fullKey(string(owner), string(controller), string(key)), value)
	return nil
}

// CreateAccount creates a new account and inserts it into the world state.
//
// This function returns an error if the input is invalid.
//
// After creating the account, this function calls the onAccountCreated callback registered
// with this context.
func (r *RuntimeContext) CreateAccount(publicKeys []values.Bytes) (values.Address, error) {
	latestAccountID, _ := r.ledger.Get(keyLatestAccount)

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := values.Address(flow.BytesToAddress(accountIDBytes))

	accountID := accountAddress[:]

	r.ledger.Set(fullKey(string(accountID), "", keyBalance), big.NewInt(0).Bytes())

	err := r.setAccountPublicKeys(accountID, publicKeys)
	if err != nil {
		return values.Address{}, err
	}

	r.ledger.Set(keyLatestAccount, accountID)

	r.Log("Creating new account\n")
	r.Log(fmt.Sprintf("Address: %x", accountAddress))

	return accountAddress, nil
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (r *RuntimeContext) AddAccountKey(address values.Address, publicKey values.Bytes) error {
	accountID := address[:]

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

// RemoveAccountKey removes a public key by index from an existing account.
//
// This function returns an error if the specified account does not exist, the
// provided key is invalid, or if key deletion fails.
func (r *RuntimeContext) RemoveAccountKey(address values.Address, index values.Int) (publicKey values.Bytes, err error) {
	accountID := address[:]
	i := index.ToInt()

	bal, err := r.ledger.Get(fullKey(string(accountID), "", keyBalance))
	if err != nil {
		return nil, err
	}
	if bal == nil {
		return nil, fmt.Errorf("account with ID %s does not exist", accountID)
	}

	publicKeys, err := r.getAccountPublicKeys(accountID)
	if err != nil {
		return nil, err
	}

	if i < 0 || i > len(publicKeys)-1 {
		return nil, fmt.Errorf("invalid key index %d, account has %d keys", i, len(publicKeys))
	}

	removedKey := publicKeys[i]

	publicKeys = append(publicKeys[:i], publicKeys[i+1:]...)

	err = r.setAccountPublicKeys(accountID, publicKeys)
	if err != nil {
		return nil, err
	}

	return removedKey, nil
}

func (r *RuntimeContext) getAccountPublicKeys(accountID []byte) (publicKeys []values.Bytes, err error) {
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

		publicKeys[i] = publicKey
	}

	return publicKeys, nil
}

func (r *RuntimeContext) setAccountPublicKeys(accountID []byte, publicKeys []values.Bytes) error {
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

// CheckCode checks the code for its validity.
func (r *RuntimeContext) CheckCode(address values.Address, code values.Bytes) (err error) {
	return r.checkProgram(code, address)
}

// UpdateAccountCode updates the deployed code on an existing account.
//
// This function returns an error if the specified account does not exist or is
// not a valid signing account.
func (r *RuntimeContext) UpdateAccountCode(address values.Address, code values.Bytes, checkPermission bool) (err error) {
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

// GetAccount gets an account by address.
//
// The function returns nil if the specified account does not exist.
func (r *RuntimeContext) GetAccount(address flow.Address) *flow.Account {
	accountID := address.Bytes()

	balanceBytes, _ := r.ledger.Get(fullKey(string(accountID), "", keyBalance))
	if balanceBytes == nil {
		return nil
	}

	balanceInt := big.NewInt(0).SetBytes(balanceBytes)

	code, _ := r.ledger.Get(fullKey(string(accountID), string(accountID), keyCode))

	publicKeys, err := r.getAccountPublicKeys(accountID)
	if err != nil {
		panic(err)
	}

	accountPublicKeys := make([]flow.AccountPublicKey, len(publicKeys))
	for i, publicKey := range publicKeys {
		accountPublicKey, err := flow.DecodeAccountPublicKey(publicKey)
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

// ResolveImport imports code for the provided import location.
//
// This function returns an error if the import location is not an account address,
// or if there is no code deployed at the specified address.
func (r *RuntimeContext) ResolveImport(location runtime.Location) (values.Bytes, error) {
	addressLocation, ok := location.(runtime.AddressLocation)
	if !ok {
		return nil, errors.New("import location must be an account address")
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

// Log captures a log message from the runtime.
func (r *RuntimeContext) Log(message string) {
	r.logs = append(r.logs, message)
}

// EmitEvent is called when an event is emitted by the runtime.
func (r *RuntimeContext) EmitEvent(event values.Event) {
	r.events = append(r.events, event)
}

func (r *RuntimeContext) isValidSigningAccount(address values.Address) bool {
	for _, accountAddress := range r.GetSigningAccounts() {
		if accountAddress == address {
			return true
		}
	}

	return false
}

// checkProgram checks the given code for syntactic and semantic correctness.
func (r *RuntimeContext) checkProgram(code []byte, address values.Address) error {
	if code == nil {
		return nil
	}

	location := runtime.AddressLocation(address[:])

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
