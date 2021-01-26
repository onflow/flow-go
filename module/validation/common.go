package validation

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// identityForNode ensures that `nodeID` is an authorized member of the network
// at the given block and returns the corresponding node's full identity.
// Error returns:
//   * sentinel engine.InvalidInputError is nodeID is NOT an authorized member of the network
//   * generic error indicating a fatal internal problem
func identityForNode(state protocol.State, blockID flow.Identifier, nodeID flow.Identifier) (*flow.Identity, error) {
	// get the identity of the origin node
	identity, err := state.AtBlockID(blockID).Identity(nodeID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return nil, engine.NewInvalidInputErrorf("unknown node identity: %w", err)
		}
		// unexpected exception
		return nil, fmt.Errorf("failed to retrieve node identity: %w", err)
	}

	return identity, nil
}
