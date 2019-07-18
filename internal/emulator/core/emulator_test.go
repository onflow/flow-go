package core

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

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

	Expect(b.txPool).To(HaveLen(0))

	// Submit tx1
	b.SubmitTransaction(tx1)
	ws2 := b.pendingWorldState.Hash()
	t.Logf("world state after tx1: \t%s\n", ws2)

	Expect(b.txPool).To(HaveLen(1))
	Expect(ws2).NotTo(Equal(ws1))

	// Submit tx1 again
	b.SubmitTransaction(tx1)
	ws3 := b.pendingWorldState.Hash()
	t.Logf("world state after dup tx1: \t%s\n", ws3)

	Expect(b.txPool).To(HaveLen(1))
	Expect(ws3).To(Equal(ws2))

	// Submit tx2
	b.SubmitTransaction(tx2)
	ws4 := b.pendingWorldState.Hash()
	t.Logf("world state after tx2: \t%s\n", ws4)

	Expect(b.txPool).To(HaveLen(2))
	Expect(ws4).NotTo(Equal(ws3))

	// Commit new block
	b.CommitBlock()
	ws5 := b.pendingWorldState.Hash()
	t.Logf("world state after commit: \t%s\n", ws5)

	Expect(b.txPool).To(HaveLen(0))
	Expect(ws5).NotTo(Equal(ws4))
	Expect(b.worldStates).To(HaveKey(ws5))

	// Submit tx3
	b.SubmitTransaction(tx3)
	ws6 := b.pendingWorldState.Hash()
	t.Logf("world state after tx3: \t%s\n", ws6)

	Expect(b.txPool).To(HaveLen(1))
	Expect(ws6).NotTo(Equal(ws5))

	// Seek to committed block/world state
	b.SeekToState(ws5)
	ws7 := b.pendingWorldState.Hash()
	t.Logf("world state after seek: \t%s\n", ws7)

	Expect(b.txPool).To(HaveLen(0))
	Expect(ws7).To(Equal(ws5))

	// Seek to non-committed world state
	b.SeekToState(ws4)
	ws8 := b.pendingWorldState.Hash()
	t.Logf("world state after failed seek: \t%s\n", ws8)

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

	err := b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())

	tx2 := b.GetTransaction(tx1.Hash())
	Expect(tx2.Status).To(Equal(types.TransactionFinalized))
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

	err := b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())

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

	err := b.SubmitTransaction(tx1)
	Expect(err).To(HaveOccurred())

	tx2 := b.GetTransaction(tx1.Hash())
	Expect(tx2.Status).To(Equal(types.TransactionReverted))
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

	value, err := b.CallScript([]byte(sampleCall))
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(0))

	err = b.SubmitTransaction(tx1)
	Expect(err).ToNot(HaveOccurred())

	value, err = b.CallScript([]byte(sampleCall))
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(2))
}
