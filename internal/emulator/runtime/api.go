package runtime

import (
	"math/big"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"

	etypes "github.com/dapperlabs/bamboo-node/internal/emulator/types"
)

type EmulatorRuntimeAPI struct {
	registers *etypes.RegistersView
}

func NewEmulatorRuntimeAPI(registers *etypes.RegistersView) *EmulatorRuntimeAPI {
	return &EmulatorRuntimeAPI{registers}
}

func (i *EmulatorRuntimeAPI) GetValue(controller, owner, key []byte) ([]byte, error) {
	v, _ := i.registers.Get(fullKey(controller, owner, key))
	return v, nil
}

func (i *EmulatorRuntimeAPI) SetValue(controller, owner, key, value []byte) error {
	i.registers.Set(fullKey(controller, owner, key), value)
	return nil
}

func (i *EmulatorRuntimeAPI) CreateAccount(publicKey, code []byte) (id []byte, err error) {
	latestAccountID, _ := i.registers.Get(keyLatestAccount())

	accountIDInt := big.NewInt(0).SetBytes(latestAccountID)
	accountIDBytes := accountIDInt.Add(accountIDInt, big.NewInt(1)).Bytes()

	accountAddress := crypto.BytesToAddress(accountIDBytes)

	accountID := accountAddress.Bytes()

	// TODO: determine better key for account balance
	i.registers.Set(fullKey(accountID, accountID, []byte("balance")), big.NewInt(0).Bytes())
	i.registers.Set(fullKey(accountID, accountID, []byte("public_key")), publicKey)
	i.registers.Set(fullKey(accountID, accountID, []byte("code")), code)

	i.registers.Set(keyLatestAccount(), accountID)

	address := crypto.BytesToAddress(accountID)

	return address.Bytes(), nil
}

func (i *EmulatorRuntimeAPI) GetAccount(address crypto.Address) *crypto.Account {
	accountID := address.Bytes()

	balanceBytes, exists := i.registers.Get(fullKey(accountID, accountID, []byte("balance")))
	if !exists {
		return nil
	}

	balanceInt := big.NewInt(0).SetBytes(balanceBytes)

	publicKey, _ := i.registers.Get(fullKey(accountID, accountID, []byte("public_key")))
	code, _ := i.registers.Get(fullKey(accountID, accountID, []byte("code")))

	return &crypto.Account{
		Address:    address,
		Balance:    balanceInt.Uint64(),
		Code:       code,
		PublicKeys: [][]byte{publicKey},
	}
}

func keyLatestAccount() crypto.Hash {
	return crypto.NewHash([]byte("latestAccount"))
}

func fullKey(controller, owner, key []byte) crypto.Hash {
	fullKey := append(controller, owner...)
	fullKey = append(fullKey, key...)

	return crypto.NewHash(fullKey)
}
