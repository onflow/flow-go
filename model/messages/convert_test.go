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
	proposalMsg := messages.NewBlockProposal(proposal)
	converted := proposalMsg.ToInternal()
	assert.Equal(t, proposal, converted)
}

func TestClusterBlockProposal(t *testing.T) {
	block := unittest.ClusterBlockFixture()
	proposal := unittest.ClusterProposalFromBlock(&block)
	proposalMsg := messages.ClusterBlockProposalFrom(proposal)
	converted := proposalMsg.ToInternal()
	assert.Equal(t, proposal, converted)
}

func TestBlockResponse(t *testing.T) {
	expected := []*flow.BlockProposal{unittest.ProposalFixture(), unittest.ProposalFixture()}
	res := messages.BlockResponse{
		Blocks: []messages.BlockProposal{
			*messages.NewBlockProposal(expected[0]),
			*messages.NewBlockProposal(expected[1]),
		},
	}
	converted := res.BlocksInternal()
	assert.Equal(t, expected, converted)
}

func TestClusterBlockResponse(t *testing.T) {
	b1 := unittest.ClusterBlockFixture()
	b2 := unittest.ClusterBlockFixture()
	expected := []*cluster.BlockProposal{unittest.ClusterProposalFromBlock(&b1), unittest.ClusterProposalFromBlock(&b2)}
	res := messages.ClusterBlockResponse{
		Blocks: []messages.ClusterBlockProposal{
			*messages.ClusterBlockProposalFrom(expected[0]),
			*messages.ClusterBlockProposalFrom(expected[1]),
		},
	}
	converted := res.BlocksInternal()
	assert.Equal(t, expected, converted)
}
