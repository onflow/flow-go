package verification

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCombinedVote(t *testing.T) {

	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus))
	committeeState, stakingKeys, beaconKeys := MakeHotstuffCommitteeState(t, identities, true, epochCounter)
	signers := MakeSigners(t, committeeState, identities.NodeIDs(), stakingKeys, beaconKeys)
	packer := signature.NewConsensusSigDataPacker(committeeState)
	verifier := NewCombinedVerifier(committeeState, packer)

	// create proposal
	block := helper.MakeBlock(helper.WithBlockProposer(identities[2].NodeID))
	vote, err := signers[0].CreateVote(block)
	voter := identities[0]
	require.NoError(t, err)

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
	// TODO: change error handling so split failure and invalid signature is
	// treated the same
	vote.SigData[4]++
	valid, err = verifier.VerifyVote(voter, vote.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed signature should be invalid")
	vote.SigData[4]--
}

func TestCombinedProposalIsVote(t *testing.T) {

	// NOTE: I don't think this is true for every signature scheme
	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus))
	committeeState, stakingKeys, beaconKeys := MakeHotstuffCommitteeState(t, identities, true, epochCounter)
	signers := MakeSigners(t, committeeState, identities.NodeIDs(), stakingKeys, beaconKeys)

	// create proposal
	block := helper.MakeBlock(helper.WithBlockProposer(identities[0].NodeID))
	proposal, err := signers[0].CreateProposal(block)
	require.NoError(t, err)
	vote, err := signers[0].CreateVote(block)
	require.NoError(t, err)

	assert.Equal(t, proposal.SigData, vote.SigData)
}
