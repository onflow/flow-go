package core

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

// sampleScript runs a script that adds 2 to a value.
const sampleScript = `
	fun main() {
		const controller = [1]
		const owner = [2]
		const key = [3]
		const value = getValue(controller, owner, key)
		setValue(controller, owner, key, value + 2)
	}
`

const sampleCall = `
	fun main() -> Int {
		return getValue([1], [2], [3])
	}
`

func TestWorldStates(t *testing.T) {
	RegisterTestingT(t)

	// Create new emulated blockchain
	b := NewEmulatedBlockchain()

	// Create 3 signed transactions (tx1, tx2. tx3)
	tx1 := &types.SignedTransaction{
		Script:         []byte(sampleScript),
		Nonce:          1,
		ComputeLimit:   10,
		Timestamp:      time.Now(),
		PayerSignature: crypto.Signature{},
	}

	tx2 := &types.SignedTransaction{
		Script:         []byte(sampleScript),
		Nonce:          2,
		ComputeLimit:   10,
		Timestamp:      time.Now(),
		PayerSignature: crypto.Signature{},
	}

	tx3 := &types.SignedTransaction{
		Script:         []byte(sampleScript),
		Nonce:          3,
		ComputeLimit:   10,
		Timestamp:      time.Now(),
		PayerSignature: crypto.Signature{},
	}

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
	Expect(b.worldStates).To(HaveKey(ws5))

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

	b := NewEmulatedBlockchain()

	tx1 := &types.SignedTransaction{
		Script:         []byte(sampleScript),
		Nonce:          1,
		ComputeLimit:   10,
		Timestamp:      time.Now(),
		PayerSignature: crypto.Signature{},
	}

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())

	// tx1 status becomes TransactionFinalized
	Expect(b.GetTransaction(tx1.Hash()).Status).To(Equal(types.TransactionFinalized))
}

func TestSubmitDuplicateTransaction(t *testing.T) {
	RegisterTestingT(t)

	b := NewEmulatedBlockchain()

	tx1 := &types.SignedTransaction{
		Script:         []byte(sampleScript),
		Nonce:          1,
		ComputeLimit:   10,
		Timestamp:      time.Now(),
		PayerSignature: crypto.Signature{},
	}

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())

	// Submit tx1 again (errors)
	err = b.SubmitTransaction(tx1)
	Expect(err).To(MatchError(&ErrDuplicateTransaction{TxHash: tx1.Hash()}))
}

func TestSubmitTransactionReverted(t *testing.T) {
	RegisterTestingT(t)

	b := NewEmulatedBlockchain()

	tx1 := &types.SignedTransaction{
		Script:         []byte("invalid script"),
		Nonce:          1,
		ComputeLimit:   10,
		Timestamp:      time.Now(),
		PayerSignature: crypto.Signature{},
	}

	// Submit invalid tx1 (errors)
	err := b.SubmitTransaction(tx1)
	Expect(err).To(HaveOccurred())

	// tx1 status becomes TransactionReverted
	Expect(b.GetTransaction(tx1.Hash()).Status).To(Equal(types.TransactionReverted))
}

func TestCommitBlock(t *testing.T) {
	RegisterTestingT(t)

	b := NewEmulatedBlockchain()

	tx1 := &types.SignedTransaction{
		Script:         []byte(sampleScript),
		Nonce:          1,
		ComputeLimit:   10,
		Timestamp:      time.Now(),
		PayerSignature: crypto.Signature{},
	}

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())
	Expect(b.GetTransaction(tx1.Hash()).Status).To(Equal(types.TransactionFinalized))

	tx2 := &types.SignedTransaction{
		Script:         []byte("invalid script"),
		Nonce:          1,
		ComputeLimit:   10,
		Timestamp:      time.Now(),
		PayerSignature: crypto.Signature{},
	}

	// Submit invalid tx2
	err = b.SubmitTransaction(tx2)
	Expect(err).To(HaveOccurred())
	Expect(b.GetTransaction(tx2.Hash()).Status).To(Equal(types.TransactionReverted))

	// Commit tx1 and tx2 into new block
	b.CommitBlock()

	// tx1 status becomes TransactionSealed
	Expect(b.GetTransaction(tx1.Hash()).Status).To(Equal(types.TransactionSealed))
	// tx2 status stays TransactionReverted
	Expect(b.GetTransaction(tx2.Hash()).Status).To(Equal(types.TransactionReverted))
}

func TestCallScript(t *testing.T) {
	RegisterTestingT(t)

	b := NewEmulatedBlockchain()

	tx1 := &types.SignedTransaction{
		Script:         []byte(sampleScript),
		Nonce:          1,
		ComputeLimit:   10,
		Timestamp:      time.Now(),
		PayerSignature: crypto.Signature{},
	}

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

	b := NewEmulatedBlockchain()

	tx1 := &types.SignedTransaction{
		Script:         []byte(sampleScript),
		Nonce:          1,
		ComputeLimit:   10,
		Timestamp:      time.Now(),
		PayerSignature: crypto.Signature{},
	}

	tx2 := &types.SignedTransaction{
		Script:         []byte(sampleScript),
		Nonce:          2,
		ComputeLimit:   10,
		Timestamp:      time.Now(),
		PayerSignature: crypto.Signature{},
	}

	invalidWorldState := crypto.NewHash([]byte("invalid state"))

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
	Expect(err).ToNot(HaveOccurred())
	Expect(tx).To(BeNil())

	// tx1 does exist at ws2
	tx, err = b.GetTransactionAtVersion(tx1.Hash(), ws2)
	Expect(err).ToNot(HaveOccurred())
	Expect(tx).ToNot(BeNil())

	// tx2 does not exist at ws2
	tx, err = b.GetTransactionAtVersion(tx2.Hash(), ws2)
	Expect(err).ToNot(HaveOccurred())
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
