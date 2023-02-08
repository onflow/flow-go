package signature

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
)

// BlockSignerDecoder is a wrapper around the `hotstuff.DynamicCommittee`, which implements
// the auxilluary logic for de-coding signer indices of a block (header) to full node IDs
type BlockSignerDecoder struct {
	hotstuff.DynamicCommittee
}

func NewBlockSignerDecoder(committee hotstuff.DynamicCommittee) *BlockSignerDecoder {
	return &BlockSignerDecoder{committee}
}

var _ hotstuff.BlockSignerDecoder = (*BlockSignerDecoder)(nil)

// DecodeSignerIDs decodes the signer indices from the given block header into full node IDs.
// Note: A block header contains a quorum certificate for its parent, which proves that
// the block extends a valid fork. Consequently, the returned IdentifierList contains the
// consensus participants that signed the parent block.
// Expected Error returns during normal operations:
//   - model.ErrViewForUnknownEpoch if the given block is within an unknown epoch
//   - signature.InvalidSignerIndicesError if signer indices included in the header do
//     not encode a valid subset of the consensus committee
func (b *BlockSignerDecoder) DecodeSignerIDs(header *flow.Header) (flow.IdentifierList, error) {
	// root block does not have signer indices
	if header.ParentVoterIndices == nil && header.View == 0 {
		return []flow.Identifier{}, nil
	}

	members, err := b.IdentitiesByEpoch(header.ParentView)
	if err != nil {
		if errors.Is(err, model.ErrViewForUnknownEpoch) {
			return nil, fmt.Errorf("could not retrieve identities for block %x with QC view %d: %w", header.ID(), header.ParentView, err)
		}
		return nil, fmt.Errorf("unexpected error retrieving identities for block %v: %w", header.ID(), err)
	}
	signerIDs, err := signature.DecodeSignerIndicesToIdentifiers(members.NodeIDs(), header.ParentVoterIndices)
	if err != nil {
		return nil, fmt.Errorf("could not decode signer indices for block %v: %w", header.ID(), err)
	}

	return signerIDs, nil
}

// NoopBlockSignerDecoder does not decode any signer indices and consistently returns
// nil for the signing node IDs (auxiliary data)
type NoopBlockSignerDecoder struct{}

func NewNoopBlockSignerDecoder() *NoopBlockSignerDecoder {
	return &NoopBlockSignerDecoder{}
}

func (b *NoopBlockSignerDecoder) DecodeSignerIDs(_ *flow.Header) (flow.IdentifierList, error) {
	return nil, nil
}
