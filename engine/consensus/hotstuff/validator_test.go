package hotstuff

import "testing"

func TestValidateVote(t *testing.T) {
	// Happy Path
	t.Run("A valid vote should be valid", testVoteOK)
	// Unhappy Path
	// t.Run("A vote with invalid view should be rejected", testVoteInvalidView)
	// t.Run("A vote with invalid block ID should be rejected", testVoteInvalidBlock)
	// t.Run("A vote from unstaked node should be rejected", testVoteUnstakedNode)
	// t.Run("A vote with invalid staking sig should be rejected", testVoteInvalidStaking)
	// t.Run("A vote with invalid random beacon sig should be rejected", testVoteInvalidRandomB)
}

// func TestValidateQC(t *testing.T) {
// 	// Happy Path
// 	t.Run("A valid QC should be valid", testQCOK)
// 	// Unhappy Path
// 	t.Run("A QC with invalid blockID should be rejected", testQCInvalidBlock)
// 	t.Run("A QC with invalid view should be rejected", testQCInvalidView)
// 	t.Run("A QC from unstaked nodes should be rejected", testQCHasUnstakedSigner)
// 	t.Run("A QC from duplicated nodes should be rejected", testQCHasDuplicatedSigner)
// 	t.Run("A QC with insufficient stakes should be rejected", testQCHasInsufficentStake)
// 	t.Run("A QC with invalid staking sig should be rejected", testQCHasInvalidStakingSig)
// 	t.Run("A QC with invalid random beacon sig should be rejected", testQCHasInvalidRandomBSig)
// }
//
// func TestValidateProposal(t *testing.T) {
// 	// Happy Path
// 	t.Run("A valid proposal should be accepted", testProposalOK)
// 	// Unhappy Path
// 	t.Run("A proposal with invalid view should be rejected", testProposalInvalidView)
// 	t.Run("A proposal with invalid block ID should be rejected", testProposalInvalidBlock)
// 	t.Run("A proposal from unstaked node should be rejected", testProposalUnstakedNode)
// 	t.Run("A proposal with invalid staking sig should be rejected", testProposalInvalidStaking)
// 	t.Run("A proposal with invalid random beacon sig should be rejected", testProposalInvalidRandomB)
// 	t.Run("A proposal from the wrong leader should be rejected", testProposalWrongLeader)
// 	t.Run("A proposal with a QC pointing to a non-existing block, but equal to finalized view should be rejected", testProposalWrongParentEqual)
// 	t.Run("A proposal with a QC pointing to a non-existing block, but above finalized view should be rejected", testProposalWrongParentAbove)
// 	t.Run("A proposal with a QC pointing to a non-existing block, but below finalized view should be unverifiable", testProposalWrongParentBelow)
// 	t.Run("A proposal with a invalid QC should be rejected", testProposalInvalidQC)
// }

func testVoteOK(t *testing.T) {
}
