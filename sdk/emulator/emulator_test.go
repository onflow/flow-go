package emulator

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/pkg/constants"
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/templates"
)

// addTwoScript runs a script that adds 2 to a value.
const addTwoScript = `
	fun main(account: Account) {
		let controller = [1]
		let owner = [2]
		let key = [3]
		let value = getValue(controller, owner, key)
		setValue(controller, owner, key, value + 2)
	}
`

const sampleCall = `
	fun main(): Int {
		return getValue([1], [2], [3])
	}
`

func TestWorldStates(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

	// Create 3 signed transactions (tx1, tx2, tx3)
	tx1 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	tx2 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx2.AddSignature(b.RootAccountAddress(), b.RootKey())

	tx3 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              3,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx3.AddSignature(b.RootAccountAddress(), b.RootKey())

	ws1 := b.pendingWorldState.Hash()
	t.Logf("initial world state: %x\n", ws1)

	// Tx pool contains nothing
	assert.Len(t, b.txPool, 0)

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	ws2 := b.pendingWorldState.Hash()
	t.Logf("world state after tx1: %x\n", ws2)

	// tx1 included in tx pool
	assert.Len(t, b.txPool, 1)
	// World state updates
	assert.NotEqual(t, ws1, ws2)

	// Submit tx1 again
	err = b.SubmitTransaction(tx1)
	assert.NotNil(t, err)

	ws3 := b.pendingWorldState.Hash()
	t.Logf("world state after dup tx1: %x\n", ws3)

	// tx1 not included in tx pool
	assert.Len(t, b.txPool, 1)
	// World state does not update
	assert.Equal(t, ws2, ws3)

	// Submit tx2
	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)

	ws4 := b.pendingWorldState.Hash()
	t.Logf("world state after tx2: %x\n", ws4)

	// tx2 included in tx pool
	assert.Len(t, b.txPool, 2)
	// World state updates
	assert.NotEqual(t, ws3, ws4)

	// Commit new block
	b.CommitBlock()
	ws5 := b.pendingWorldState.Hash()
	t.Logf("world state after commit: %x\n", ws5)

	// Tx pool cleared
	assert.Len(t, b.txPool, 0)
	// World state updates
	assert.NotEqual(t, ws4, ws5)
	// World state is indexed
	assert.Contains(t, b.worldStates, string(ws5))

	// Submit tx3
	err = b.SubmitTransaction(tx3)
	assert.Nil(t, err)

	ws6 := b.pendingWorldState.Hash()
	t.Logf("world state after tx3: %x\n", ws6)

	// tx3 included in tx pool
	assert.Len(t, b.txPool, 1)
	// World state updates
	assert.NotEqual(t, ws5, ws6)

	// Seek to committed block/world state
	b.SeekToState(ws5)
	ws7 := b.pendingWorldState.Hash()
	t.Logf("world state after seek: %x\n", ws7)

	// Tx pool cleared
	assert.Len(t, b.txPool, 0)
	// World state rollback to ws5 (before tx3)
	assert.Equal(t, ws5, ws7)
	// World state does not include tx3
	assert.False(t, b.pendingWorldState.ContainsTransaction(tx3.Hash()))

	// Seek to non-committed world state
	b.SeekToState(ws4)
	ws8 := b.pendingWorldState.Hash()
	t.Logf("world state after failed seek: %x\n", ws8)

	// World state does not rollback to ws4 (before commit block)
	assert.NotEqual(t, ws4, ws8)
}

func TestSubmitTransaction(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

	tx1 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	// tx1 status becomes TransactionFinalized
	tx, err := b.GetTransaction(tx1.Hash())
	assert.Nil(t, err)
	assert.Equal(t, types.TransactionFinalized, tx.Status)
}

func TestSubmitDuplicateTransaction(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

	tx1 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
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
	assert.IsType(t, err, &ErrDuplicateTransaction{})
}

func TestSubmitTransactionScriptAccounts(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

	privateKeyA := b.RootKey()

	privateKeyB, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("elephant ears"))
	publicKeyB, _ := privateKeyB.Publickey().Encode()

	accountKeyB := types.AccountKey{
		PublicKey: publicKeyB,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	accountAddressA := b.RootAccountAddress()
	accountAddressB, err := b.CreateAccount([]types.AccountKey{accountKeyB}, nil)
	assert.Nil(t, err)

	t.Run("TooManyAccountsForScript", func(t *testing.T) {
		// script only supports one account
		script := []byte("fun main(account: Account) {}")

		// create transaction with two accounts
		tx1 := &types.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
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
		b := NewEmulatedBlockchain(DefaultOptions)

		addressA := types.HexToAddress("0000000000000000000000000000000000000002")

		tx1 := &types.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			ComputeLimit:       10,
			PayerAccount:       addressA,
			ScriptAccounts:     []types.Address{b.RootAccountAddress()},
		}

		tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

		err := b.SubmitTransaction(tx1)

		assert.IsType(t, err, &ErrMissingSignature{})
	})

	t.Run("InvalidAccount", func(t *testing.T) {
		b := NewEmulatedBlockchain(DefaultOptions)

		invalidAddress := types.HexToAddress("0000000000000000000000000000000000000002")

		tx1 := &types.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []types.Address{b.RootAccountAddress()},
		}

		tx1.AddSignature(invalidAddress, b.RootKey())

		err := b.SubmitTransaction(tx1)

		assert.IsType(t, err, &ErrInvalidSignatureAccount{})
	})

	t.Run("InvalidKeyPair", func(t *testing.T) {
		b := NewEmulatedBlockchain(DefaultOptions)

		// use key-pair that does not exist on root account
		invalidKey, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("invalid key"))

		tx1 := &types.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []types.Address{b.RootAccountAddress()},
		}

		tx1.AddSignature(b.RootAccountAddress(), invalidKey)

		err := b.SubmitTransaction(tx1)
		assert.IsType(t, err, &ErrInvalidSignaturePublicKey{})
	})

	t.Run("KeyWeights", func(t *testing.T) {
		b := NewEmulatedBlockchain(DefaultOptions)

		privateKeyA, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("elephant ears"))
		publicKeyA, _ := privateKeyA.Publickey().Encode()

		privateKeyB, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("space cowboy"))
		publicKeyB, _ := privateKeyB.Publickey().Encode()

		accountKeyA := types.AccountKey{
			PublicKey: publicKeyA,
			Weight:    constants.AccountKeyWeightThreshold / 2,
		}

		accountKeyB := types.AccountKey{
			PublicKey: publicKeyB,
			Weight:    constants.AccountKeyWeightThreshold / 2,
		}

		accountAddressA, err := b.CreateAccount([]types.AccountKey{accountKeyA, accountKeyB}, nil)

		script := []byte("fun main(account: Account) {}")

		tx := &types.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []types.Address{accountAddressA},
		}

		t.Run("InsufficientKeyWeight", func(t *testing.T) {
			tx.AddSignature(accountAddressA, privateKeyB)

			err = b.SubmitTransaction(tx)
			assert.IsType(t, err, &ErrMissingSignature{})
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
		b := NewEmulatedBlockchain(DefaultOptions)

		addressA := types.HexToAddress("0000000000000000000000000000000000000002")

		tx1 := &types.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []types.Address{addressA},
		}

		tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

		err := b.SubmitTransaction(tx1)
		assert.NotNil(t, err)
		assert.IsType(t, err, &ErrMissingSignature{})
	})

	t.Run("MultipleAccounts", func(t *testing.T) {
		loggedMessages := make([]string, 0)

		b := NewEmulatedBlockchain(EmulatedBlockchainOptions{
			OnLogMessage: func(msg string) {
				loggedMessages = append(loggedMessages, msg)
			},
		})

		privateKeyA := b.RootKey()

		privateKeyB, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("elephant ears"))
		publicKeyB, _ := privateKeyB.Publickey().Encode()

		accountKeyB := types.AccountKey{
			PublicKey: publicKeyB,
			Weight:    constants.AccountKeyWeightThreshold,
		}

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]types.AccountKey{accountKeyB}, nil)
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

func TestSubmitTransactionReverted(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

	tx1 := &types.Transaction{
		Script:             []byte("invalid script"),
		ReferenceBlockHash: nil,
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

func TestCommitBlock(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

	tx1 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	tx, _ := b.GetTransaction(tx1.Hash())
	assert.Nil(t, err)
	assert.Equal(t, types.TransactionFinalized, tx.Status)

	tx2 := &types.Transaction{
		Script:             []byte("invalid script"),
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx2.AddSignature(b.RootAccountAddress(), b.RootKey())

	// Submit invalid tx2
	err = b.SubmitTransaction(tx2)
	assert.NotNil(t, err)

	tx, err = b.GetTransaction(tx2.Hash())
	assert.Nil(t, err)

	assert.Equal(t, types.TransactionReverted, tx.Status)

	// Commit tx1 and tx2 into new block
	b.CommitBlock()

	// tx1 status becomes TransactionSealed
	tx, _ = b.GetTransaction(tx1.Hash())
	assert.Equal(t, types.TransactionSealed, tx.Status)
	// tx2 status stays TransactionReverted
	tx, _ = b.GetTransaction(tx2.Hash())
	assert.Equal(t, types.TransactionReverted, tx.Status)
}

func TestCreateAccount(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

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

	account, err := b.GetAccount(b.LastCreatedAccount().Address)
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

	account, err = b.GetAccount(b.LastCreatedAccount().Address)

	assert.Nil(t, err)
	assert.Equal(t, uint64(0), account.Balance)
	assert.Equal(t, accountKeyC, account.Keys[0])
	assert.Equal(t, codeB, account.Code)
}

func TestAddAccountKey(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

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
	b := NewEmulatedBlockchain(DefaultOptions)

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
		b := NewEmulatedBlockchain(DefaultOptions)

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
		b := NewEmulatedBlockchain(DefaultOptions)

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
		b := NewEmulatedBlockchain(DefaultOptions)

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
	b := NewEmulatedBlockchain(DefaultOptions)

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

func TestCallScript(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

	tx1 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	// Sample call (value is 0)
	value, err := b.CallScript([]byte(sampleCall))
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(0), value)

	// Submit tx1 (script adds 2)
	err = b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	// Sample call (value is 2)
	value, err = b.CallScript([]byte(sampleCall))
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(2), value)
}

func TestQueryByVersion(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

	tx1 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	tx2 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx2.AddSignature(b.RootAccountAddress(), b.RootKey())

	var invalidWorldState crypto.Hash

	// Submit tx1 and tx2 (logging state versions before and after)
	ws1 := b.pendingWorldState.Hash()

	err := b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	ws2 := b.pendingWorldState.Hash()

	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)

	ws3 := b.pendingWorldState.Hash()

	// Get transaction at invalid world state version (errors)
	tx, err := b.GetTransactionAtVersion(tx1.Hash(), invalidWorldState)
	assert.IsType(t, err, &ErrInvalidStateVersion{})
	assert.Nil(t, tx)

	// tx1 does not exist at ws1
	tx, err = b.GetTransactionAtVersion(tx1.Hash(), ws1)
	assert.IsType(t, err, &ErrTransactionNotFound{})
	assert.Nil(t, tx)

	// tx1 does exist at ws2
	tx, err = b.GetTransactionAtVersion(tx1.Hash(), ws2)
	assert.Nil(t, err)
	assert.NotNil(t, tx)

	// tx2 does not exist at ws2
	tx, err = b.GetTransactionAtVersion(tx2.Hash(), ws2)
	assert.IsType(t, err, &ErrTransactionNotFound{})
	assert.Nil(t, tx)

	// tx2 does exist at ws3
	tx, err = b.GetTransactionAtVersion(tx2.Hash(), ws3)
	assert.Nil(t, err)
	assert.NotNil(t, tx)

	// Call script at invalid world state version (errors)
	value, err := b.CallScriptAtVersion([]byte(sampleCall), invalidWorldState)
	assert.IsType(t, err, &ErrInvalidStateVersion{})
	assert.Nil(t, value)

	// Value at ws1 is 0
	value, err = b.CallScriptAtVersion([]byte(sampleCall), ws1)
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(0), value)

	// Value at ws2 is 2 (after script executed)
	value, err = b.CallScriptAtVersion([]byte(sampleCall), ws2)
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(2), value)

	// Value at ws3 is 4 (after script executed)
	value, err = b.CallScriptAtVersion([]byte(sampleCall), ws3)
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(4), value)

	// Pending state does not change after call scripts/get transactions
	assert.Equal(t, ws3, b.pendingWorldState.Hash())
}

func TestRuntimeLogger(t *testing.T) {
	loggedMessages := make([]string, 0)

	b := NewEmulatedBlockchain(EmulatedBlockchainOptions{
		OnLogMessage: func(msg string) {
			loggedMessages = append(loggedMessages, msg)
		},
	})

	script := []byte(`
		fun main() {
			log("elephant ears")
		}
	`)

	_, err := b.CallScript(script)
	assert.Nil(t, err)
	assert.Equal(t, []string{`"elephant ears"`}, loggedMessages)
}

func TestEventEmitted(t *testing.T) {
	events := make([]types.Event, 0)

	b := NewEmulatedBlockchain(EmulatedBlockchainOptions{
		OnEventEmitted: func(event types.Event, blockNumber uint64, txHash crypto.Hash) {
			events = append(events, event)
		},
	})

	script := []byte(`
		event MyEvent(x: Int, y: Int)

		fun main() {
		  emit MyEvent(x: 1, y: 2)
		}
	`)

	tx := &types.Transaction{
		Script:             script,
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
	}

	tx.AddSignature(b.RootAccountAddress(), b.RootKey())

	err := b.SubmitTransaction(tx)
	assert.Nil(t, err)

	require.Len(t, events, 1)

	expectedID := fmt.Sprintf("tx.%s.MyEvent", tx.Hash().Hex())
	assert.Equal(t, expectedID, events[0].ID)

	assert.Equal(t, big.NewInt(1), events[0].Values["x"])
	assert.Equal(t, big.NewInt(2), events[0].Values["y"])
}

func TestEventEmittedFromAccount(t *testing.T) {
	events := make([]types.Event, 0)

	b := NewEmulatedBlockchain(EmulatedBlockchainOptions{
		OnEventEmitted: func(event types.Event, blockNumber uint64, txHash crypto.Hash) {
			events = append(events, event)
		},
	})

	accountScript := []byte(`
		event MyEvent(x: Int, y: Int)
	`)

	publicKeyA, _ := b.RootKey().Publickey().Encode()

	accountKeyA := types.AccountKey{
		PublicKey: publicKeyA,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	addressA, err := b.CreateAccount([]types.AccountKey{accountKeyA}, accountScript)
	assert.Nil(t, err)

	script := []byte(fmt.Sprintf(`
		import 0x%s
	
		fun main() {
			emit MyEvent(x: 1, y: 2)
		}
	`, addressA.Hex()))

	tx := &types.Transaction{
		Script:             script,
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
	}

	tx.AddSignature(b.RootAccountAddress(), b.RootKey())

	err = b.SubmitTransaction(tx)
	assert.Nil(t, err)

	require.Len(t, events, 1)

	expectedID := fmt.Sprintf("account.%s.MyEvent", addressA.Hex())
	assert.Equal(t, expectedID, events[0].ID)

	assert.Equal(t, big.NewInt(1), events[0].Values["x"])
	assert.Equal(t, big.NewInt(2), events[0].Values["y"])
}
