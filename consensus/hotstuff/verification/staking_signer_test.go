package verification

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/module/local"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestStakingSigner_CreateProposal verifies that StakingSigner can produce correctly signed proposal
// that can be verified later using StakingVerifier.
// Additionally, we check cases where errors during signing are happening.
func TestStakingSigner_CreateProposal(t *testing.T) {
	stakingPriv := unittest.StakingPrivKeyFixture()
	signer := unittest.IdentityFixture()
	signerID := signer.NodeID
	signer.StakingPubKey = stakingPriv.PublicKey()

	t.Run("invalid-signer-id", func(t *testing.T) {
		me := &modulemock.Local{}
		me.On("NodeID").Return(signerID)
		signer := NewStakingSigner(me)

		block := helper.MakeBlock()
		proposal, err := signer.CreateProposal(block)
		require.Error(t, err)
		require.Nil(t, proposal)
	})
	t.Run("could-not-sign", func(t *testing.T) {
		signException := errors.New("sign-exception")
		me := &modulemock.Local{}
		me.On("NodeID").Return(signerID)
		me.On("Sign", mock.Anything, mock.Anything).Return(nil, signException).Once()
		signer := NewStakingSigner(me)

		block := helper.MakeBlock()
		proposal, err := signer.CreateProposal(block)
		require.ErrorAs(t, err, &signException)
		require.Nil(t, proposal)
	})
	t.Run("created-proposal", func(t *testing.T) {
		me, err := local.New(signer, stakingPriv)
		require.NoError(t, err)

		signerIdentity := unittest.IdentityFixture(unittest.WithNodeID(signerID),
			unittest.WithStakingPubKey(stakingPriv.PublicKey()))

		signer := NewStakingSigner(me)

		block := helper.MakeBlock(helper.WithBlockProposer(signerID))
		proposal, err := signer.CreateProposal(block)
		require.NoError(t, err)
		require.NotNil(t, proposal)

		verifier := NewStakingVerifier()
		err = verifier.VerifyVote(signerIdentity, proposal.SigData, proposal.Block)
		require.NoError(t, err)
	})
}

// TestStakingSigner_CreateVote verifies that StakingSigner can produce correctly signed vote
// that can be verified later using StakingVerifier.
// Additionally, we check cases where errors during signing are happening.
func TestStakingSigner_CreateVote(t *testing.T) {
	stakingPriv := unittest.StakingPrivKeyFixture()
	signer := unittest.IdentityFixture()
	signer.StakingPubKey = stakingPriv.PublicKey()
	signerID := signer.NodeID

	t.Run("could-not-sign", func(t *testing.T) {
		signException := errors.New("sign-exception")
		me := &modulemock.Local{}
		me.On("NodeID").Return(signerID)
		me.On("Sign", mock.Anything, mock.Anything).Return(nil, signException).Once()
		signer := NewStakingSigner(me)

		block := helper.MakeBlock()
		proposal, err := signer.CreateProposal(block)
		require.ErrorAs(t, err, &signException)
		require.Nil(t, proposal)
	})
	t.Run("created-vote", func(t *testing.T) {
		me, err := local.New(signer, stakingPriv)
		require.NoError(t, err)

		signerIdentity := unittest.IdentityFixture(unittest.WithNodeID(signerID),
			unittest.WithStakingPubKey(stakingPriv.PublicKey()))

		signer := NewStakingSigner(me)

		block := helper.MakeBlock(helper.WithBlockProposer(signerID))
		vote, err := signer.CreateVote(block)
		require.NoError(t, err)
		require.NotNil(t, vote)

		verifier := NewStakingVerifier()
		err = verifier.VerifyVote(signerIdentity, vote.SigData, block)
		require.NoError(t, err)
	})
}
