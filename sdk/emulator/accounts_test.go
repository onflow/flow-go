package emulator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/constants"
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/templates"
)

func TestCreateAccount(t *testing.T) {
	var lastEvent types.Event

	b := emulator.NewEmulatedBlockchain(emulator.EmulatedBlockchainOptions{
		OnEventEmitted: func(event types.Event, blockNumber uint64, txHash crypto.Hash) {
			lastEvent = event
		},
	})

	accountKeyA := types.AccountKey{
		PublicKey: []byte{1, 2, 3},
		Weight:    constants.AccountKeyWeightThreshold,
	}

	accountKeyB := types.AccountKey{
		PublicKey: []byte{4, 5, 6},
		Weight:    constants.AccountKeyWeightThreshold,
	}

	codeA := []byte("fun main() {}")

	createAccountScriptA := templates.CreateAccount([]types.AccountKey{accountKeyA, accountKeyB}, codeA)

	tx1 := &types.Transaction{
		Script:             createAccountScriptA,
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	err := b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	assert.Equal(t, constants.EventAccountCreated, lastEvent.ID)
	assert.IsType(t, types.Address{}, lastEvent.Values["address"])

	accountAddress := lastEvent.Values["address"].(types.Address)
	account, err := b.GetAccount(accountAddress)
	assert.Nil(t, err)

	assert.Equal(t, uint64(0), account.Balance)
	assert.Equal(t, accountKeyA, account.Keys[0])
	assert.Equal(t, accountKeyB, account.Keys[1])
	assert.Equal(t, codeA, account.Code)

	accountKeyC := types.AccountKey{
		PublicKey: []byte{7, 8, 9},
		Weight:    constants.AccountKeyWeightThreshold,
	}

	codeB := []byte("fun main() {}")

	createAccountScriptC := templates.CreateAccount([]types.AccountKey{accountKeyC}, codeB)

	tx2 := &types.Transaction{
		Script:             createAccountScriptC,
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
	}

	tx2.AddSignature(b.RootAccountAddress(), b.RootKey())

	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)

	assert.Equal(t, constants.EventAccountCreated, lastEvent.ID)
	assert.IsType(t, types.Address{}, lastEvent.Values["address"])

	accountAddress = lastEvent.Values["address"].(types.Address)
	account, err = b.GetAccount(accountAddress)
	assert.Nil(t, err)

	assert.Equal(t, uint64(0), account.Balance)
	assert.Equal(t, accountKeyC, account.Keys[0])
	assert.Equal(t, codeB, account.Code)
}

func TestAddAccountKey(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	privateKeyA, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("elephant ears"))
	publicKeyA, _ := privateKeyA.Publickey().Encode()

	accountKeyA := types.AccountKey{
		PublicKey: publicKeyA,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	tx1 := &types.Transaction{
		Script:             templates.AddAccountKey(accountKeyA),
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	err := b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	script := []byte("fun main(account: Account) {}")

	tx2 := &types.Transaction{
		Script:             script,
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx2.AddSignature(b.RootAccountAddress(), privateKeyA)

	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)
}

func TestRemoveAccountKey(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	privateKeyA, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("elephant ears"))
	publicKeyA, _ := privateKeyA.Publickey().Encode()

	accountKeyA := types.AccountKey{
		PublicKey: publicKeyA,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	tx1 := &types.Transaction{
		Script:             templates.AddAccountKey(accountKeyA),
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	err := b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	account, err := b.GetAccount(b.RootAccountAddress())
	assert.Nil(t, err)

	assert.Len(t, account.Keys, 2)

	tx2 := &types.Transaction{
		Script:             templates.RemoveAccountKey(0),
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx2.AddSignature(b.RootAccountAddress(), b.RootKey())

	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)

	account, err = b.GetAccount(b.RootAccountAddress())
	assert.Nil(t, err)

	assert.Len(t, account.Keys, 1)

	tx3 := &types.Transaction{
		Script:             templates.RemoveAccountKey(0),
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx3.AddSignature(b.RootAccountAddress(), b.RootKey())

	err = b.SubmitTransaction(tx3)
	assert.NotNil(t, err)

	account, err = b.GetAccount(b.RootAccountAddress())
	assert.Nil(t, err)

	assert.Len(t, account.Keys, 1)

	tx4 := &types.Transaction{
		Script:             templates.RemoveAccountKey(0),
		ReferenceBlockHash: nil,
		Nonce:              2,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx4.AddSignature(b.RootAccountAddress(), privateKeyA)

	err = b.SubmitTransaction(tx4)
	assert.Nil(t, err)

	account, err = b.GetAccount(b.RootAccountAddress())
	assert.Nil(t, err)

	assert.Len(t, account.Keys, 0)
}

func TestUpdateAccountCode(t *testing.T) {
	privateKeyB, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("elephant ears"))
	publicKeyB, _ := privateKeyB.Publickey().Encode()

	accountKeyB := types.AccountKey{
		PublicKey: publicKeyB,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	t.Run("ValidSignature", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		privateKeyA := b.RootKey()

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]types.AccountKey{accountKeyB}, []byte{4, 5, 6})
		assert.Nil(t, err)

		account, err := b.GetAccount(accountAddressB)

		assert.Nil(t, err)
		assert.Equal(t, []byte{4, 5, 6}, account.Code)

		tx := &types.Transaction{
			Script:             templates.UpdateAccountCode([]byte{7, 8, 9}),
			ReferenceBlockHash: nil,
			Nonce:              1,
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []types.Address{accountAddressB},
		}

		tx.AddSignature(accountAddressA, privateKeyA)
		tx.AddSignature(accountAddressB, privateKeyB)

		err = b.SubmitTransaction(tx)
		assert.Nil(t, err)

		account, err = b.GetAccount(accountAddressB)

		assert.Nil(t, err)
		assert.Equal(t, []byte{7, 8, 9}, account.Code)
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		privateKeyA := b.RootKey()

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]types.AccountKey{accountKeyB}, []byte{4, 5, 6})
		assert.Nil(t, err)

		account, err := b.GetAccount(accountAddressB)

		assert.Nil(t, err)
		assert.Equal(t, []byte{4, 5, 6}, account.Code)

		tx := &types.Transaction{
			Script:             templates.UpdateAccountCode([]byte{7, 8, 9}),
			ReferenceBlockHash: nil,
			Nonce:              1,
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []types.Address{accountAddressB},
		}

		tx.AddSignature(accountAddressA, privateKeyA)

		err = b.SubmitTransaction(tx)
		assert.NotNil(t, err)

		account, err = b.GetAccount(accountAddressB)

		// code should not be updated
		assert.Nil(t, err)
		assert.Equal(t, []byte{4, 5, 6}, account.Code)
	})

	t.Run("UnauthorizedAccount", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		privateKeyA := b.RootKey()

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]types.AccountKey{accountKeyB}, []byte{4, 5, 6})
		assert.Nil(t, err)

		account, err := b.GetAccount(accountAddressB)

		assert.Nil(t, err)
		assert.Equal(t, []byte{4, 5, 6}, account.Code)

		unauthorizedUpdateAccountCodeScript := []byte(fmt.Sprintf(`
			fun main(account: Account) {
				let code = [7, 8, 9]
				updateAccountCode(%s, code)
			}
		`, accountAddressB.Hex()))

		tx := &types.Transaction{
			Script:             unauthorizedUpdateAccountCodeScript,
			ReferenceBlockHash: nil,
			Nonce:              1,
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []types.Address{accountAddressA},
		}

		tx.AddSignature(accountAddressA, privateKeyA)

		err = b.SubmitTransaction(tx)
		assert.NotNil(t, err)

		account, err = b.GetAccount(accountAddressB)

		// code should not be updated
		assert.Nil(t, err)
		assert.Equal(t, []byte{4, 5, 6}, account.Code)
	})
}

func TestImportAccountCode(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	accountScript := []byte(`
		fun answer(): Int {
			return 42
		}
	`)

	publicKeyA, _ := b.RootKey().Publickey().Encode()

	accountKeyA := types.AccountKey{
		PublicKey: publicKeyA,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	_, err := b.CreateAccount([]types.AccountKey{accountKeyA}, accountScript)
	assert.Nil(t, err)

	script := []byte(`
		import 0x0000000000000000000000000000000000000002

		fun main(account: Account) {
			let answer = answer()
			if answer != 42 {
				panic("?!")
			}
		}
	`)

	tx2 := &types.Transaction{
		Script:             script,
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx2.AddSignature(b.RootAccountAddress(), b.RootKey())

	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)
}
