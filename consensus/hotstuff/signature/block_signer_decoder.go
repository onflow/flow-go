package signature

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
)

// BlockSignerDecoder is a wrapper around the `hotstuff.Committee`, which implements
// the auxilluary logic for de-coding signer indices of a block (header) to full node IDs
type BlockSignerDecoder struct {
	// TODO: update to Replicas API once active PaceMaker is merged
	hotstuff.Committee
}

func NewBlockSignerDecoder(committee hotstuff.Committee) *BlockSignerDecoder {
	return &BlockSignerDecoder{committee}
}

var _ hotstuff.BlockSignerDecoder = (*BlockSignerDecoder)(nil)

// DecodeSignerIDs decodes the signer indices from the given block header into full node IDs.
// Expected Error returns during normal operations:
//  * state.UnknownBlockError if block has not been ingested yet
//  * signature.InvalidSignerIndicesError if signer indices included in the header do
//    not encode a valid subset of the consensus committee
func (b *BlockSignerDecoder) DecodeSignerIDs(header *flow.Header) (flow.IdentifierList, error) {
	// root block does not have signer indices
	if header.ParentVoterIndices == nil && header.View == 0 {
		return []flow.Identifier{}, nil
	}

	id := header.ID()
	members, err := b.Identities(id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, state.NewUnknownBlockErrorf("block %v has not been processed yet: %w", id, err)
		}
		return nil, fmt.Errorf("fail to retrieve identities for block %v: %w", id, err)
	}
	signerIDs, err := signature.DecodeSignerIndicesToIdentifiers(members.NodeIDs(), header.ParentVoterIndices)
	if err != nil {
		return nil, fmt.Errorf("could not decode signer indices for block %v: %w", header.ID(), err)
	}

	return signerIDs, nil
}

// NoopBlockSignerDecoder does not decode any signer indices and consistenlty return nil
type NoopBlockSignerDecoder struct{}

func (b *NoopBlockSignerDecoder) DecodeSignerIDs(_ *flow.Header) ([]flow.Identifier, error) {
	return nil, nil
}

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
