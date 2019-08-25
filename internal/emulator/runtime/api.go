package runtime

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	etypes "github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type EmulatorRuntimeAPI struct {
	registers *etypes.RegistersView
	Accounts  []types.Address
	Logger    func(string)
}

func NewEmulatorRuntimeAPI(registers *etypes.RegistersView) *EmulatorRuntimeAPI {
	return &EmulatorRuntimeAPI{
		registers: registers,
		Logger:    func(string) {},
	}
}

func (i *EmulatorRuntimeAPI) GetValue(owner, controller, key []byte) ([]byte, error) {
	v, _ := i.registers.Get(fullKey(string(owner), string(controller), string(key)))
	return v, nil
}

func (i *EmulatorRuntimeAPI) SetValue(owner, controller, key, value []byte) error {
	i.registers.Set(fullKey(string(owner), string(controller), string(key)), value)
	return nil
}

func (i *EmulatorRuntimeAPI) CreateAccount(publicKey, code []byte) (id []byte, err error) {
	latestAccountID, _ := i.registers.Get(keyLatestAccount)

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := types.BytesToAddress(accountIDBytes)

	accountID := accountAddress.Bytes()

	i.registers.Set(fullKey(string(accountID), "", keyBalance), big.NewInt(0).Bytes())
	i.registers.Set(fullKey(string(accountID), string(accountID), keyPublicKey), publicKey)
	i.registers.Set(fullKey(string(accountID), string(accountID), keyCode), code)

	i.registers.Set(keyLatestAccount, accountID)

	return accountID, nil
}

func (i *EmulatorRuntimeAPI) UpdateAccountCode(accountID, code []byte) (err error) {
	_, exists := i.registers.Get(fullKey(string(accountID), "", keyBalance))
	if !exists {
		return fmt.Errorf("Account with ID %s does not exist", accountID)
	}

	i.registers.Set(fullKey(string(accountID), string(accountID), keyCode), code)

	return nil
}

func (i *EmulatorRuntimeAPI) GetAccount(address types.Address) *types.Account {
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

func (i *EmulatorRuntimeAPI) ResolveImport(location runtime.ImportLocation) ([]byte, error) {
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

func (i *EmulatorRuntimeAPI) GetSigningAccounts() []types.Address {
	return i.Accounts
}

func (i *EmulatorRuntimeAPI) Log(message string) {
	i.Logger(message)
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
