package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	protocolstate "github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
)

// identityForNode ensures that `nodeID` is an authorized member of the network
// at the given block and returns the corresponding node's full identity.
// Error returns:
//   - engine.InvalidInputError if nodeID is NOT an authorized member of the network at the given block
//   - module.UnknownBlockError if blockID is not known to the protocol state
//
// All other error are potential symptoms critical internal failures, such as bugs or state corruption.
func identityForNode(state protocol.State, blockID flow.Identifier, nodeID flow.Identifier) (*flow.Identity, error) {
	// get the identity of the origin node
	identity, err := state.AtBlockID(blockID).Identity(nodeID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return nil, engine.NewInvalidInputErrorf("unknown node identity: %w", err)
		}
		if errors.Is(err, protocolstate.ErrUnknownSnapshotReference) {
			return nil, module.NewUnknownBlockError("block %v is unknown: %w", blockID, err)
		}
		return nil, fmt.Errorf("unexpected exception retrieving node identity: %w", err)
	}

	return identity, nil
}

// ensureNodeHasWeightAndRole checks whether, at the given block, `nodeID`
//   - has _positive_ weight
//   - and has the expected role
//   - and is not ejected
//
// This function is side-effect free. The only possible error it returns is of type
//   - engine.InvalidInputError if any of the above-listed conditions are violated.
//
// Note: the method receives the identity as proof of its existence.
// Therefore, we consider the case where the respective identity is unknown to the
// protocol state as a symptom of a fatal implementation bug.
func ensureNodeHasWeightAndRole(identity *flow.Identity, expectedRole flow.Role) error {
	// check that the role is expected
	if identity.Role != expectedRole {
		return engine.NewInvalidInputErrorf("expected node %x to have role %s but got %s", identity.NodeID, expectedRole, identity.Role)
	}

	// check if the identity has non-zero weight
	if identity.Weight == 0 {
		return engine.NewInvalidInputErrorf("node has zero weight (%x)", identity.NodeID)
	}

	// check that node was not ejected
	if identity.Ejected {
		return engine.NewInvalidInputErrorf("node was ejected from network (%x)", identity.NodeID)
	}

	return nil
}
