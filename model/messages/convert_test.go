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

func TestBlockResponse(t *testing.T) {
	expected := []*flow.Proposal{unittest.ProposalFixture(), unittest.ProposalFixture()}
	res := messages.BlockResponse{
		Blocks: []flow.UntrustedProposal{
			flow.UntrustedProposal(*expected[0]),
			flow.UntrustedProposal(*expected[1]),
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
		Blocks: []cluster.UntrustedProposal{
			cluster.UntrustedProposal(*expected[0]),
			cluster.UntrustedProposal(*expected[1]),
		},
	}
	converted, err := res.BlocksInternal()
	require.NoError(t, err)
	assert.Equal(t, expected, converted)
}
