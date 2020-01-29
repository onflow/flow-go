package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/iface"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/protocol"
)

type PayloadProvider interface {
	NewPayload(parentID flow.Identifier) (payloadID flow.Identifier)
}

type payloadProvider struct {
	state protocol.State

	// pool of free transactions
	free mempool.Transactions
}

// add a new block
//
// this happens when a block proposal is received from the network OR when we
// create a new block proposal from within HotStuff.
func (p *payloadProvider) add(parentID flow.Identifier, payload iface.Payload) error {
	// if parent does not exist, return an error -- must buffer the block
	// add block payload with link to parent
	// remove any items in free that exist in payload
	return nil
}

// prune forks that will never be used
//
// this happens when a block is finalized such that it would be impossible for
// the branch led by this block to ever be finalized.
//
// HotStuff notifies when a block is finalized
func (p *payloadProvider) prune(payloadID flow.Identifier) error {
	// remove the given payload and any children
	// for all removed payloads, if there exist any payload items that
	// do not exist in the finalized chain, nor in any un-finalized blocks,
	// add those items back to the free pool.
	return nil
}

func (p *payloadProvider) NewPayload(parentID flow.Identifier) (payloadID flow.Identifier) {
	// assert that the parent ID is known
	// NOTE: it is allowed for the parent to have a child block already
	//
	// produce a payload from free pool and from non-conflicting forks
	// add the payload: p.add(parentID, payload)
	// remove any items from free pool that are included in the payload
	// return hash of payload
	return flow.ZeroID
}
