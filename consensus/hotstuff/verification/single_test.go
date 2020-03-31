package verification

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/helper"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestSingleProposal(t *testing.T) {

	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus))
	state, stakingKeys, _ := MakeProtocolState(t, identities, false)
	signers := MakeSigners(t, state, identities.NodeIDs(), stakingKeys, nil)

	// create proposal
	block := helper.MakeBlock(t, helper.WithBlockProposerID(identities[0].NodeID))
	proposal, err := signers[0].CreateProposal(block)
	require.NoError(t, err)

	// proposal should be valid
	valid, err := signers[0].VerifyProposal(proposal)
	require.NoError(t, err)
	assert.True(t, valid, "original proposal should be valid")

	// proposal with different block ID should be invalid
	proposal.Block.BlockID[0]++
	valid, err = signers[0].VerifyProposal(proposal)
	require.NoError(t, err)
	assert.False(t, valid, "proposal with changed block ID should be invalid")
	proposal.Block.BlockID[0]--

	// proposal with different view should be invalid
	proposal.Block.View++
	valid, err = signers[0].VerifyProposal(proposal)
	require.NoError(t, err)
	assert.False(t, valid, "proposal with changed view should be invalid")
	proposal.Block.View--

	// proposal from a different proposer should be invalid
	proposal.Block.ProposerID = identities[1].NodeID
	valid, err = signers[0].VerifyProposal(proposal)
	require.NoError(t, err)
	assert.False(t, valid, "proposal with changed proposer ID should be invalid")
	proposal.Block.ProposerID = identities[0].NodeID

	// proposal with invalid signature should be invalid
	proposal.Signature[0]++
	valid, err = signers[0].VerifyProposal(proposal)
	require.NoError(t, err)
	assert.False(t, valid, "proposal with changed signature should be invalid")
	proposal.Signature[0]--
}

func TestSingleVote(t *testing.T) {

	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus))
	state, stakingKeys, _ := MakeProtocolState(t, identities, false)
	signers := MakeSigners(t, state, identities.NodeIDs(), stakingKeys, nil)

	// create proposal
	block := helper.MakeBlock(t, helper.WithBlockProposerID(identities[2].NodeID))
	vote, err := signers[0].CreateVote(block)
	require.NoError(t, err)

	// vote should be valid
	valid, err := signers[0].VerifyVote(vote)
	require.NoError(t, err)
	assert.True(t, valid, "original vote should be valid")

	// vote on different block should be invalid
	vote.BlockID[0]++
	valid, err = signers[0].VerifyVote(vote)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed block ID should be invalid")
	vote.BlockID[0]--

	// vote with changed view should be invalid
	vote.View++
	valid, err = signers[0].VerifyVote(vote)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed view should be invalid")
	vote.View--

	// vote by different signer should be invalid
	vote.SignerID = identities[1].NodeID
	valid, err = signers[0].VerifyVote(vote)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed identity should be invalid")
	vote.SignerID = identities[0].NodeID

	// vote with changed signature should be invalid
	vote.Signature[0]++
	valid, err = signers[0].VerifyVote(vote)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed signature should be invalid")
	vote.Signature[0]--
}

func TestSingleProposalIsVote(t *testing.T) {

	// NOTE: I don't think this is true for every signature scheme

	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus))
	state, stakingKeys, _ := MakeProtocolState(t, identities, false)
	signers := MakeSigners(t, state, identities.NodeIDs(), stakingKeys, nil)

	// create proposal
	block := helper.MakeBlock(t, helper.WithBlockProposerID(identities[0].NodeID))
	proposal, err := signers[0].CreateProposal(block)
	require.NoError(t, err)
	vote, err := signers[0].CreateVote(block)
	require.NoError(t, err)

	assert.Equal(t, proposal.Signature, vote.Signature)
}

func TestSingleQC(t *testing.T) {

	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus))
	state, stakingKeys, _ := MakeProtocolState(t, identities, false)
	signers := MakeSigners(t, state, identities.NodeIDs(), stakingKeys, nil)

	// create proposal
	block := helper.MakeBlock(t, helper.WithBlockProposerID(identities[0].NodeID))
	var votes []*model.Vote
	for _, signer := range signers {
		vote, err := signer.CreateVote(block)
		require.NoError(t, err)
		votes = append(votes, vote)
	}

	// should be able to create QC from 4 votes and verify
	qc, err := signers[0].CreateQC(votes)
	require.NoError(t, err)
	valid, err := signers[0].VerifyQC(qc)
	require.NoError(t, err)
	assert.True(t, valid, "original QC should be valid")

	// creation from different views should fail
	votes[0].View++
	_, err = signers[0].CreateQC(votes)
	assert.Error(t, err, "creating QC with mismatching view should fail")
	votes[0].View--

	// creation from different block IDs should fail
	votes[0].BlockID[0]++
	_, err = signers[0].CreateQC(votes)
	assert.Error(t, err, "creating QC with mismatching block ID should fail")
	votes[0].BlockID[0]--
}
