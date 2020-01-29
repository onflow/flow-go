package iface

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// PayloadItem used to represent the individual items in a payload.
// This will be eg. transactions in collection node, guarantees in consensus
// nodes.
type PayloadItem struct{}

// Payload used here to represent a generic payload type, would be a set of
// payload items.
type Payload []PayloadItem

type AggregatedSignature []byte

type QC struct {
	View    uint64
	BlockID flow.Identifier
	Sig     AggregatedSignature
}

type BlockHeader struct {
	ChainID   uint64
	Height    uint64
	ParentID  flow.Identifier
	PayloadID flow.Identifier
	Justify   *QC
}

func (b *BlockHeader) View() uint64 {
	return b.Justify.View + 1
}

type Vote struct {
	View    uint64
	BlockID flow.Identifier
}

type HotStuff interface {
	Start() error
	SubmitBlock(block BlockHeader)
	SubmitVote(vote Vote)
}

// Mempool represents pending payload items. Payload items can be in the free
// pool, in an unfinalized block's payload, or in a finalized block's payload.
//
// Free items are not in any known block's payload. They are safe to include in
// any new block.
//
// Un-finalized items are in at least one known un-finalized block's payload
// and in no finalized block's payload.
// They are safe to include in any new block that DOES NOT extend a block that
// includes the item.
//
// Finalized items are in exactly one known finalized block's payload.
// They are not safe to include in any new block.
type Mempool interface {

	// AddItem adds an item to the free pool that is not yet associated with a
	// block payload
	AddItem(item PayloadItem)

	// AddBlockPayload adds a block payload to the mempool
	AddBlockPayload(parentID flow.Identifier, payload Payload)

	// FreeBlockPayload frees the payload for the given block. For any item in
	// the payload that does not yet exist in any finalized or un-finalized block,
	// add that item back to the free pool.
	FreeBlockPayload(payloadID flow.Identifier)
}

// PayloadProducer is a view into the overall chain state allowing the block
// producer module in HotStuff to generate a valid payload when proposing a new
// block.
type PayloadProducer interface {

	// GetPayload generates a payload that is safe w.r.t to the chain fork we
	// are building the new block upon. In other words, the payload contents
	// must not conflict with the contents of any payloads in:
	// * any finalized blocks
	// * the parent block and any un-finalized blocks it extends
	//
	// The payload can and should include items from unfinalized blocks in
	// conflicting forks.
	GetPayload(parentID flow.Identifier) (payloadID flow.Identifier)
}
