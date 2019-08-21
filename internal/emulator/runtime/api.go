package runtime

import (
	"errors"
	"math/big"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	etypes "github.com/dapperlabs/bamboo-node/internal/emulator/types"
)

type EmulatorRuntimeAPI struct {
	registers *etypes.RegistersView
	Accounts  []types.Address
}

func NewEmulatorRuntimeAPI(registers *etypes.RegistersView) *EmulatorRuntimeAPI {
	return &EmulatorRuntimeAPI{registers: registers}
}

func (i *EmulatorRuntimeAPI) GetValue(owner, controller, key []byte) ([]byte, error) {
	v, _ := i.registers.Get(fullKey(owner, controller, key))
	return v, nil
}

func (i *EmulatorRuntimeAPI) SetValue(owner, controller, key, value []byte) error {
	i.registers.Set(fullKey(owner, controller, key), value)
	return nil
}

func (i *EmulatorRuntimeAPI) CreateAccount(publicKey, code []byte) (id []byte, err error) {
	latestAccountID, _ := i.registers.Get(keyLatestAccount())

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := types.BytesToAddress(accountIDBytes)

	accountID := accountAddress.Bytes()

	i.registers.Set(fullKey(accountID, []byte{}, keyBalance()), big.NewInt(0).Bytes())
	i.registers.Set(fullKey(accountID, accountID, keyPublicKey()), publicKey)
	i.registers.Set(fullKey(accountID, accountID, keyCode()), code)

	i.registers.Set(keyLatestAccount(), accountID)

	address := types.BytesToAddress(accountID)

	return address.Bytes(), nil
}

func (i *EmulatorRuntimeAPI) GetAccount(address types.Address) *types.Account {
	accountID := address.Bytes()

	balanceBytes, exists := i.registers.Get(fullKey(accountID, []byte{}, keyBalance()))
	if !exists {
		return nil
	}

	balanceInt := big.NewInt(0).SetBytes(balanceBytes)

	publicKey, _ := i.registers.Get(fullKey(accountID, accountID, keyPublicKey()))
	code, _ := i.registers.Get(fullKey(accountID, accountID, keyCode()))

	return &types.Account{
		Address:    address,
		Balance:    balanceInt.Uint64(),
		Code:       code,
		PublicKeys: [][]byte{publicKey},
	}
}

func (i *EmulatorRuntimeAPI) ResolveImport(location runtime.ImportLocation) ([]byte, error) {
	// TODO:
	return nil, errors.New("not supported")
}
func (i *EmulatorRuntimeAPI) GetSigningAccounts() []types.Address {
	return i.Accounts
}

func (i *EmulatorRuntimeAPI) Log(message string) {
	// TODO:
}

func keyLatestAccount() crypto.Hash {
	return crypto.NewHash([]byte("latest_account"))
}

func keyBalance() []byte {
	return []byte("balance")
}

func keyPublicKey() []byte {
	return []byte("public_key")
}

func keyCode() []byte {
	return []byte("code")
}

func fullKey(owner, controller, key []byte) crypto.Hash {
	fullKey := append(owner, controller...)
	fullKey = append(fullKey, key...)
	return crypto.NewHash(fullKey)
}
