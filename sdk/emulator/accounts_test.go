package emulator_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/pkg/constants"
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/templates"
)

func TestCreateAccount(t *testing.T) {
	t.Run("SingleKey", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		accountKey := types.AccountKey{
			PublicKey: []byte{1, 2, 3},
			Weight:    constants.AccountKeyWeightThreshold,
		}

		createAccountScript := templates.CreateAccount([]types.AccountKey{accountKey}, nil)

		tx := &types.Transaction{
			Script:             createAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		tx.AddSignature(b.RootAccountAddress(), b.RootKey())

		err := b.SubmitTransaction(tx)
		assert.Nil(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 1)
		assert.Equal(t, accountKey, account.Keys[0])
		assert.Empty(t, account.Code)
	})

	t.Run("MultipleKeys", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		accountKeyA := types.AccountKey{
			PublicKey: []byte{1, 2, 3},
			Weight:    constants.AccountKeyWeightThreshold,
		}

		accountKeyB := types.AccountKey{
			PublicKey: []byte{4, 5, 6},
			Weight:    constants.AccountKeyWeightThreshold,
		}

		createAccountScript := templates.CreateAccount([]types.AccountKey{accountKeyA, accountKeyB}, nil)

		tx := &types.Transaction{
			Script:             createAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		tx.AddSignature(b.RootAccountAddress(), b.RootKey())

		err := b.SubmitTransaction(tx)
		assert.Nil(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 2)
		assert.Equal(t, accountKeyA, account.Keys[0])
		assert.Equal(t, accountKeyB, account.Keys[1])
		assert.Empty(t, account.Code)
	})

	t.Run("KeysAndCode", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		accountKeyA := types.AccountKey{
			PublicKey: []byte{1, 2, 3},
			Weight:    constants.AccountKeyWeightThreshold,
		}

		accountKeyB := types.AccountKey{
			PublicKey: []byte{4, 5, 6},
			Weight:    constants.AccountKeyWeightThreshold,
		}

		code := []byte("fun main() {}")

		createAccountScript := templates.CreateAccount([]types.AccountKey{accountKeyA, accountKeyB}, code)

		tx := &types.Transaction{
			Script:             createAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		tx.AddSignature(b.RootAccountAddress(), b.RootKey())

		err := b.SubmitTransaction(tx)
		assert.Nil(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 2)
		assert.Equal(t, accountKeyA, account.Keys[0])
		assert.Equal(t, accountKeyB, account.Keys[1])
		assert.Equal(t, code, account.Code)
	})

	t.Run("CodeAndNoKeys", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		code := []byte("fun main() {}")

		createAccountScript := templates.CreateAccount(nil, code)

		tx := &types.Transaction{
			Script:             createAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		tx.AddSignature(b.RootAccountAddress(), b.RootKey())

		err := b.SubmitTransaction(tx)
		assert.Nil(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		assert.Empty(t, account.Keys)
		assert.Equal(t, code, account.Code)
	})

	t.Run("EventEmitted", func(t *testing.T) {
		var lastEvent types.Event

		b := emulator.NewEmulatedBlockchain(emulator.EmulatedBlockchainOptions{
			OnEventEmitted: func(event types.Event, blockNumber uint64, txHash crypto.Hash) {
				lastEvent = event
			},
		})

		accountKey := types.AccountKey{
			PublicKey: []byte{1, 2, 3},
			Weight:    constants.AccountKeyWeightThreshold,
		}

		code := []byte("fun main() {}")

		createAccountScript := templates.CreateAccount([]types.AccountKey{accountKey}, code)

		tx := &types.Transaction{
			Script:             createAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		tx.AddSignature(b.RootAccountAddress(), b.RootKey())

		err := b.SubmitTransaction(tx)
		assert.Nil(t, err)

		require.Equal(t, constants.EventAccountCreated, lastEvent.ID)
		require.IsType(t, types.Address{}, lastEvent.Values["address"])

		accountAddress := lastEvent.Values["address"].(types.Address)
		account, err := b.GetAccount(accountAddress)
		assert.Nil(t, err)

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 1)
		assert.Equal(t, accountKey, account.Keys[0])
		assert.Equal(t, code, account.Code)
	})
}

func TestAddAccountKey(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	privateKey, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("elephant ears"))
	publicKey, _ := privateKey.PublicKey().Encode()

	accountKeyA := types.AccountKey{
		PublicKey: publicKey,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	tx1 := &types.Transaction{
		Script:             templates.AddAccountKey(accountKeyA),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
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
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx2.AddSignature(b.RootAccountAddress(), privateKey)

	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)
}

func TestRemoveAccountKey(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	privateKey, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("elephant ears"))
	publicKey, _ := privateKey.PublicKey().Encode()

	accountKey := types.AccountKey{
		PublicKey: publicKey,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	tx1 := &types.Transaction{
		Script:             templates.AddAccountKey(accountKey),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
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
		Nonce:              getNonce(),
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
		Nonce:              getNonce(),
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
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx4.AddSignature(b.RootAccountAddress(), privateKey)

	err = b.SubmitTransaction(tx4)
	assert.Nil(t, err)

	account, err = b.GetAccount(b.RootAccountAddress())
	assert.Nil(t, err)

	assert.Empty(t, account.Keys)
}

func TestUpdateAccountCode(t *testing.T) {
	privateKeyB, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("elephant ears"))
	publicKeyB, _ := privateKeyB.PublicKey().Encode()

	accountKeyB := types.AccountKey{
		PublicKey: publicKeyB,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	t.Run("ValidSignature", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		privateKeyA := b.RootKey()

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]types.AccountKey{accountKeyB}, []byte{4, 5, 6}, getNonce())
		assert.Nil(t, err)

		account, err := b.GetAccount(accountAddressB)

		assert.Nil(t, err)
		assert.Equal(t, []byte{4, 5, 6}, account.Code)

		tx := &types.Transaction{
			Script:             templates.UpdateAccountCode([]byte{7, 8, 9}),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
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
		accountAddressB, err := b.CreateAccount([]types.AccountKey{accountKeyB}, []byte{4, 5, 6}, getNonce())
		assert.Nil(t, err)

		account, err := b.GetAccount(accountAddressB)

		assert.Nil(t, err)
		assert.Equal(t, []byte{4, 5, 6}, account.Code)

		tx := &types.Transaction{
			Script:             templates.UpdateAccountCode([]byte{7, 8, 9}),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
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
		accountAddressB, err := b.CreateAccount([]types.AccountKey{accountKeyB}, []byte{4, 5, 6}, getNonce())
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
			Nonce:              getNonce(),
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

	publicKey, _ := b.RootKey().PublicKey().Encode()

	accountKey := types.AccountKey{
		PublicKey: publicKey,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	address, err := b.CreateAccount([]types.AccountKey{accountKey}, accountScript, getNonce())
	assert.Nil(t, err)

	script := []byte(fmt.Sprintf(`
		import 0x%s

		fun main(account: Account) {
			let answer = answer()
			if answer != 42 {
				panic("?!")
			}
		}
	`, address.Hex()))

	tx := &types.Transaction{
		Script:             script,
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx.AddSignature(b.RootAccountAddress(), b.RootKey())

	err = b.SubmitTransaction(tx)
	assert.Nil(t, err)
}
