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
	proposal := messages.NewBlockProposalFromInternal(&block)
	assert.Equal(t, block, proposal.Block)
}

func TestClusterBlockProposal(t *testing.T) {
	block := unittest.ClusterBlockFixture()
	proposal := messages.NewClusterBlockProposalFromInternal(&block)
	assert.Equal(t, block, proposal.Block)
}

func TestBlockResponse(t *testing.T) {
	expected := unittest.BlockFixtures(2)
	res := messages.BlockResponse{
		Blocks: []flow.Block{
			*expected[0],
			*expected[1],
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
		Blocks: []cluster.Block{
			b1,
			b2,
		},
	}
	converted := res.BlocksInternal()
	assert.Equal(t, expected, converted)
}
