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

func TestCombinedVote(t *testing.T) {

	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus))
	committeeState, dkg, stakingKeys, beaconKeys := MakeHotstuffCommitteeState(t, identities, true)
	signers := MakeSigners(t, committeeState, dkg, identities.NodeIDs(), stakingKeys, beaconKeys)

	// create proposal
	block := helper.MakeBlock(t, helper.WithBlockProposer(identities[2].NodeID))
	vote, err := signers[0].CreateVote(block)
	require.NoError(t, err)

	// vote should be valid
	valid, err := signers[0].VerifyVote(vote.SignerID, vote.SigData, block)
	require.NoError(t, err)
	assert.True(t, valid, "original vote should be valid")

	// vote on different block should be invalid
	block.BlockID[0]++
	valid, err = signers[0].VerifyVote(vote.SignerID, vote.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed block ID should be invalid")
	block.BlockID[0]--

	// vote with changed view should be invalid
	block.View++
	valid, err = signers[0].VerifyVote(vote.SignerID, vote.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed view should be invalid")
	block.View--

	// vote by different signer should be invalid
	vote.SignerID = identities[1].NodeID
	valid, err = signers[0].VerifyVote(vote.SignerID, vote.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed identity should be invalid")
	vote.SignerID = identities[0].NodeID

	// vote with changed signature should be invalid
	// TODO: change error handling so split failure and invalid signature is
	// treated the same
	vote.SigData[4]++
	valid, err = signers[0].VerifyVote(vote.SignerID, vote.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "vote with changed signature should be invalid")
	vote.SigData[4]--
}

func TestCombinedProposalIsVote(t *testing.T) {

	// NOTE: I don't think this is true for every signature scheme

	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus))
	committeeState, dkg, stakingKeys, beaconKeys := MakeHotstuffCommitteeState(t, identities, true)
	signers := MakeSigners(t, committeeState, dkg, identities.NodeIDs(), stakingKeys, beaconKeys)

	// create proposal
	block := helper.MakeBlock(t, helper.WithBlockProposer(identities[0].NodeID))
	proposal, err := signers[0].CreateProposal(block)
	require.NoError(t, err)
	vote, err := signers[0].CreateVote(block)
	require.NoError(t, err)

	assert.Equal(t, proposal.SigData, vote.SigData)
}

func TestCombinedQC(t *testing.T) {

	identities := unittest.IdentityListFixture(8, unittest.WithRole(flow.RoleConsensus))
	voterIDs := identities.NodeIDs()
	minShares := (len(voterIDs)-1) / 2 + 1
	committeeState, dkg, stakingKeys, beaconKeys := MakeHotstuffCommitteeState(t, identities, true)
	signers := MakeSigners(t, committeeState, dkg, identities.NodeIDs(), stakingKeys, beaconKeys)

	// create proposal
	block := helper.MakeBlock(t, helper.WithBlockProposer(identities[0].NodeID))
	var votes []*model.Vote
	for _, signer := range signers {
		vote, err := signer.CreateVote(block)
		require.NoError(t, err)
		votes = append(votes, vote)
	}

	// should be able to create QC from 4 votes and verify
	qc, err := signers[0].CreateQC(votes[:minShares])
	require.NoError(t, err, "should be able to create QC from valid votes")

	// creation with insufficient threshold should fail
	_, err = signers[0].CreateQC(votes[:minShares-1])
	assert.Error(t, err, "creating QC with insufficient shares should fail")

	// creation from different views should fail
	votes[0].View++
	_, err = signers[0].CreateQC(votes[:minShares])
	assert.Error(t, err, "creating QC with mismatching view should fail")
	votes[0].View--

	// creation from different block IDs should fail
	votes[0].BlockID[0]++
	_, err = signers[0].CreateQC(votes[:minShares])
	assert.Error(t, err, "creating QC with mismatching block ID should fail")
	votes[0].BlockID[0]--

	valid, err := signers[0].VerifyQC(voterIDs[:minShares], qc.SigData, block)
	require.NoError(t, err)
	assert.True(t, valid, "original QC should be valid")

	// verification with missing identity should be invalid
	_, err = signers[0].VerifyQC(voterIDs[:minShares-1], qc.SigData, block)
	assert.Error(t, err, "verification of QC should fail with missing voter ID")

	// verification with changed signature should fail
	// TODO: change error handling so split failure & invalid signature is
	// treated the same
	qc.SigData[8]++
	valid, err = signers[0].VerifyQC(voterIDs[:minShares], qc.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "QC with changed signature data should be invalid")
	qc.SigData[8]--

	// verification with changed block ID should fail
	block.BlockID[0]++
	valid, err = signers[0].VerifyQC(voterIDs[:minShares], qc.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "QC with changed block ID should be invalid")
	block.BlockID[0]--

	// verification with changed view should fail
	block.View++
	valid, err = signers[0].VerifyQC(voterIDs[:minShares], qc.SigData, block)
	require.NoError(t, err)
	assert.False(t, valid, "QC with changed block view should be invalid")
	block.View--
}
