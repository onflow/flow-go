package execution

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dapperlabs/flow-go/pkg/language/runtime"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

// RuntimeContext implements host functionality required by the Cadence runtime.
//
// A context is short-lived and is intended to be used when executing a single transaction.
//
// The logic in this runtime context is specific to the emulator and is designed to be
// used with an EmulatedBlockchain instance.
type RuntimeContext struct {
	registers       *types.RegistersView
	signingAccounts []types.Address
	onLogMessage    func(string)
	events          []types.Event
}

// NewRuntimeContext returns a new RuntimeContext instance.
func NewRuntimeContext(registers *types.RegistersView) *RuntimeContext {
	return &RuntimeContext{
		registers:    registers,
		onLogMessage: func(string) {},
		events:       make([]types.Event, 0),
	}
}

// SetSigningAccounts sets the signing accounts for this context.
//
// Signing accounts are the accounts that signed the transaction executing
// inside this context.
func (r *RuntimeContext) SetSigningAccounts(accounts []types.Address) {
	r.signingAccounts = accounts
}

// GetSigningAccounts gets the signing accounts for this context.
//
// Signing accounts are the accounts that signed the transaction executing
// inside this context.
func (r *RuntimeContext) GetSigningAccounts() []types.Address {
	return r.signingAccounts
}

// SetOnLogMessage sets the logging function for this context.
func (r *RuntimeContext) SetOnLogMessage(callback func(string)) {
	r.onLogMessage = callback
}

// Events returns all events emitted by the runtime to this context.
func (r *RuntimeContext) Events() []types.Event {
	return r.events
}

// GetValue gets a register value from the world state.
func (r *RuntimeContext) GetValue(owner, controller, key []byte) ([]byte, error) {
	v, _ := r.registers.Get(fullKey(string(owner), string(controller), string(key)))
	return v, nil
}

// SetValue sets a register value in the world state.
func (r *RuntimeContext) SetValue(owner, controller, key, value []byte) error {
	r.registers.Set(fullKey(string(owner), string(controller), string(key)), value)
	return nil
}

// CreateAccount creates a new account and inserts it into the world state.
//
// This function returns an error if the input is invalid.
//
// After creating the account, this function calls the onAccountCreated callback registered
// with this context.
func (r *RuntimeContext) CreateAccount(publicKeys [][]byte, code []byte) (types.Address, error) {
	latestAccountID, _ := r.registers.Get(keyLatestAccount)

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := types.BytesToAddress(accountIDBytes)

	accountID := accountAddress.Bytes()

	r.registers.Set(fullKey(string(accountID), "", keyBalance), big.NewInt(0).Bytes())
	r.registers.Set(fullKey(string(accountID), string(accountID), keyCode), code)

	err := r.setAccountPublicKeys(accountID, publicKeys)
	if err != nil {
		return types.Address{}, err
	}

	r.registers.Set(keyLatestAccount, accountID)

	r.Log("Creating new account\n")
	r.Log(fmt.Sprintf("Address: %s", accountAddress.Hex()))
	r.Log(fmt.Sprintf("Code:\n%s", string(code)))

	return accountAddress, nil
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (r *RuntimeContext) AddAccountKey(address types.Address, publicKey []byte) error {
	accountID := address.Bytes()

	_, exists := r.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
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
func (r *RuntimeContext) RemoveAccountKey(address types.Address, index int) (publicKey []byte, err error) {
	accountID := address.Bytes()

	_, exists := r.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
		return nil, fmt.Errorf("account with ID %s does not exist", accountID)
	}

	publicKeys, err := r.getAccountPublicKeys(accountID)
	if err != nil {
		return nil, err
	}

	if index < 0 || index > len(publicKeys)-1 {
		return nil, fmt.Errorf("invalid key index %d, account has %d keys", index, len(publicKeys))
	}

	removedKey := publicKeys[index]

	publicKeys = append(publicKeys[:index], publicKeys[index+1:]...)

	err = r.setAccountPublicKeys(accountID, publicKeys)
	if err != nil {
		return nil, err
	}

	return removedKey, nil
}

func (r *RuntimeContext) getAccountPublicKeys(accountID []byte) (publicKeys [][]byte, err error) {
	countBytes, exists := r.registers.Get(
		fullKey(string(accountID), string(accountID), keyPublicKeyCount),
	)
	if !exists {
		return [][]byte{}, fmt.Errorf("key count not set")
	}

	count := int(big.NewInt(0).SetBytes(countBytes).Int64())

	publicKeys = make([][]byte, count)

	for i := 0; i < count; i++ {
		publicKey, exists := r.registers.Get(
			fullKey(string(accountID), string(accountID), keyPublicKey(i)),
		)
		if !exists {
			return nil, fmt.Errorf("failed to retrieve key from account %s", accountID)
		}

		publicKeys[i] = publicKey
	}

	return publicKeys, nil
}

func (r *RuntimeContext) setAccountPublicKeys(accountID []byte, publicKeys [][]byte) error {
	var existingCount int

	countBytes, exists := r.registers.Get(
		fullKey(string(accountID), string(accountID), keyPublicKeyCount),
	)
	if exists {
		existingCount = int(big.NewInt(0).SetBytes(countBytes).Int64())
	} else {
		existingCount = 0
	}

	newCount := len(publicKeys)

	r.registers.Set(
		fullKey(string(accountID), string(accountID), keyPublicKeyCount),
		big.NewInt(int64(newCount)).Bytes(),
	)

	for i, publicKey := range publicKeys {
		if err := keys.ValidateEncodedPublicKey(publicKey); err != nil {
			return err
		}

		r.registers.Set(
			fullKey(string(accountID), string(accountID), keyPublicKey(i)),
			publicKey,
		)
	}

	// delete leftover keys
	for i := newCount; i < existingCount; i++ {
		r.registers.Delete(fullKey(string(accountID), string(accountID), keyPublicKey(i)))
	}

	return nil
}

// UpdateAccountCode updates the deployed code on an existing account.
//
// This function returns an error if the specified account does not exist or is
// not a valid signing account.
func (r *RuntimeContext) UpdateAccountCode(address types.Address, code []byte) (err error) {
	accountID := address.Bytes()

	if !r.isValidSigningAccount(address) {
		return fmt.Errorf("not permitted to update account with ID %s", accountID)
	}

	_, exists := r.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
		return fmt.Errorf("account with ID %s does not exist", accountID)
	}

	r.registers.Set(fullKey(string(accountID), string(accountID), keyCode), code)

	return nil
}

// GetAccount gets an account by address.
//
// The function returns nil if the specified account does not exist.
func (r *RuntimeContext) GetAccount(address types.Address) *types.Account {
	accountID := address.Bytes()

	balanceBytes, exists := r.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
		return nil
	}

	balanceInt := big.NewInt(0).SetBytes(balanceBytes)

	code, _ := r.registers.Get(fullKey(string(accountID), string(accountID), keyCode))

	publicKeys, err := r.getAccountPublicKeys(accountID)
	if err != nil {
		panic(err)
	}

	accountPublicKeys := make([]types.AccountPublicKey, len(publicKeys))
	for i, publicKey := range publicKeys {
		accountPublicKey, err := types.DecodeAccountPublicKey(publicKey)
		if err != nil {
			panic(err)
		}

		accountPublicKeys[i] = accountPublicKey
	}

	return &types.Account{
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
func (r *RuntimeContext) ResolveImport(location runtime.ImportLocation) ([]byte, error) {
	addressLocation, ok := location.(runtime.AddressImportLocation)
	if !ok {
		return nil, errors.New("import location must be an account address")
	}

	accountID := []byte(addressLocation)

	code, exists := r.registers.Get(fullKey(string(accountID), string(accountID), keyCode))
	if !exists {
		return nil, fmt.Errorf("no code deployed at address %x", accountID)
	}

	return code, nil
}

// Log logs a message from the runtime.
//
// This functions calls the onLog callback registered with this context.
func (r *RuntimeContext) Log(message string) {
	r.onLogMessage(message)
}

// EmitEvent is called when an event is emitted by the runtime.
func (r *RuntimeContext) EmitEvent(event types.Event) {
	r.events = append(r.events, event)
}

func (r *RuntimeContext) isValidSigningAccount(address types.Address) bool {
	for _, accountAddress := range r.GetSigningAccounts() {
		if accountAddress == address {
			return true
		}
	}

	return false
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
