package emulator_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/constants"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

func TestSubmitTransaction(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	tx1 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	sig, _ := keys.SignTransaction(tx1, b.RootKey())

	tx1.AddSignature(b.RootAccountAddress(), sig)

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	// tx1 status becomes TransactionFinalized
	tx, err := b.GetTransaction(tx1.Hash())
	assert.Nil(t, err)
	assert.Equal(t, types.TransactionFinalized, tx.Status)
}

// TODO: Add test case for missing ReferenceBlockHash
func TestSubmitInvalidTransaction(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	t.Run("EmptyTransaction", func(t *testing.T) {
		// Create empty transaction (no required fields)
		tx1 := &types.Transaction{}

		tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

		// Submit tx1
		err := b.SubmitTransaction(tx1)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})

	t.Run("MissingScript", func(t *testing.T) {
		// Create transaction with no Script field
		tx1 := &types.Transaction{
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

		// Submit tx1
		err := b.SubmitTransaction(tx1)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})

	t.Run("MissingNonce", func(t *testing.T) {
		// Create transaction with no Nonce field
		tx1 := &types.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

		// Submit tx1
		err := b.SubmitTransaction(tx1)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})

	t.Run("MissingComputeLimit", func(t *testing.T) {
		// Create transaction with no ComputeLimit field
		tx1 := &types.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			PayerAccount:       b.RootAccountAddress(),
		}

		tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

		// Submit tx1
		err := b.SubmitTransaction(tx1)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})

	t.Run("MissingPayerAccount", func(t *testing.T) {
		// Create transaction with no PayerAccount field
		tx1 := &types.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
		}

		tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

		// Submit tx1
		err := b.SubmitTransaction(tx1)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})
}

func TestSubmitDuplicateTransaction(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	tx1 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	// Submit tx1 again (errors)
	err = b.SubmitTransaction(tx1)
	assert.IsType(t, err, &emulator.ErrDuplicateTransaction{})
}

func TestSubmitTransactionReverted(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	tx1 := &types.Transaction{
		Script:             []byte("invalid script"),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	// Submit invalid tx1 (errors)
	err := b.SubmitTransaction(tx1)
	assert.NotNil(t, err)

	// tx1 status becomes TransactionReverted
	tx, err := b.GetTransaction(tx1.Hash())
	assert.Nil(t, err)
	assert.Equal(t, types.TransactionReverted, tx.Status)
}

func TestSubmitTransactionScriptAccounts(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	privateKeyA := b.RootKey()

	privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, []byte("elephant ears"))
	publicKeyB := privateKeyB.PublicKey(constants.AccountKeyWeightThreshold)

	accountAddressA := b.RootAccountAddress()
	accountAddressB, err := b.CreateAccount([]types.AccountPublicKey{publicKeyB}, nil, getNonce())
	assert.Nil(t, err)

	t.Run("TooManyAccountsForScript", func(t *testing.T) {
		// script only supports one account
		script := []byte("fun main(account: Account) {}")

		// create transaction with two accounts
		tx1 := &types.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []types.Address{accountAddressA, accountAddressB},
		}

		tx1.AddSignature(accountAddressA, privateKeyA)
		tx1.AddSignature(accountAddressB, privateKeyB)

		err := b.SubmitTransaction(tx1)
		assert.NotNil(t, err)
	})

	t.Run("NotEnoughAccountsForScript", func(t *testing.T) {
		// script requires two accounts
		script := []byte("fun main(accountA: Account, accountB: Account) {}")

		// create transaction with two accounts
		tx1 := &types.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []types.Address{accountAddressA},
		}

		tx1.AddSignature(accountAddressA, privateKeyA)

		err := b.SubmitTransaction(tx1)
		assert.NotNil(t, err)
	})
}

func TestSubmitTransactionPayerSignature(t *testing.T) {
	t.Run("MissingPayerSignature", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		addressA := types.HexToAddress("0000000000000000000000000000000000000002")

		tx1 := &types.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       addressA,
			ScriptAccounts:     []types.Address{b.RootAccountAddress()},
		}

		tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

		err := b.SubmitTransaction(tx1)

		assert.IsType(t, err, &emulator.ErrMissingSignature{})
	})

	t.Run("InvalidAccount", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		invalidAddress := types.HexToAddress("0000000000000000000000000000000000000002")

		tx1 := &types.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []types.Address{b.RootAccountAddress()},
		}

		tx1.AddSignature(invalidAddress, b.RootKey())

		err := b.SubmitTransaction(tx1)

		assert.IsType(t, err, &emulator.ErrInvalidSignatureAccount{})
	})

	t.Run("InvalidKeyPair", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		// use key-pair that does not exist on root account
		invalidKey, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, []byte("invalid key"))

		tx1 := &types.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []types.Address{b.RootAccountAddress()},
		}

		tx1.AddSignature(b.RootAccountAddress(), invalidKey)

		err := b.SubmitTransaction(tx1)
		assert.IsType(t, err, &emulator.ErrInvalidSignaturePublicKey{})
	})

	t.Run("KeyWeights", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		privateKeyA, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, []byte("elephant ears"))
		publicKeyA := privateKeyA.PublicKey(constants.AccountKeyWeightThreshold / 2)

		privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, []byte("space cowboy"))
		publicKeyB := privateKeyB.PublicKey(constants.AccountKeyWeightThreshold / 2)

		accountAddressA, err := b.CreateAccount([]types.AccountPublicKey{publicKeyA, publicKeyB}, nil, getNonce())

		script := []byte("fun main(account: Account) {}")

		tx := &types.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []types.Address{accountAddressA},
		}

		t.Run("InsufficientKeyWeight", func(t *testing.T) {
			tx.AddSignature(accountAddressA, privateKeyB)

			err = b.SubmitTransaction(tx)
			assert.IsType(t, err, &emulator.ErrMissingSignature{})
		})

		t.Run("SufficientKeyWeight", func(t *testing.T) {
			tx.AddSignature(accountAddressA, privateKeyA)
			tx.AddSignature(accountAddressA, privateKeyB)

			err = b.SubmitTransaction(tx)
			assert.Nil(t, err)
		})
	})
}

func TestSubmitTransactionScriptSignatures(t *testing.T) {
	t.Run("MissingScriptSignature", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		addressA := types.HexToAddress("0000000000000000000000000000000000000002")

		tx1 := &types.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []types.Address{addressA},
		}

		tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

		err := b.SubmitTransaction(tx1)
		assert.NotNil(t, err)
		assert.IsType(t, err, &emulator.ErrMissingSignature{})
	})

	t.Run("MultipleAccounts", func(t *testing.T) {
		loggedMessages := make([]string, 0)

		b := emulator.NewEmulatedBlockchain(emulator.EmulatedBlockchainOptions{
			OnLogMessage: func(msg string) {
				loggedMessages = append(loggedMessages, msg)
			},
		})

		privateKeyA := b.RootKey()

		privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, []byte("elephant ears"))
		publicKeyB := privateKeyB.PublicKey(constants.AccountKeyWeightThreshold)

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]types.AccountPublicKey{publicKeyB}, nil, getNonce())
		assert.Nil(t, err)

		multipleAccountScript := []byte(`
			fun main(accountA: Account, accountB: Account) {
				log(accountA.address)
				log(accountB.address)
			}
		`)

		tx2 := &types.Transaction{
			Script:             multipleAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []types.Address{accountAddressA, accountAddressB},
		}

		tx2.AddSignature(accountAddressA, privateKeyA)
		tx2.AddSignature(accountAddressB, privateKeyB)

		err = b.SubmitTransaction(tx2)
		assert.Nil(t, err)

		assert.Contains(t, loggedMessages, fmt.Sprintf(`"%x"`, accountAddressA.Bytes()))
		assert.Contains(t, loggedMessages, fmt.Sprintf(`"%x"`, accountAddressB.Bytes()))
	})
}
