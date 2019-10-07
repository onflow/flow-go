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
	signingAccounts  []types.Address
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

func (r *RuntimeContext) SetSigningAccounts(accounts []types.Address) {
	r.signingAccounts = accounts
}

func (r *RuntimeContext) SetLogger(callback func(string)) {
	r.onLog = callback
}

func (r *RuntimeContext) SetOnAccountCreated(callback func(account types.Account)) {
	r.onAccountCreated = callback
}

func (r *RuntimeContext) GetValue(owner, controller, key []byte) ([]byte, error) {
	v, _ := r.registers.Get(fullKey(string(owner), string(controller), string(key)))
	return v, nil
}

func (r *RuntimeContext) SetValue(owner, controller, key, value []byte) error {
	r.registers.Set(fullKey(string(owner), string(controller), string(key)), value)
	return nil
}

func (r *RuntimeContext) CreateAccount(publicKeys [][]byte, keyWeights []int, code []byte) ([]byte, error) {
	if len(publicKeys) != len(keyWeights) {
		return nil, fmt.Errorf(
			"publicKeys (length: %d) and keyWeights (length: %d) do not match",
			len(publicKeys),
			len(keyWeights),
		)
	}

	latestAccountID, _ := r.registers.Get(keyLatestAccount)

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := types.BytesToAddress(accountIDBytes)

	accountID := accountAddress.Bytes()

	r.registers.Set(fullKey(string(accountID), "", keyBalance), big.NewInt(0).Bytes())
	r.registers.Set(fullKey(string(accountID), string(accountID), keyCode), code)

	r.setAccountPublicKeys(accountID, publicKeys, keyWeights)

	r.registers.Set(keyLatestAccount, accountID)

	r.Log("Creating new account\n")
	r.Log(fmt.Sprintf("Address: %s", accountAddress.Hex()))
	r.Log(fmt.Sprintf("Code:\n%s", string(code)))

	account := r.GetAccount(accountAddress)
	r.onAccountCreated(*account)

	return accountID, nil
}

func (r *RuntimeContext) AddAccountKey(accountID, publicKey []byte, keyWeight int) error {
	_, exists := r.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
		return fmt.Errorf("account with ID %s does not exist", accountID)
	}

	publicKeys, keyWeights, err := r.getAccountPublicKeys(accountID)
	if err != nil {
		return err
	}

	publicKeys = append(publicKeys, publicKey)
	keyWeights = append(keyWeights, keyWeight)

	r.setAccountPublicKeys(accountID, publicKeys, keyWeights)

	return nil
}

func (r *RuntimeContext) RemoveAccountKey(accountID []byte, index int) error {
	_, exists := r.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
		return fmt.Errorf("account with ID %s does not exist", accountID)
	}

	publicKeys, keyWeights, err := r.getAccountPublicKeys(accountID)
	if err != nil {
		return err
	}

	if index < 0 || index > len(publicKeys)-1 {
		return fmt.Errorf("invalid key index %d, account has %d keys", index, len(publicKeys))
	}

	publicKeys = append(publicKeys[:index], publicKeys[index+1:]...)
	keyWeights = append(keyWeights[:index], keyWeights[index+1:]...)

	r.setAccountPublicKeys(accountID, publicKeys, keyWeights)

	return nil
}

func (r *RuntimeContext) getAccountPublicKeys(accountID []byte) (publicKeys [][]byte, keyWeights []int, err error) {
	countBytes, exists := r.registers.Get(
		fullKey(string(accountID), string(accountID), keyPublicKeyCount),
	)
	if !exists {
		return [][]byte{}, []int{}, fmt.Errorf("key count not set")
	}

	count := int(big.NewInt(0).SetBytes(countBytes).Int64())

	publicKeys = make([][]byte, count)
	keyWeights = make([]int, count)

	for i := 0; i < count; i++ {
		publicKey, publicKeyExists := r.registers.Get(
			fullKey(string(accountID), string(accountID), keyPublicKey(i)),
		)

		keyWeightBytes, keyWeightExists := r.registers.Get(
			fullKey(string(accountID), string(accountID), keyPublicKeyWeight(i)),
		)

		if !publicKeyExists || !keyWeightExists {
			return nil, nil, fmt.Errorf("failed to retrieve key from account %s", accountID)
		}

		publicKeys[i] = publicKey
		keyWeights[i] = int(big.NewInt(0).SetBytes(keyWeightBytes).Int64())
	}

	return publicKeys, keyWeights, nil
}

func (r *RuntimeContext) setAccountPublicKeys(accountID []byte, publicKeys [][]byte, keyWeights []int) error {
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
		r.registers.Set(
			fullKey(string(accountID), string(accountID), keyPublicKey(i)),
			publicKey,
		)

		weight := big.NewInt(int64(keyWeights[i]))

		r.registers.Set(
			fullKey(string(accountID), string(accountID), keyPublicKeyWeight(i)),
			weight.Bytes(),
		)
	}

	// delete leftover keys
	for i := newCount; i < existingCount; i++ {
		r.registers.Delete(fullKey(string(accountID), string(accountID), keyPublicKey(i)))
		r.registers.Delete(fullKey(string(accountID), string(accountID), keyPublicKeyWeight(i)))
	}

	return nil
}

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

func (r *RuntimeContext) GetAccount(address types.Address) *types.Account {
	accountID := address.Bytes()

	balanceBytes, exists := r.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
		return nil
	}

	balanceInt := big.NewInt(0).SetBytes(balanceBytes)

	// TODO: handle errors properly
	code, _ := r.registers.Get(fullKey(string(accountID), string(accountID), keyCode))
	publicKeys, keyWeights, _ := r.getAccountPublicKeys(accountID)

	accountKeys := make([]types.AccountKey, len(publicKeys))
	for i, publicKey := range publicKeys {
		accountKeys[i] = types.AccountKey{
			PublicKey: publicKey,
			Weight:    keyWeights[i],
		}
	}

	return &types.Account{
		Address: address,
		Balance: balanceInt.Uint64(),
		Code:    code,
		Keys:    accountKeys,
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
	return r.signingAccounts
}

func (r *RuntimeContext) Log(message string) {
	r.onLog(message)
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

func keyPublicKeyWeight(index int) string {
	return fmt.Sprintf("public_key_weight_%d", index)
}
