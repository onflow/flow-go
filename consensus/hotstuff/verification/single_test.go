package verification

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSingleVote(t *testing.T) {

	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleCollection))
	committeeState, stakingKeys, _ := MakeHotstuffCommitteeState(t, identities, false, epochCounter)
	signers := MakeSigners(t, identities.NodeIDs(), stakingKeys, nil, encoding.CollectorVoteTag)
	verifier := NewSingleVerifier(
		committeeState,
		signature.NewAggregationVerifier(encoding.CollectorVoteTag),
	)

	// create proposal
	block := helper.MakeBlock(helper.WithBlockProposer(identities[2].NodeID))
	vote, err := signers[0].CreateVote(block)
	require.NoError(t, err)
	voter := identities[0]

	// vote should be valid
	valid, err := verifier.VerifyVote(voter, vote.SigData, block)
	require.NoError(t, err)
	assert.True(t, valid, "original vote should be valid")

	// vote on different block should be invalid
	block.BlockID[0]++
	valid, err = verifier.VerifyVote(voter, vote.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed block ID should be invalid")
	block.BlockID[0]--

	// vote with changed view should be invalid
	block.View++
	valid, err = verifier.VerifyVote(voter, vote.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed view should be invalid")
	block.View--

	// vote by different signer should be invalid
	voter = identities[1]
	valid, err = verifier.VerifyVote(voter, vote.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed identity should be invalid")
	voter = identities[0]

	// vote with changed signature should be invalid
	vote.SigData[0]++
	valid, err = verifier.VerifyVote(voter, vote.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed signature should be invalid")
	vote.SigData[0]--
}

func TestSingleProposalIsVote(t *testing.T) {

	// NOTE: I don't think this is true for every signature scheme

	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleCollection))
	_, stakingKeys, _ := MakeHotstuffCommitteeState(t, identities, false, epochCounter)
	signers := MakeSigners(t, identities.NodeIDs(), stakingKeys, nil, encoding.CollectorVoteTag)

	// create proposal
	block := helper.MakeBlock(helper.WithBlockProposer(identities[0].NodeID))
	proposal, err := signers[0].CreateProposal(block)
	require.NoError(t, err)
	vote, err := signers[0].CreateVote(block)
	require.NoError(t, err)

	assert.Equal(t, proposal.SigData, vote.SigData)
}
