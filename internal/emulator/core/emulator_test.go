package core

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
)

func TestEmulatedBlockchain(t *testing.T) {
	RegisterTestingT(t)

	// Create new emulated blockchain
	b := NewEmulatedBlockchain()

	// Create 3 txns and "sign" them
	tx1, tx2, tx3 := &types.RawTransaction{Nonce: 1}, &types.RawTransaction{Nonce: 2}, &types.RawTransaction{Nonce: 3}
	signedTx1, signedTx2, signedTx3 := &types.SignedTransaction{Transaction: tx1}, &types.SignedTransaction{Transaction: tx2}, &types.SignedTransaction{Transaction: tx3}

	ws1 := b.pendingWorldState.Hash()
	fmt.Printf("initial world state: \t%s\n", ws1)

	Expect(b.txPool).To(HaveLen(0))

	// Submit tx1
	b.SubmitTransaction(signedTx1)
	ws2 := b.pendingWorldState.Hash()
	fmt.Printf("world state after tx1: \t%s\n", ws2)

	Expect(b.txPool).To(HaveLen(1))
	Expect(ws2).NotTo(Equal(ws1))

	// Submit tx1 again
	b.SubmitTransaction(signedTx1)
	ws3 := b.pendingWorldState.Hash()
	fmt.Printf("world state after dup tx1: \t%s\n", ws3)

	Expect(b.txPool).To(HaveLen(1))
	Expect(ws3).To(Equal(ws2))

	// Submit tx2
	b.SubmitTransaction(signedTx2)
	ws4 := b.pendingWorldState.Hash()
	fmt.Printf("world state after tx2: \t%s\n", ws4)

	Expect(b.txPool).To(HaveLen(2))
	Expect(ws4).NotTo(Equal(ws3))

	// Commit new block
	b.CommitBlock()
	ws5 := b.pendingWorldState.Hash()
	fmt.Printf("world state after commit: \t%s\n", ws5)

	Expect(b.txPool).To(HaveLen(0))
	Expect(ws5).NotTo(Equal(ws4))
	Expect(b.worldStates).To(HaveKey(ws5))

	// Submit tx3
	b.SubmitTransaction(signedTx3)
	ws6 := b.pendingWorldState.Hash()
	fmt.Printf("world state after tx3: \t%s\n", ws6)

	Expect(b.txPool).To(HaveLen(1))
	Expect(ws6).NotTo(Equal(ws5))

	// Seek to committed block/world state
	b.SeekToState(ws5)
	ws7 := b.pendingWorldState.Hash()
	fmt.Printf("world state after seek: \t%s\n", ws7)

	Expect(b.txPool).To(HaveLen(0))
	Expect(ws7).To(Equal(ws5))
}
