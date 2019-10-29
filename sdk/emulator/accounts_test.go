package emulator_test

import (
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/constants"
	"github.com/dapperlabs/flow-go/sdk/keys"
	"github.com/dapperlabs/flow-go/sdk/templates"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestCreateAccount(t *testing.T) {
	publicKeys := unittest.PublicKeyFixtures()

	t.Run("SingleKey", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		publicKey := flow.AccountPublicKey{
			PublicKey: publicKeys[0],
			SignAlgo:  crypto.ECDSA_P256,
			HashAlgo:  crypto.SHA3_256,
			Weight:    constants.AccountKeyWeightThreshold,
		}

		createAccountScript, err := templates.CreateAccount([]flow.AccountPublicKey{publicKey}, nil)
		require.Nil(t, err)

		tx := flow.Transaction{
			Script:             createAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)
		assert.Nil(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 1)
		assert.Equal(t, publicKey, account.Keys[0])
		assert.Empty(t, account.Code)
	})

	t.Run("MultipleKeys", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		publicKeyA := flow.AccountPublicKey{
			PublicKey: publicKeys[0],
			SignAlgo:  crypto.ECDSA_P256,
			HashAlgo:  crypto.SHA3_256,
			Weight:    constants.AccountKeyWeightThreshold,
		}

		publicKeyB := flow.AccountPublicKey{
			PublicKey: publicKeys[1],
			SignAlgo:  crypto.ECDSA_P256,
			HashAlgo:  crypto.SHA3_256,
			Weight:    constants.AccountKeyWeightThreshold,
		}

		createAccountScript, err := templates.CreateAccount([]flow.AccountPublicKey{publicKeyA, publicKeyB}, nil)
		assert.Nil(t, err)

		tx := flow.Transaction{
			Script:             createAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)
		assert.Nil(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 2)
		assert.Equal(t, publicKeyA, account.Keys[0])
		assert.Equal(t, publicKeyB, account.Keys[1])
		assert.Empty(t, account.Code)
	})

	t.Run("KeysAndCode", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		publicKeyA := flow.AccountPublicKey{
			PublicKey: publicKeys[0],
			SignAlgo:  crypto.ECDSA_P256,
			HashAlgo:  crypto.SHA3_256,
			Weight:    constants.AccountKeyWeightThreshold,
		}

		publicKeyB := flow.AccountPublicKey{
			PublicKey: publicKeys[1],
			SignAlgo:  crypto.ECDSA_P256,
			HashAlgo:  crypto.SHA3_256,
			Weight:    constants.AccountKeyWeightThreshold,
		}

		code := []byte("fun main() {}")

		createAccountScript, err := templates.CreateAccount([]flow.AccountPublicKey{publicKeyA, publicKeyB}, code)
		assert.Nil(t, err)

		tx := flow.Transaction{
			Script:             createAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)
		assert.Nil(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 2)
		assert.Equal(t, publicKeyA, account.Keys[0])
		assert.Equal(t, publicKeyB, account.Keys[1])
		assert.Equal(t, code, account.Code)
	})

	t.Run("CodeAndNoKeys", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		code := []byte("fun main() {}")

		createAccountScript, err := templates.CreateAccount(nil, code)
		assert.Nil(t, err)

		tx := flow.Transaction{
			Script:             createAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)
		assert.Nil(t, err)

		account := b.LastCreatedAccount()

		assert.Equal(t, uint64(0), account.Balance)
		assert.Empty(t, account.Keys)
		assert.Equal(t, code, account.Code)
	})

	t.Run("EventEmitted", func(t *testing.T) {
		var lastEvent flow.Event

		b := emulator.NewEmulatedBlockchain(emulator.EmulatedBlockchainOptions{
			OnEventEmitted: func(event flow.Event, blockNumber uint64, txHash crypto.Hash) {
				lastEvent = event
			},
		})

		publicKey := flow.AccountPublicKey{
			PublicKey: publicKeys[0],
			SignAlgo:  crypto.ECDSA_P256,
			HashAlgo:  crypto.SHA3_256,
			Weight:    constants.AccountKeyWeightThreshold,
		}

		code := []byte("fun main() {}")

		createAccountScript, err := templates.CreateAccount([]flow.AccountPublicKey{publicKey}, code)
		assert.Nil(t, err)

		tx := flow.Transaction{
			Script:             createAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)
		assert.Nil(t, err)

		require.Equal(t, constants.EventAccountCreated, lastEvent.ID)
		require.IsType(t, flow.Address{}, lastEvent.Values["address"])

		accountAddress := lastEvent.Values["address"].(flow.Address)
		account, err := b.GetAccount(accountAddress)
		assert.Nil(t, err)

		assert.Equal(t, uint64(0), account.Balance)
		require.Len(t, account.Keys, 1)
		assert.Equal(t, publicKey, account.Keys[0])
		assert.Equal(t, code, account.Code)
	})

	t.Run("InvalidKeyHashingAlgorithm", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		lastAccount := b.LastCreatedAccount()

		publicKey := flow.AccountPublicKey{
			PublicKey: unittest.PublicKeyFixtures()[0],
			SignAlgo:  crypto.ECDSA_P256,
			// SHA2_384 is not compatible with ECDSA_P256
			HashAlgo: crypto.SHA2_384,
			Weight:   constants.AccountKeyWeightThreshold,
		}

		createAccountScript, err := templates.CreateAccount([]flow.AccountPublicKey{publicKey}, nil)
		require.Nil(t, err)

		tx := &flow.Transaction{
			Script:             createAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		tx.AddSignature(b.RootAccountAddress(), b.RootKey())

		err = b.SubmitTransaction(tx)
		assert.IsType(t, &emulator.ErrTransactionReverted{}, err)

		newAccount := b.LastCreatedAccount()

		assert.Equal(t, lastAccount, newAccount)
	})
}

func TestAddAccountKey(t *testing.T) {
	t.Run("ValidKey", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		privateKey, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, []byte("elephant ears"))
		publicKey := privateKey.PublicKey(constants.AccountKeyWeightThreshold)

		addKeyScript, err := templates.AddAccountKey(publicKey)
		assert.Nil(t, err)

		tx1 := flow.Transaction{
			Script:             addKeyScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
		}

		sig, err := keys.SignTransaction(tx1, b.RootKey())
		assert.Nil(t, err)

		tx1.AddSignature(b.RootAccountAddress(), sig)
		err = b.SubmitTransaction(tx1)
		assert.Nil(t, err)

		script := []byte("fun main(account: Account) {}")

		tx2 := flow.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
		}

		sig, err = keys.SignTransaction(tx2, privateKey)
		assert.Nil(t, err)

		tx2.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx2)
		assert.Nil(t, err)
	})

	t.Run("InvalidKeyHashingAlgorithm", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		publicKey := flow.AccountPublicKey{
			PublicKey: unittest.PublicKeyFixtures()[0],
			SignAlgo:  crypto.ECDSA_P256,
			// SHA2_384 is not compatible with ECDSA_P256
			HashAlgo: crypto.SHA2_384,
			Weight:   constants.AccountKeyWeightThreshold,
		}

		addKeyScript, err := templates.AddAccountKey(publicKey)
		assert.Nil(t, err)

		tx := &flow.Transaction{
			Script:             addKeyScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
		}

		tx.AddSignature(b.RootAccountAddress(), b.RootKey())

		err = b.SubmitTransaction(tx)
		assert.IsType(t, &emulator.ErrTransactionReverted{}, err)
	})
}

func TestRemoveAccountKey(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	privateKey, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, []byte("elephant ears"))
	publicKey := privateKey.PublicKey(constants.AccountKeyWeightThreshold)

	addKeyScript, err := templates.AddAccountKey(publicKey)
	assert.Nil(t, err)

	tx1 := flow.Transaction{
		Script:             addKeyScript,
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err := keys.SignTransaction(tx1, b.RootKey())
	assert.Nil(t, err)

	tx1.AddSignature(b.RootAccountAddress(), sig)

	err = b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	account, err := b.GetAccount(b.RootAccountAddress())
	assert.Nil(t, err)

	assert.Len(t, account.Keys, 2)

	tx2 := &flow.Transaction{
		Script:             templates.RemoveAccountKey(0),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err = keys.SignTransaction(tx2, b.RootKey())
	assert.Nil(t, err)

	tx2.AddSignature(b.RootAccountAddress(), sig)

	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)

	account, err = b.GetAccount(b.RootAccountAddress())
	assert.Nil(t, err)

	assert.Len(t, account.Keys, 1)

	tx3 := flow.Transaction{
		Script:             templates.RemoveAccountKey(0),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err = keys.SignTransaction(tx3, b.RootKey())
	assert.Nil(t, err)

	tx3.AddSignature(b.RootAccountAddress(), sig)

	err = b.SubmitTransaction(tx3)
	assert.NotNil(t, err)

	account, err = b.GetAccount(b.RootAccountAddress())
	assert.Nil(t, err)

	assert.Len(t, account.Keys, 1)

	tx4 := flow.Transaction{
		Script:             templates.RemoveAccountKey(0),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err = keys.SignTransaction(tx4, privateKey)
	assert.Nil(t, err)

	tx4.AddSignature(b.RootAccountAddress(), sig)

	err = b.SubmitTransaction(tx4)
	assert.Nil(t, err)

	account, err = b.GetAccount(b.RootAccountAddress())
	assert.Nil(t, err)

	assert.Empty(t, account.Keys)
}

func TestUpdateAccountCode(t *testing.T) {
	privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, []byte("elephant ears"))
	publicKeyB := privateKeyB.PublicKey(constants.AccountKeyWeightThreshold)

	t.Run("ValidSignature", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		privateKeyA := b.RootKey()

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]flow.AccountPublicKey{publicKeyB}, []byte{4, 5, 6}, getNonce())
		assert.Nil(t, err)

		account, err := b.GetAccount(accountAddressB)

		assert.Nil(t, err)
		assert.Equal(t, []byte{4, 5, 6}, account.Code)

		tx := flow.Transaction{
			Script:             templates.UpdateAccountCode([]byte{7, 8, 9}),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []flow.Address{accountAddressB},
		}

		sigA, err := keys.SignTransaction(tx, privateKeyA)
		assert.Nil(t, err)

		sigB, err := keys.SignTransaction(tx, privateKeyB)
		assert.Nil(t, err)

		tx.AddSignature(accountAddressA, sigA)
		tx.AddSignature(accountAddressB, sigB)

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
		accountAddressB, err := b.CreateAccount([]flow.AccountPublicKey{publicKeyB}, []byte{4, 5, 6}, getNonce())
		assert.Nil(t, err)

		account, err := b.GetAccount(accountAddressB)

		assert.Nil(t, err)
		assert.Equal(t, []byte{4, 5, 6}, account.Code)

		tx := flow.Transaction{
			Script:             templates.UpdateAccountCode([]byte{7, 8, 9}),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []flow.Address{accountAddressB},
		}

		sig, err := keys.SignTransaction(tx, privateKeyA)
		assert.Nil(t, err)

		tx.AddSignature(accountAddressA, sig)

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
		accountAddressB, err := b.CreateAccount([]flow.AccountPublicKey{publicKeyB}, []byte{4, 5, 6}, getNonce())
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

		tx := flow.Transaction{
			Script:             unauthorizedUpdateAccountCodeScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []flow.Address{accountAddressA},
		}

		sig, err := keys.SignTransaction(tx, privateKeyA)
		assert.Nil(t, err)

		tx.AddSignature(accountAddressA, sig)

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

	publicKey := b.RootKey().PublicKey(constants.AccountKeyWeightThreshold)

	address, err := b.CreateAccount([]flow.AccountPublicKey{publicKey}, accountScript, getNonce())
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

	tx := flow.Transaction{
		Script:             script,
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err := keys.SignTransaction(tx, b.RootKey())
	assert.Nil(t, err)

	tx.AddSignature(b.RootAccountAddress(), sig)

	err = b.SubmitTransaction(tx)
	assert.Nil(t, err)
}
