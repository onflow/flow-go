package messages_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockProposal(t *testing.T) {
	block := unittest.FullBlockFixture()
	proposal := unittest.ProposalFromBlock(&block)
	proposalMsg := messages.NewUntrustedProposal(proposal)
	converted := proposalMsg.ToInternal()
	assert.Equal(t, proposal, converted)
}

func TestClusterBlockProposal(t *testing.T) {
	block := unittest.ClusterBlockFixture()
	proposal := unittest.ClusterProposalFromBlock(block)
	proposalMsg := messages.UntrustedClusterProposalFromInternal(proposal)
	converted := proposalMsg.ToInternal()
	assert.Equal(t, proposal, converted)
}

func TestBlockResponse(t *testing.T) {
	expected := []*flow.BlockProposal{unittest.ProposalFixture(), unittest.ProposalFixture()}
	res := messages.BlockResponse{
		Blocks: []messages.UntrustedProposal{
			*messages.NewUntrustedProposal(expected[0]),
			*messages.NewUntrustedProposal(expected[1]),
		},
	}
	converted := res.BlocksInternal()
	assert.Equal(t, expected, converted)
}

func TestClusterBlockResponse(t *testing.T) {
	b1 := unittest.ClusterBlockFixture()
	b2 := unittest.ClusterBlockFixture()
	expected := []*cluster.BlockProposal{unittest.ClusterProposalFromBlock(b1), unittest.ClusterProposalFromBlock(b2)}
	res := messages.ClusterBlockResponse{
		Blocks: []messages.UntrustedClusterProposal{
			*messages.UntrustedClusterProposalFromInternal(expected[0]),
			*messages.UntrustedClusterProposalFromInternal(expected[1]),
		},
	}
	converted := res.BlocksInternal()
	assert.Equal(t, expected, converted)
}
