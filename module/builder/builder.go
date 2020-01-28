// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package builder

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/protocol"
)

// Builder is a simply block builder.
type Builder struct {
	state      protocol.State
	guarantees mempool.Guarantees
	seals      mempool.Seals
}

// New initializes a new block builder that builds blocks using the given
// protocol state and the provided memory pools.
func New(state protocol.State, guarantees mempool.Guarantees, seals mempool.Seals) *Builder {
	b := &Builder{
		state:      state,
		guarantees: guarantees,
		seals:      seals,
	}
	return b
}

// BuildOn creates a new block on top of the provided block.
func (b *Builder) BuildOn(parentID flow.Identifier) (flow.Identifier, error) {

	// at the moment, we simply include all new guarantees
	guarantees := b.guarantees.All()

	// get the finalized head block for the parent ID
	parent, err := b.state.AtBlockID(parentID).Head()
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get parent header: %w", err)
	}

	// NOTE: this sanity check should never fail, but I'd rather make sure and
	// see if we ever crash and need to rethink the protocol state logic
	if parent.ID() != parentID {
		panic("should not build on block with finalized child")
	}

	// get the execution state for the parent block
	commit, err := b.state.AtBlockID(parent.ID()).Commit()
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get execution state commit: %w", err)
	}

	// TODO: manage pending guarantees for competing forks

	// we then keep adding seals that follow this state commit from the pool
	var seals []*flow.Seal
	for {

		// first, we get the seal that has the current state as parent
		seal, err := b.seals.ByParentCommit(commit)
		if errors.Is(err, mempool.ErrEntityNotFound) {
			break
		}
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not get mempool seal: %w", err)
		}

		// TODO: handle multiple seals with same parent

		// we then check if this block already exists in our chain
		_, err = b.state.AtBlockID(seal.BlockID).Head()
		if err != nil {
			break
		}

		// if it exists, we can include it and forward to next seal
		seals = append(seals, seal)
		commit = seal.StateCommit
	}

	// create the block content with the collection guarantees
	payload := flow.Payload{
		Identities: nil,
		Guarantees: guarantees,
		Seals:      seals,
	}

	// TODO: remember the payload here

	return payload.Hash(), nil
}
