package emulator

import (
	"fmt"
	"math/big"
	"testing"

	. "github.com/onsi/gomega"

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
	RegisterTestingT(t)

	// Create new emulated blockchain
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
	Expect(b.txPool).To(HaveLen(0))

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())

	ws2 := b.pendingWorldState.Hash()
	t.Logf("world state after tx1: %x\n", ws2)

	// tx1 included in tx pool
	Expect(b.txPool).To(HaveLen(1))
	// World state updates
	Expect(ws2).NotTo(Equal(ws1))

	// Submit tx1 again
	err = b.SubmitTransaction(tx1)
	Expect(err).To(HaveOccurred())

	ws3 := b.pendingWorldState.Hash()
	t.Logf("world state after dup tx1: %x\n", ws3)

	// tx1 not included in tx pool
	Expect(b.txPool).To(HaveLen(1))
	// World state does not update
	Expect(ws3).To(Equal(ws2))

	// Submit tx2
	err = b.SubmitTransaction(tx2)
	Expect(err).ToNot(HaveOccurred())

	ws4 := b.pendingWorldState.Hash()
	t.Logf("world state after tx2: %x\n", ws4)

	// tx2 included in tx pool
	Expect(b.txPool).To(HaveLen(2))
	// World state updates
	Expect(ws4).NotTo(Equal(ws3))

	// Commit new block
	b.CommitBlock()
	ws5 := b.pendingWorldState.Hash()
	t.Logf("world state after commit: %x\n", ws5)

	// Tx pool cleared
	Expect(b.txPool).To(HaveLen(0))
	// World state updates
	Expect(ws5).NotTo(Equal(ws4))
	// World state is indexed
	Expect(b.worldStates).To(HaveKey(string(ws5)))

	// Submit tx3
	err = b.SubmitTransaction(tx3)
	Expect(err).ToNot(HaveOccurred())

	ws6 := b.pendingWorldState.Hash()
	t.Logf("world state after tx3: %x\n", ws6)

	// tx3 included in tx pool
	Expect(b.txPool).To(HaveLen(1))
	// World state updates
	Expect(ws6).NotTo(Equal(ws5))

	// Seek to committed block/world state
	b.SeekToState(ws5)
	ws7 := b.pendingWorldState.Hash()
	t.Logf("world state after seek: %x\n", ws7)

	// Tx pool cleared
	Expect(b.txPool).To(HaveLen(0))
	// World state rollback to ws5 (before tx3)
	Expect(ws7).To(Equal(ws5))
	// World state does not include tx3
	Expect(b.pendingWorldState.ContainsTransaction(tx3.Hash())).To(BeFalse())

	// Seek to non-committed world state
	b.SeekToState(ws4)
	ws8 := b.pendingWorldState.Hash()
	t.Logf("world state after failed seek: %x\n", ws8)

	// World state does not rollback to ws4 (before commit block)
	Expect(ws8).ToNot(Equal(ws4))
}

func TestSubmitTransaction(t *testing.T) {
	RegisterTestingT(t)

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
	Expect(err).ToNot(HaveOccurred())

	// tx1 status becomes TransactionFinalized
	tx, err := b.GetTransaction(tx1.Hash())
	Expect(err).ToNot(HaveOccurred())
	Expect(tx.Status).To(Equal(types.TransactionFinalized))
}

func TestSubmitDuplicateTransaction(t *testing.T) {
	RegisterTestingT(t)

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
	Expect(err).ToNot(HaveOccurred())

	// Submit tx1 again (errors)
	err = b.SubmitTransaction(tx1)
	Expect(err).To(MatchError(&ErrDuplicateTransaction{TxHash: tx1.Hash()}))
}

func TestSubmitTransactionScriptAccounts(t *testing.T) {
	RegisterTestingT(t)

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
	Expect(err).ToNot(HaveOccurred())

	t.Run("TooManyAccountsForScript", func(t *testing.T) {
		RegisterTestingT(t)

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
		Expect(err).To(HaveOccurred())
	})

	t.Run("NotEnoughAccountsForScript", func(t *testing.T) {
		RegisterTestingT(t)

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
		Expect(err).To(HaveOccurred())
	})
}

func TestSubmitTransactionPayerSignature(t *testing.T) {
	t.Run("MissingPayerSignature", func(t *testing.T) {
		RegisterTestingT(t)

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

		Expect(err).To(MatchError(&ErrMissingSignature{Account: addressA}))
	})

	t.Run("InvalidAccount", func(t *testing.T) {
		RegisterTestingT(t)

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

		Expect(err).To(MatchError(&ErrInvalidSignatureAccount{Account: invalidAddress}))
	})

	t.Run("InvalidKeyPair", func(t *testing.T) {
		RegisterTestingT(t)

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
		Expect(err).To(MatchError(&ErrInvalidSignaturePublicKey{Account: b.RootAccountAddress()}))
	})

	t.Run("KeyWeights", func(t *testing.T) {
		RegisterTestingT(t)

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
			RegisterTestingT(t)

			tx.AddSignature(accountAddressA, privateKeyB)

			err = b.SubmitTransaction(tx)
			Expect(err).To(MatchError(&ErrMissingSignature{Account: accountAddressA}))
		})

		t.Run("SufficientKeyWeight", func(t *testing.T) {
			RegisterTestingT(t)

			tx.AddSignature(accountAddressA, privateKeyA)
			tx.AddSignature(accountAddressA, privateKeyB)

			err = b.SubmitTransaction(tx)
			Expect(err).To(Not(HaveOccurred()))
		})
	})
}

func TestSubmitTransactionScriptSignatures(t *testing.T) {
	t.Run("MissingScriptSignature", func(t *testing.T) {
		RegisterTestingT(t)

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
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(&ErrMissingSignature{Account: addressA}))
	})

	t.Run("MultipleAccounts", func(t *testing.T) {
		RegisterTestingT(t)

		loggedMessages := make([]string, 0)

		b := NewEmulatedBlockchain(&EmulatedBlockchainOptions{
			RuntimeLogger: func(msg string) {
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
		Expect(err).ToNot(HaveOccurred())

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
		Expect(err).ToNot(HaveOccurred())

		Expect(loggedMessages).To(ContainElement(fmt.Sprintf(`"%x"`, accountAddressA.Bytes())))
		Expect(loggedMessages).To(ContainElement(fmt.Sprintf(`"%x"`, accountAddressB.Bytes())))
	})
}

func TestSubmitTransactionReverted(t *testing.T) {
	RegisterTestingT(t)

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
	Expect(err).To(HaveOccurred())

	// tx1 status becomes TransactionReverted
	tx, err := b.GetTransaction(tx1.Hash())
	Expect(err).ToNot(HaveOccurred())
	Expect(tx.Status).To(Equal(types.TransactionReverted))
}

func TestCommitBlock(t *testing.T) {
	RegisterTestingT(t)

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
	Expect(err).ToNot(HaveOccurred())
	Expect(tx.Status).To(Equal(types.TransactionFinalized))

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
	Expect(err).To(HaveOccurred())

	tx, err = b.GetTransaction(tx2.Hash())
	Expect(err).ToNot(HaveOccurred())

	Expect(tx.Status).To(Equal(types.TransactionReverted))

	// Commit tx1 and tx2 into new block
	b.CommitBlock()

	// tx1 status becomes TransactionSealed
	tx, _ = b.GetTransaction(tx1.Hash())
	Expect(tx.Status).To(Equal(types.TransactionSealed))
	// tx2 status stays TransactionReverted
	tx, _ = b.GetTransaction(tx2.Hash())
	Expect(tx.Status).To(Equal(types.TransactionReverted))
}

func TestCreateAccount(t *testing.T) {
	RegisterTestingT(t)

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
	Expect(err).ToNot(HaveOccurred())

	account, err := b.GetAccount(b.LastCreatedAccount().Address)
	Expect(err).ToNot(HaveOccurred())

	Expect(account.Balance).To(Equal(uint64(0)))
	Expect(account.Keys[0]).To(Equal(accountKeyA))
	Expect(account.Keys[1]).To(Equal(accountKeyB))
	Expect(account.Code).To(Equal(codeA))

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
	Expect(err).ToNot(HaveOccurred())

	account, err = b.GetAccount(b.LastCreatedAccount().Address)

	Expect(err).ToNot(HaveOccurred())
	Expect(account.Balance).To(Equal(uint64(0)))
	Expect(account.Keys[0]).To(Equal(accountKeyC))
	Expect(account.Code).To(Equal(codeB))
}

func TestAddAccountKey(t *testing.T) {
	RegisterTestingT(t)

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
	Expect(err).To(Not(HaveOccurred()))

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
	Expect(err).To(Not(HaveOccurred()))
}

func TestRemoveAccountKey(t *testing.T) {
	RegisterTestingT(t)

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
	Expect(err).To(Not(HaveOccurred()))

	account, err := b.GetAccount(b.RootAccountAddress())
	Expect(err).To(Not(HaveOccurred()))

	Expect(account.Keys).To(HaveLen(2))

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
	Expect(err).To(Not(HaveOccurred()))

	account, err = b.GetAccount(b.RootAccountAddress())
	Expect(err).To(Not(HaveOccurred()))

	Expect(account.Keys).To(HaveLen(1))

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
	Expect(err).To(HaveOccurred())

	account, err = b.GetAccount(b.RootAccountAddress())
	Expect(err).To(Not(HaveOccurred()))

	Expect(account.Keys).To(HaveLen(1))

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
	Expect(err).To(Not(HaveOccurred()))

	account, err = b.GetAccount(b.RootAccountAddress())
	Expect(err).To(Not(HaveOccurred()))

	Expect(account.Keys).To(HaveLen(0))
}

func TestUpdateAccountCode(t *testing.T) {
	privateKeyB, _ := crypto.GeneratePrivateKey(crypto.ECDSA_P256, []byte("elephant ears"))
	publicKeyB, _ := privateKeyB.Publickey().Encode()

	accountKeyB := types.AccountKey{
		PublicKey: publicKeyB,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	t.Run("ValidSignature", func(t *testing.T) {
		RegisterTestingT(t)

		b := NewEmulatedBlockchain(DefaultOptions)

		privateKeyA := b.RootKey()

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]types.AccountKey{accountKeyB}, []byte{4, 5, 6})
		Expect(err).ToNot(HaveOccurred())

		account, err := b.GetAccount(accountAddressB)

		Expect(err).ToNot(HaveOccurred())
		Expect(account.Code).To(Equal([]byte{4, 5, 6}))

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
		Expect(err).ToNot(HaveOccurred())

		account, err = b.GetAccount(accountAddressB)

		Expect(err).ToNot(HaveOccurred())
		Expect(account.Code).To(Equal([]byte{7, 8, 9}))
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		RegisterTestingT(t)

		b := NewEmulatedBlockchain(DefaultOptions)

		privateKeyA := b.RootKey()

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]types.AccountKey{accountKeyB}, []byte{4, 5, 6})
		Expect(err).ToNot(HaveOccurred())

		account, err := b.GetAccount(accountAddressB)

		Expect(err).ToNot(HaveOccurred())
		Expect(account.Code).To(Equal([]byte{4, 5, 6}))

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
		Expect(err).To(HaveOccurred())

		account, err = b.GetAccount(accountAddressB)

		// code should not be updated
		Expect(err).ToNot(HaveOccurred())
		Expect(account.Code).To(Equal([]byte{4, 5, 6}))
	})

	t.Run("UnauthorizedAccount", func(t *testing.T) {
		RegisterTestingT(t)

		b := NewEmulatedBlockchain(DefaultOptions)

		privateKeyA := b.RootKey()

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]types.AccountKey{accountKeyB}, []byte{4, 5, 6})
		Expect(err).ToNot(HaveOccurred())

		account, err := b.GetAccount(accountAddressB)

		Expect(err).ToNot(HaveOccurred())
		Expect(account.Code).To(Equal([]byte{4, 5, 6}))

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
		Expect(err).To(HaveOccurred())

		account, err = b.GetAccount(accountAddressB)

		// code should not be updated
		Expect(err).ToNot(HaveOccurred())
		Expect(account.Code).To(Equal([]byte{4, 5, 6}))
	})
}

func TestImportAccountCode(t *testing.T) {
	RegisterTestingT(t)

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
	Expect(err).ToNot(HaveOccurred())

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
	Expect(err).ToNot(HaveOccurred())
}

func TestCallScript(t *testing.T) {
	RegisterTestingT(t)

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
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(big.NewInt(0)))

	// Submit tx1 (script adds 2)
	err = b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())

	// Sample call (value is 2)
	value, err = b.CallScript([]byte(sampleCall))
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(big.NewInt(2)))
}

func TestQueryByVersion(t *testing.T) {
	RegisterTestingT(t)

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
	Expect(err).ToNot(HaveOccurred())

	ws2 := b.pendingWorldState.Hash()

	err = b.SubmitTransaction(tx2)
	Expect(err).ToNot(HaveOccurred())

	ws3 := b.pendingWorldState.Hash()

	// Get transaction at invalid world state version (errors)
	tx, err := b.GetTransactionAtVersion(tx1.Hash(), invalidWorldState)
	Expect(err).To(MatchError(&ErrInvalidStateVersion{Version: invalidWorldState}))
	Expect(tx).To(BeNil())

	// tx1 does not exist at ws1
	tx, err = b.GetTransactionAtVersion(tx1.Hash(), ws1)
	Expect(err).To(MatchError(&ErrTransactionNotFound{TxHash: tx1.Hash()}))
	Expect(tx).To(BeNil())

	// tx1 does exist at ws2
	tx, err = b.GetTransactionAtVersion(tx1.Hash(), ws2)
	Expect(err).ToNot(HaveOccurred())
	Expect(tx).ToNot(BeNil())

	// tx2 does not exist at ws2
	tx, err = b.GetTransactionAtVersion(tx2.Hash(), ws2)
	Expect(err).To(MatchError(&ErrTransactionNotFound{TxHash: tx2.Hash()}))
	Expect(tx).To(BeNil())

	// tx2 does exist at ws3
	tx, err = b.GetTransactionAtVersion(tx2.Hash(), ws3)
	Expect(err).ToNot(HaveOccurred())
	Expect(tx).ToNot(BeNil())

	// Call script at invalid world state version (errors)
	value, err := b.CallScriptAtVersion([]byte(sampleCall), invalidWorldState)
	Expect(err).To(MatchError(&ErrInvalidStateVersion{Version: invalidWorldState}))
	Expect(value).To(BeNil())

	// Value at ws1 is 0
	value, err = b.CallScriptAtVersion([]byte(sampleCall), ws1)
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(big.NewInt(0)))

	// Value at ws2 is 2 (after script executed)
	value, err = b.CallScriptAtVersion([]byte(sampleCall), ws2)
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(big.NewInt(2)))

	// Value at ws3 is 4 (after script executed)
	value, err = b.CallScriptAtVersion([]byte(sampleCall), ws3)
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(big.NewInt(4)))

	// Pending state does not change after call scripts/get transactions
	Expect(b.pendingWorldState.Hash()).To(Equal(ws3))
}

func TestRuntimeLogger(t *testing.T) {
	RegisterTestingT(t)

	loggedMessages := make([]string, 0)

	b := NewEmulatedBlockchain(&EmulatedBlockchainOptions{
		RuntimeLogger: func(msg string) {
			loggedMessages = append(loggedMessages, msg)
		},
	})

	script := []byte(`
		fun main() {
			log("elephant ears")
		}
	`)

	_, err := b.CallScript(script)
	Expect(err).ToNot(HaveOccurred())
	Expect(loggedMessages).To(Equal([]string{`"elephant ears"`}))
}
