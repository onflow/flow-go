package messages_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/cluster"
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
	expected := unittest.BlockFixtures(2)
	res := messages.BlockResponse{
		Blocks: []messages.UntrustedBlock{
			messages.UntrustedBlockFromInternal(expected[0]),
			messages.UntrustedBlockFromInternal(expected[1]),
		},
	}
	converted := res.BlocksInternal()
	assert.Equal(t, expected, converted)
}

func TestClusterBlockResponse(t *testing.T) {
	b1 := unittest.ClusterBlockFixture()
	b2 := unittest.ClusterBlockFixture()
	expected := []*cluster.Block{&b1, &b2}
	res := messages.ClusterBlockResponse{
		Blocks: []messages.UntrustedClusterBlock{
			messages.UntrustedClusterBlockFromInternal(expected[0]),
			messages.UntrustedClusterBlockFromInternal(expected[1]),
		},
	}
	converted := res.BlocksInternal()
	assert.Equal(t, expected, converted)
}
