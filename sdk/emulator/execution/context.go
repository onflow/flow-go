package execution

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dapperlabs/flow-go/pkg/language/runtime"
	"github.com/dapperlabs/flow-go/pkg/types"
)

type RuntimeContext struct {
	registers        *types.RegistersView
	Accounts         []types.Address
	onLog            func(string)
	onAccountCreated func(account types.Account)
}

func NewRuntimeContext(registers *types.RegistersView) *RuntimeContext {
	return &RuntimeContext{
		registers:        registers,
		onLog:            func(string) {},
		onAccountCreated: func(types.Account) {},
	}
}

func (i *RuntimeContext) SetLogger(callback func(string)) {
	i.onLog = callback
}

func (i *RuntimeContext) SetOnAccountCreated(callback func(account types.Account)) {
	i.onAccountCreated = callback
}

func (i *RuntimeContext) GetValue(owner, controller, key []byte) ([]byte, error) {
	v, _ := i.registers.Get(fullKey(string(owner), string(controller), string(key)))
	return v, nil
}

func (i *RuntimeContext) SetValue(owner, controller, key, value []byte) error {
	i.registers.Set(fullKey(string(owner), string(controller), string(key)), value)
	return nil
}

func (i *RuntimeContext) CreateAccount(publicKey, code []byte) (id []byte, err error) {
	latestAccountID, _ := i.registers.Get(keyLatestAccount)

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := types.BytesToAddress(accountIDBytes)

	accountID := accountAddress.Bytes()

	i.registers.Set(fullKey(string(accountID), "", keyBalance), big.NewInt(0).Bytes())
	i.registers.Set(fullKey(string(accountID), string(accountID), keyPublicKey), publicKey)
	i.registers.Set(fullKey(string(accountID), string(accountID), keyCode), code)

	i.registers.Set(keyLatestAccount, accountID)

	i.Log("Creating new account\n")
	i.Log(fmt.Sprintf("Address: %s", accountAddress.Hex()))
	i.Log(fmt.Sprintf("Code:\n%s", string(code)))

	account := i.GetAccount(accountAddress)
	i.onAccountCreated(*account)

	return accountID, nil
}

func (i *RuntimeContext) UpdateAccountCode(address types.Address, code []byte) (err error) {
	accountID := address.Bytes()

	_, exists := i.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
		return fmt.Errorf("Account with ID %s does not exist", accountID)
	}

	i.registers.Set(fullKey(string(accountID), string(accountID), keyCode), code)

	return nil
}

func (i *RuntimeContext) GetAccount(address types.Address) *types.Account {
	accountID := address.Bytes()

	balanceBytes, exists := i.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
		return nil
	}

	balanceInt := big.NewInt(0).SetBytes(balanceBytes)

	publicKey, _ := i.registers.Get(fullKey(string(accountID), string(accountID), keyPublicKey))
	code, _ := i.registers.Get(fullKey(string(accountID), string(accountID), keyCode))

	return &types.Account{
		Address:    address,
		Balance:    balanceInt.Uint64(),
		Code:       code,
		PublicKeys: [][]byte{publicKey},
	}
}

func (i *RuntimeContext) ResolveImport(location runtime.ImportLocation) ([]byte, error) {
	addressLocation, ok := location.(runtime.AddressImportLocation)
	if !ok {
		return nil, errors.New("import location must be an account address")
	}

	accountID := []byte(addressLocation)

	code, exists := i.registers.Get(fullKey(string(accountID), string(accountID), keyCode))
	if !exists {
		return nil, fmt.Errorf("no code deployed at address %x", accountID)
	}

	return code, nil
}

func (i *RuntimeContext) Log(message string) {
	i.onLog(message)
}

func (i *RuntimeContext) GetSigningAccounts() []types.Address {
	return i.Accounts
}

const (
	keyLatestAccount = "latest_account"
	keyBalance       = "balance"
	keyPublicKey     = "public_key"
	keyCode          = "code"
)

func fullKey(owner, controller, key string) string {
	return strings.Join([]string{owner, controller, key}, "__")
}
