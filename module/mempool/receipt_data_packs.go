package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// ReceiptDataPacks represents a concurrency-safe memory pool for ReceiptDataPack data structure.
type ReceiptDataPacks interface {

	// Add will add the given ReceiptDataPack to the memory pool. It will return
	// false if it was already in the mempool.
	Add(rdp *verification.ReceiptDataPack) bool

	// Get returns the ReceiptDataPack and true, if the ReceiptDataPack is in the
	// mempool. Otherwise, it returns nil and false.
	Get(rdpID flow.Identifier) (*verification.ReceiptDataPack, bool)

	// Has checks if the given ReceiptDataPack is part of the memory pool.
	Has(rdpID flow.Identifier) bool

	// Rem will remove a ReceiptDataPack by ID.
	Rem(rdpID flow.Identifier) bool

	// Size returns total number ReceiptDataPacks in mempool
	Size() uint

	// All will return a list of all ReceiptDataPacks in the memory pool.
	All() []*verification.ReceiptDataPack
}
