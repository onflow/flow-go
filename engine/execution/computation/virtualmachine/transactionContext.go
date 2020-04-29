package virtualmachine

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

type CheckerFunc func([]byte, runtime.Location) error

type TransactionContext struct {
	LedgerAccess
	signingAccounts   []runtime.Address
	checker           CheckerFunc
	logs              []string
	events            []runtime.Event
	OnSetValueHandler func(owner, controller, key, value []byte)
	gasUsed           uint64 // TODO fill with actual gas
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
	v, _ := r.Ledger.Get(fullKeyHash(string(owner), string(controller), string(key)))
	return v, nil
}

// SetValue sets a register value in the world state.
func (r *TransactionContext) SetValue(owner, controller, key, value []byte) error {
	r.Ledger.Set(fullKeyHash(string(owner), string(controller), string(key)), value)
	if r.OnSetValueHandler != nil {
		r.OnSetValueHandler(owner, controller, key, value)
	}
	return nil
}

// CreateAccount creates a new account and inserts it into the world state.
//
// This function returns an error if the input is invalid.
//
// After creating the account, this function calls the onAccountCreated callback registered
// with this context.
func (r *TransactionContext) CreateAccount(publicKeys [][]byte) (runtime.Address, error) {
	accountAddress, err := r.CreateAccountInLedger(publicKeys)
	r.Log("Creating new account\n")
	r.Log(fmt.Sprintf("Address: %x", accountAddress))

	return runtime.Address(accountAddress), err
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (r *TransactionContext) AddAccountKey(address runtime.Address, publicKey []byte) error {
	accountID := address[:]

	err := r.CheckAccountExists(accountID)
	if err != nil {
		return err
	}

	publicKeys, err := r.GetAccountPublicKeys(accountID)
	if err != nil {
		return err
	}

	publicKeys = append(publicKeys, publicKey)

	return r.SetAccountPublicKeys(accountID, publicKeys)
}

// RemoveAccountKey removes a public key by index from an existing account.
//
// This function returns an error if the specified account does not exist, the
// provided key is invalid, or if key deletion fails.
func (r *TransactionContext) RemoveAccountKey(address runtime.Address, index int) (publicKey []byte, err error) {
	accountID := address[:]

	err = r.CheckAccountExists(accountID)
	if err != nil {
		return nil, err
	}

	publicKeys, err := r.GetAccountPublicKeys(accountID)
	if err != nil {
		return publicKey, err
	}

	if index < 0 || index > len(publicKeys)-1 {
		return publicKey, fmt.Errorf("invalid key index %d, account has %d keys", index, len(publicKeys))
	}

	removedKey := publicKeys[index]

	publicKeys = append(publicKeys[:index], publicKeys[index+1:]...)

	err = r.SetAccountPublicKeys(accountID, publicKeys)
	if err != nil {
		return publicKey, err
	}

	return removedKey, nil
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

	err = r.CheckAccountExists(accountID)
	if err != nil {
		return err
	}

	r.Ledger.Set(fullKeyHash(string(accountID), string(accountID), keyCode), code)

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

	code, err := r.Ledger.Get(fullKeyHash(string(accountID), string(accountID), keyCode))
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
