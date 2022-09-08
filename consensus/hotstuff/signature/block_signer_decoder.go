package signature

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"
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
// Expected Error returns during normal operations:
//   - state.UnknownBlockError if block has not been ingested yet
//   - signature.InvalidSignerIndicesError if signer indices included in the header do
//     not encode a valid subset of the consensus committee
func (b *BlockSignerDecoder) DecodeSignerIDs(header *flow.Header) (flow.IdentifierList, error) {
	// root block does not have signer indices
	if header.ParentVoterIndices == nil && header.View == 0 {
		return []flow.Identifier{}, nil
	}

	id := header.ID()
	members, err := b.IdentitiesByBlock(id)
	if err != nil {
		// TODO: this potentially needs to be updated when we implement and document proper error handling for
		//       `hotstuff.Committee` and underlying code (such as `protocol.Snapshot`)
		if errors.Is(err, storage.ErrNotFound) {
			return nil, state.WrapAsUnknownBlockError(id, err)
		}
		return nil, fmt.Errorf("fail to retrieve identities for block %v: %w", id, err)
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
