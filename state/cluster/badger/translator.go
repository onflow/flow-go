package badger

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Translator is a translation layer that determines the reference block on
// the main chain for a given cluster block, using the reference block from
// the cluster block's payload.
type Translator struct {
	payloads storage.ClusterPayloads
	state    protocol.State
}

// NewTranslator returns a new block ID translator.
func NewTranslator(payloads storage.ClusterPayloads, state protocol.State) *Translator {
	translator := &Translator{
		payloads: payloads,
		state:    state,
	}
	return translator
}

// Translate retrieves the reference main-chain block ID for the given cluster
// block ID.
func (t *Translator) Translate(blockID flow.Identifier) (flow.Identifier, error) {

	payload, err := t.payloads.ByBlockID(blockID)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not retrieve reference block payload: %w", err)
	}

	// if a reference block is specified, use that
	if payload.ReferenceBlockID != flow.ZeroID {
		return payload.ReferenceBlockID, nil
	}

	// otherwise, we are dealing with a root block, and must retrieve the
	// reference block by epoch number
	//TODO this returns the latest block in the epoch, thus will take slashing
	// into account. We don't slash yet, so this is OK short-term.
	// We should change the API boundaries a bit here, so this chain-aware
	// translation changes to be f(blockID) -> IdentityList rather than
	// f(blockID) -> blockID.
	// REF: https://github.com/dapperlabs/flow-go/issues/4655
	head, err := t.state.Final().Head()
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not retrieve block: %w", err)
	}
	return head.ID(), nil
}
