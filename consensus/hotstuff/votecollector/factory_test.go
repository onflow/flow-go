package votecollector

import (
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestVoteProcessorFactory_CreateWithValidProposal checks if VoteProcessorFactory checks the proposer vote
// based on submitted proposal
func TestVoteProcessorFactory_CreateWithValidProposal(t *testing.T) {
	mockedFactory := mockhotstuff.VoteProcessorFactory{}

	proposal := helper.MakeProposal()
	mockedProcessor := &mockhotstuff.VerifyingVoteProcessor{}
	mockedProcessor.On("Process", proposal.ProposerVote()).Return(nil).Once()
	mockedFactory.On("Create", unittest.Logger(), proposal).Return(mockedProcessor, nil).Once()

	voteProcessorFactory := &VoteProcessorFactory{
		baseFactory: func(log zerolog.Logger, block *model.Block) (hotstuff.VerifyingVoteProcessor, error) {
			return mockedFactory.Create(log, proposal)
		},
	}

	processor, err := voteProcessorFactory.Create(unittest.Logger(), proposal)
	require.NoError(t, err)
	require.NotNil(t, processor)

	mockedProcessor.AssertExpectations(t)
	mockedFactory.AssertExpectations(t)
}

// TestVoteProcessorFactory_CreateWithInvalidVote tests that processing proposal with invalid vote doesn't return
// vote processor and returns correct error(sentinel or exception).
func TestVoteProcessorFactory_CreateWithInvalidVote(t *testing.T) {
	mockedFactory := mockhotstuff.VoteProcessorFactory{}

	t.Run("invalid-vote", func(t *testing.T) {
		proposal := helper.MakeProposal()
		mockedProcessor := &mockhotstuff.VerifyingVoteProcessor{}
		mockedProcessor.On("Process", proposal.ProposerVote()).Return(model.NewInvalidVoteErrorf(proposal.ProposerVote(), "")).Once()
		mockedFactory.On("Create", unittest.Logger(), proposal).Return(mockedProcessor, nil).Once()

		voteProcessorFactory := &VoteProcessorFactory{
			baseFactory: func(log zerolog.Logger, block *model.Block) (hotstuff.VerifyingVoteProcessor, error) {
				return mockedFactory.Create(log, proposal)
			},
		}

		processor, err := voteProcessorFactory.Create(unittest.Logger(), proposal)
		require.Error(t, err)
		require.Nil(t, processor)
		require.True(t, model.IsInvalidBlockError(err))

		mockedProcessor.AssertExpectations(t)
	})
	t.Run("process-vote-exception", func(t *testing.T) {
		proposal := helper.MakeProposal()
		mockedProcessor := &mockhotstuff.VerifyingVoteProcessor{}
		exception := errors.New("process-exception")
		mockedProcessor.On("Process", proposal.ProposerVote()).Return(exception).Once()

		mockedFactory.On("Create", unittest.Logger(), proposal).Return(mockedProcessor, nil).Once()

		voteProcessorFactory := &VoteProcessorFactory{
			baseFactory: func(log zerolog.Logger, block *model.Block) (hotstuff.VerifyingVoteProcessor, error) {
				return mockedFactory.Create(log, proposal)
			},
		}

		processor, err := voteProcessorFactory.Create(unittest.Logger(), proposal)
		require.ErrorIs(t, err, exception)
		require.Nil(t, processor)
		// an unexpected exception should _not_ be interpreted as the block being invalid
		require.False(t, model.IsInvalidBlockError(err))

		mockedProcessor.AssertExpectations(t)
	})

	mockedFactory.AssertExpectations(t)
}

// TestVoteProcessorFactory_CreateProcessException tests that VoteProcessorFactory correctly handles exception
// while creating processor for requested proposal.
func TestVoteProcessorFactory_CreateProcessException(t *testing.T) {
	mockedFactory := mockhotstuff.VoteProcessorFactory{}

	proposal := helper.MakeProposal()
	exception := errors.New("create-exception")

	mockedFactory.On("Create", unittest.Logger(), proposal).Return(nil, exception).Once()
	voteProcessorFactory := &VoteProcessorFactory{
		baseFactory: func(log zerolog.Logger, block *model.Block) (hotstuff.VerifyingVoteProcessor, error) {
			return mockedFactory.Create(log, proposal)
		},
	}

	processor, err := voteProcessorFactory.Create(unittest.Logger(), proposal)
	require.ErrorIs(t, err, exception)
	require.Nil(t, processor)
	// an unexpected exception should _not_ be interpreted as the block being invalid
	require.False(t, model.IsInvalidBlockError(err))

	mockedFactory.AssertExpectations(t)
}
