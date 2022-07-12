package signature

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
)

// DecodeSignerIDs decodes the signer indices from the given block header, and finds the signer identifiers from protocol state
// Expected Error returns during normal operations:
//  * storage.ErrNotFound if block not found for the given header
//  * signature.InvalidSignerIndicesError if `signerIndices` does not encode a valid subset of the consensus committee
// TODO: change `protocol.State` to `Replicas` API once active PaceMaker is merged
func DecodeSignerIDs(committee hotstuff.Committee, header *flow.Header) ([]flow.Identifier, error) {
	// root block does not have signer indices
	if header.ParentVoterIndices == nil && header.View == 0 {
		return []flow.Identifier{}, nil
	}

	members, err := committee.Identities(header.ID())
	if err != nil {
		return nil, fmt.Errorf("fail to retrieve identities for block %v: %w", header.ID(), err)
	}
	signerIDs, err := signature.DecodeSignerIndicesToIdentifiers(members.NodeIDs(), header.ParentVoterIndices)
	if err != nil {
		return nil, fmt.Errorf("could not decode signer indices for block %v: %w", header.ID(), err)
	}

	return signerIDs, nil
}
