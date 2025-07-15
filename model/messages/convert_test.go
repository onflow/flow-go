package messages_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestClusterBlockProposal(t *testing.T) {
	block := unittest.ClusterBlockFixture()
	proposal := unittest.ClusterProposalFromBlock(block)
	proposalMsg := messages.UntrustedClusterProposalFromInternal(proposal)
	converted, err := proposalMsg.DeclareStructurallyValid()
	require.NoError(t, err)
	assert.Equal(t, proposal, converted)
}

func TestBlockResponse(t *testing.T) {
	expected := []*flow.Proposal{unittest.ProposalFixture(), unittest.ProposalFixture()}
	res := messages.BlockResponse{
		Blocks: []flow.UntrustedProposal{
			*flow.NewUntrustedProposal(expected[0]),
			*flow.NewUntrustedProposal(expected[1]),
		},
	}
	converted, err := res.BlocksInternal()
	require.NoError(t, err)
	assert.Equal(t, expected, converted)
}

func TestClusterBlockResponse(t *testing.T) {
	b1 := unittest.ClusterBlockFixture()
	b2 := unittest.ClusterBlockFixture()
	expected := []*cluster.Proposal{unittest.ClusterProposalFromBlock(b1), unittest.ClusterProposalFromBlock(b2)}
	res := messages.ClusterBlockResponse{
		Blocks: []messages.UntrustedClusterProposal{
			*messages.UntrustedClusterProposalFromInternal(expected[0]),
			*messages.UntrustedClusterProposalFromInternal(expected[1]),
		},
	}
	converted, err := res.BlocksInternal()
	require.NoError(t, err)
	assert.Equal(t, expected, converted)
}
