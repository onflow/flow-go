package recovery

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRecover(t *testing.T) {
	finalized := unittest.BlockHeaderFixture()
	blocks := unittest.ChainFixtureFrom(100, &finalized)

	pending := make([]*flow.Header, 0)
	for _, b := range blocks {
		pending = append(pending, b.Header)
	}
	recovered := make([]*model.Proposal, 0)
	onProposal := func(block *model.Proposal) error {
		recovered = append(recovered, block)
		return nil
	}

	// make 3 invalid blocks extend from the last valid block
	invalidblocks := unittest.ChainFixtureFrom(3, pending[len(pending)-1])
	invalid := make(map[flow.Identifier]struct{})
	for _, b := range invalidblocks {
		invalid[b.ID()] = struct{}{}
		pending = append(pending, b.Header)
	}

	validator := &mocks.Validator{}
	validator.On("ValidateProposal", mock.Anything).Return(func(proposal *model.Proposal) error {
		header := model.ProposalToFlow(proposal)
		_, isInvalid := invalid[header.ID()]
		if isInvalid {
			return &model.InvalidBlockError{
				BlockID: header.ID(),
				View:    header.View,
			}
		}
		return nil
	})

	err := Recover(unittest.Logger(), &finalized, pending, validator, onProposal)
	require.NoError(t, err)

	// only pending blocks are valid
	require.Len(t, recovered, len(pending))
}
