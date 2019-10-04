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
	registers *types.RegistersView
	Accounts  []types.Address
	Logger    func(string)
}

func NewRuntimeContext(registers *types.RegistersView) *RuntimeContext {
	return &RuntimeContext{
		registers: registers,
		Logger:    func(string) {},
	}
}

func (r *RuntimeContext) GetValue(owner, controller, key []byte) ([]byte, error) {
	v, _ := r.registers.Get(fullKey(string(owner), string(controller), string(key)))
	return v, nil
}

func (r *RuntimeContext) SetValue(owner, controller, key, value []byte) error {
	r.registers.Set(fullKey(string(owner), string(controller), string(key)), value)
	return nil
}

func (r *RuntimeContext) CreateAccount(publicKeys [][]byte, code []byte) (rd []byte, err error) {
	latestAccountID, _ := r.registers.Get(keyLatestAccount)

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := types.BytesToAddress(accountIDBytes)

	accountID := accountAddress.Bytes()

	r.registers.Set(fullKey(string(accountID), "", keyBalance), big.NewInt(0).Bytes())
	r.registers.Set(fullKey(string(accountID), string(accountID), keyCode), code)
	r.setAccountPublicKeys(accountID, publicKeys)

	r.registers.Set(keyLatestAccount, accountID)

	r.Log("Creating new account\n")
	r.Log(fmt.Sprintf("Address: %s", accountAddress.Hex()))
	r.Log(fmt.Sprintf("Code:\n%s", string(code)))

	return accountID, nil
}

func (r *RuntimeContext) setAccountPublicKeys(accountID []byte, publicKeys [][]byte) {
	count := big.NewInt(int64(len(publicKeys)))

	r.registers.Set(
		fullKey(string(accountID), string(accountID), keyPublicKeyCount),
		count.Bytes(),
	)

	for i, publicKey := range publicKeys {
		r.registers.Set(
			fullKey(string(accountID), string(accountID), keyPublicKey(i)),
			publicKey,
		)
	}
}

func (r *RuntimeContext) getAccountPublicKeys(accountID []byte) ([][]byte, error) {
	countBytes, exists := r.registers.Get(
		fullKey(string(accountID), string(accountID), keyPublicKeyCount),
	)
	if !exists {
		return [][]byte{}, nil
	}

	count := int(big.NewInt(0).SetBytes(countBytes).Int64())

	publicKeys := make([][]byte, count)

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

func (r *RuntimeContext) UpdateAccountCode(accountID, code []byte) (err error) {
	_, exists := r.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
		return fmt.Errorf("Account with ID %s does not exist", accountID)
	}

	r.registers.Set(fullKey(string(accountID), string(accountID), keyCode), code)

	return nil
}

func (r *RuntimeContext) GetAccount(address types.Address) *types.Account {
	accountID := address.Bytes()

	balanceBytes, exists := r.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
		return nil
	}

	balanceInt := big.NewInt(0).SetBytes(balanceBytes)

	// TODO: handle errors properly
	code, _ := r.registers.Get(fullKey(string(accountID), string(accountID), keyCode))
	publicKeys, _ := r.getAccountPublicKeys(accountID)

	return &types.Account{
		Address:    address,
		Balance:    balanceInt.Uint64(),
		Code:       code,
		PublicKeys: publicKeys,
	}
}

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

func (r *RuntimeContext) GetSigningAccounts() []types.Address {
	return r.Accounts
}

func (r *RuntimeContext) Log(message string) {
	r.Logger(message)
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
