package core

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
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

// createAccountScriptA runs a script that creates an account.
const createAccountScriptA = `
	fun main(account: Account) {
		let publicKey = [1,2,3]
		let code = [4,5,6]
		createAccount(publicKey, code)
	}
`

// createAccountScriptB runs a script that creates an account.
const createAccountScriptB = `
	fun main(account: Account) {
		let publicKey = [7,8,9]
		let code = [10,11,12]
		createAccount(publicKey, code)
	}
`

// updateAccountCodeScript runs a script that updates the code for an account.
const updateAccountCodeScript = `
	fun main(account: Account) {
		let account = [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,2]
		let code = [102,117,110,32,109,97,105,110,40,41,32,123,125]
		updateAccountCode(account, code)
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

	// Create 3 signed transactions (tx1, tx2. tx3)
	tx1 := (&types.RawTransaction{
		Script:       []byte(addTwoScript),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	tx2 := (&types.RawTransaction{
		Script:       []byte(addTwoScript),
		Nonce:        2,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	tx3 := (&types.RawTransaction{
		Script:       []byte(addTwoScript),
		Nonce:        3,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	ws1 := b.pendingWorldState.Hash()
	t.Logf("initial world state: \t%s\n", ws1)

	// Tx pool contains nothing
	Expect(b.txPool).To(HaveLen(0))

	// Submit tx1
	b.SubmitTransaction(tx1)
	ws2 := b.pendingWorldState.Hash()
	t.Logf("world state after tx1: \t%s\n", ws2)

	// tx1 included in tx pool
	Expect(b.txPool).To(HaveLen(1))
	// World state updates
	Expect(ws2).NotTo(Equal(ws1))

	// Submit tx1 again
	b.SubmitTransaction(tx1)
	ws3 := b.pendingWorldState.Hash()
	t.Logf("world state after dup tx1: \t%s\n", ws3)

	// tx1 not included in tx pool
	Expect(b.txPool).To(HaveLen(1))
	// World state does not update
	Expect(ws3).To(Equal(ws2))

	// Submit tx2
	b.SubmitTransaction(tx2)
	ws4 := b.pendingWorldState.Hash()
	t.Logf("world state after tx2: \t%s\n", ws4)

	// tx2 included in tx pool
	Expect(b.txPool).To(HaveLen(2))
	// World state updates
	Expect(ws4).NotTo(Equal(ws3))

	// Commit new block
	b.CommitBlock()
	ws5 := b.pendingWorldState.Hash()
	t.Logf("world state after commit: \t%s\n", ws5)

	// Tx pool cleared
	Expect(b.txPool).To(HaveLen(0))
	// World state updates
	Expect(ws5).NotTo(Equal(ws4))
	// World state is indexed
	Expect(b.worldStates).To(HaveKey(string(ws5.Bytes())))

	// Submit tx3
	b.SubmitTransaction(tx3)
	ws6 := b.pendingWorldState.Hash()
	t.Logf("world state after tx3: \t%s\n", ws6)

	// tx3 included in tx pool
	Expect(b.txPool).To(HaveLen(1))
	// World state updates
	Expect(ws6).NotTo(Equal(ws5))

	// Seek to committed block/world state
	b.SeekToState(ws5)
	ws7 := b.pendingWorldState.Hash()
	t.Logf("world state after seek: \t%s\n", ws7)

	// Tx pool cleared
	Expect(b.txPool).To(HaveLen(0))
	// World state rollback to ws5 (before tx3)
	Expect(ws7).To(Equal(ws5))
	// World state does not include tx3
	Expect(b.pendingWorldState.ContainsTransaction(tx3.Hash())).To(BeFalse())

	// Seek to non-committed world state
	b.SeekToState(ws4)
	ws8 := b.pendingWorldState.Hash()
	t.Logf("world state after failed seek: \t%s\n", ws8)

	// World state does not rollback to ws4 (before commit block)
	Expect(ws8).ToNot(Equal(ws4))
}

func TestSubmitTransaction(t *testing.T) {
	RegisterTestingT(t)

	b := NewEmulatedBlockchain(DefaultOptions)

	tx1 := (&types.RawTransaction{
		Script:       []byte(addTwoScript),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

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

	tx1 := (&types.RawTransaction{
		Script:       []byte(addTwoScript),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())

	// Submit tx1 again (errors)
	err = b.SubmitTransaction(tx1)
	Expect(err).To(MatchError(&ErrDuplicateTransaction{TxHash: tx1.Hash()}))
}

func TestSubmitTransactionInvalidAccount(t *testing.T) {
	RegisterTestingT(t)

	b := NewEmulatedBlockchain(DefaultOptions)

	invalidAddress := types.HexToAddress("0000000000000000000000000000000000000002")

	tx1 := (&types.RawTransaction{
		Script:       []byte(addTwoScript),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(invalidAddress, b.RootKey())

	// Submit invalid tx1 (errors)
	err := b.SubmitTransaction(tx1)
	Expect(err).To(MatchError(&ErrInvalidSignatureAccount{Account: invalidAddress}))
}

func TestSubmitTransactionInvalidKeyPair(t *testing.T) {
	RegisterTestingT(t)

	b := NewEmulatedBlockchain(DefaultOptions)

	// use key pair that does not exist on root account
	salg, _ := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
	invalidKey, _ := salg.GeneratePrKey([]byte("invalid key"))

	tx1 := (&types.RawTransaction{
		Script:       []byte(addTwoScript),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), invalidKey)

	// Submit invalid tx1 (errors)
	err := b.SubmitTransaction(tx1)
	Expect(err).To(MatchError(
		&ErrInvalidSignaturePublicKey{
			Account: b.RootAccount(),
		}),
	)
}

func TestSubmitTransactionReverted(t *testing.T) {
	RegisterTestingT(t)

	b := NewEmulatedBlockchain(DefaultOptions)

	tx1 := (&types.RawTransaction{
		Script:       []byte("invalid script"),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

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

	tx1 := (&types.RawTransaction{
		Script:       []byte(addTwoScript),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	tx, _ := b.GetTransaction(tx1.Hash())
	Expect(err).ToNot(HaveOccurred())
	Expect(tx.Status).To(Equal(types.TransactionFinalized))

	tx2 := (&types.RawTransaction{
		Script:       []byte("invalid script"),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	// Submit invalid tx2
	err = b.SubmitTransaction(tx2)
	tx, _ = b.GetTransaction(tx2.Hash())
	Expect(err).To(HaveOccurred())
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

	tx1 := (&types.RawTransaction{
		Script:       []byte(createAccountScriptA),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	err := b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())

	// root account has ID 1, so expect this account to have ID 2
	address := types.HexToAddress("0000000000000000000000000000000000000002")

	account, err := b.GetAccount(address)

	Expect(err).ToNot(HaveOccurred())
	Expect(account.Balance).To(Equal(uint64(0)))
	Expect(account.PublicKeys).To(ContainElement([]byte{1, 2, 3}))
	Expect(account.Code).To(Equal([]byte{4, 5, 6}))

	tx2 := (&types.RawTransaction{
		Script:       []byte(createAccountScriptB),
		Nonce:        2,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	err = b.SubmitTransaction(tx2)
	Expect(err).ToNot(HaveOccurred())

	address = types.HexToAddress("0000000000000000000000000000000000000003")

	account, err = b.GetAccount(address)

	Expect(err).ToNot(HaveOccurred())
	Expect(account.Balance).To(Equal(uint64(0)))
	Expect(account.PublicKeys).To(ContainElement([]byte{7, 8, 9}))
	Expect(account.Code).To(Equal([]byte{10, 11, 12}))
}

func TestUpdateAccountCode(t *testing.T) {
	RegisterTestingT(t)

	b := NewEmulatedBlockchain(DefaultOptions)

	tx1 := (&types.RawTransaction{
		Script:       []byte(createAccountScriptA),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	err := b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())

	// root account has ID 1, so expect this account to have ID 2
	address := types.HexToAddress("0000000000000000000000000000000000000002")

	account, err := b.GetAccount(address)

	Expect(err).ToNot(HaveOccurred())
	Expect(account.Code).To(Equal([]byte{4, 5, 6}))

	tx2 := (&types.RawTransaction{
		Script:       []byte(updateAccountCodeScript),
		Nonce:        2,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	err = b.SubmitTransaction(tx2)
	Expect(err).ToNot(HaveOccurred())

	account, err = b.GetAccount(address)

	Expect(err).ToNot(HaveOccurred())
	Expect(account.Code).To(Equal([]byte{102, 117, 110, 32, 109, 97, 105, 110, 40, 41, 32, 123, 125}))
}

func TestCallScript(t *testing.T) {
	RegisterTestingT(t)

	b := NewEmulatedBlockchain(DefaultOptions)

	tx1 := (&types.RawTransaction{
		Script:       []byte(addTwoScript),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	// Sample call (value is 0)
	value, err := b.CallScript([]byte(sampleCall))
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(0))

	// Submit tx1 (script adds 2)
	err = b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())

	// Sample call (value is 2)
	value, err = b.CallScript([]byte(sampleCall))
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(2))
}

func TestQueryByVersion(t *testing.T) {
	RegisterTestingT(t)

	b := NewEmulatedBlockchain(DefaultOptions)

	tx1 := (&types.RawTransaction{
		Script:       []byte(addTwoScript),
		Nonce:        1,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	tx2 := (&types.RawTransaction{
		Script:       []byte(addTwoScript),
		Nonce:        2,
		ComputeLimit: 10,
		Timestamp:    time.Now(),
	}).SignPayer(b.RootAccount(), b.RootKey())

	invalidWorldState := new(crypto.Hash32)

	// Submit tx1 and tx2 (logging state versions before and after)
	ws1 := b.pendingWorldState.Hash()
	b.SubmitTransaction(tx1)
	ws2 := b.pendingWorldState.Hash()
	b.SubmitTransaction(tx2)
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
	Expect(value).To(Equal(0))

	// Value at ws2 is 2 (after script executed)
	value, err = b.CallScriptAtVersion([]byte(sampleCall), ws2)
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(2))

	// Value at ws3 is 4 (after script executed)
	value, err = b.CallScriptAtVersion([]byte(sampleCall), ws3)
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(4))

	// Pending state does not change after call scripts/get transactions
	Expect(b.pendingWorldState.Hash()).To(Equal(ws3))
}
