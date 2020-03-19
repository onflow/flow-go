// +build relic

package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/module"
)

// StakingSigner provides functions to generate and verify staking signatures.
// Specifically, it can generate and verify individual signatures (e.g. from a vote)
// and aggregated signatures (e.g. from a Quorum Certificate).
type StakingSigner struct {
	StakingSigVerifier
	viewState *hotstuff.ViewState
	me        module.Local
}

// NewStakingSigner creates an instance of StakingSigner
func NewStakingSigner(viewState *hotstuff.ViewState, stakingSigTag string, me module.Local) StakingSigner {
	return StakingSigner{
		StakingSigVerifier: NewStakingSigVerifier(stakingSigTag),
		viewState:          viewState,
		me:                 me,
	}
}

// Sign signs a the message with the node's staking key
func (s *StakingSigner) Sign(block *model.Block) (crypto.Signature, error) {
	msg := BlockToBytesForSign(block)
	return s.me.Sign(msg, s.stakingHasher)
}

// Aggregate aggregates the given signature that signed on the given block
// Implementation is OBLIVIOUS to RANDOM BEACON: RandomBeaconSignature in returned AggregatedSignature wil be nil.
// Inputs:
//    * block - it is needed in order to double check the reconstruct signature is valid
//    * And verifying the sig requires the signed message, which is the block
//    * sigs - the signatures to be aggregated. Assuming each signature has been verified already.
//
// Preconditions:
//    * each staking signature has been verified
//    * all signatures are from different parties
// Violating preconditions will result in an error (but not the construction of an invalid staking signature).
func (s *StakingSigner) Aggregate(block *model.Block, sigs []*model.SingleSignature) (*model.AggregatedSignature, error) {
	if len(sigs) == 0 { // ensure that sigs is not empty
		return nil, fmt.Errorf("cannot aggregate an empty slice of signatures")
	}

	aggStakingSig, signerIDs := unsafeAggregate(sigs)                    // unsafe aggregate staking sigs: crypto math only; will not catch error
	err := s.verifyAggregatedStakingSig(aggStakingSig, block, signerIDs) // safety: verify aggregated signature:
	if err != nil {
		return nil, fmt.Errorf("error aggregating staking signatures for block %s: %w", block.BlockID, err)
	}

	return &model.AggregatedSignature{
		StakingSignatures:     aggStakingSig,
		RandomBeaconSignature: nil,
		SignerIDs:             signerIDs,
	}, nil
}

// verifyAggregatedStakingSig verifies the aggregated staking signature as a sanity check.
// Errors on duplicated signers. Any error indicates an internal bug and results in a fatal error.
func (s *StakingSigner) verifyAggregatedStakingSig(aggStakingSig []crypto.Signature, block *model.Block, signerIDs []flow.Identifier) error {
	// This implementation will eventually be replaced by verifying the proper aggregated BLS signature:
	// Steps: (1) aggregate all the public keys
	//        (2) use aggregated public key to verify aggregated signature
	//
	// As we don't have signature aggregation implemented for now, we use the much more resource-intensive
	// way of verifying each signature individually
	stakedSigners, err := s.viewState.IdentitiesForConsensusParticipants(block.BlockID, signerIDs...)
	if err != nil {
		// if this happens, we have a bug in the calling logic
		return fmt.Errorf("constructed aggregated staking signature has invalid signers: %w", err)
	}
	// method IdentitiesForConsensusParticipants guarantees that there are no duplicated signers
	// and all signers are valid, staked consensus nodes

	// validate signature:
	// collect staking keys for for nodes that contributed to qc's aggregated sig:
	stakingPubKeys := make([]crypto.PublicKey, 0, len(stakedSigners))
	for _, signer := range stakedSigners {
		stakingPubKeys = append(stakingPubKeys, signer.StakingPubKey)
	}
	valid, err := s.VerifyStakingAggregatedSig(aggStakingSig, block, stakingPubKeys)
	if err != nil {
		return fmt.Errorf("error validating constructed aggregated staking signature: %w", err)
	}
	if !valid {
		return fmt.Errorf("constructed aggregated staking signature is invalid")
	}
	return nil
}

func unsafeAggregate(sigs []*model.SingleSignature) ([]crypto.Signature, []flow.Identifier) {
	// This implementation is a naive way of aggregation the signatures. It will work, with
	// the downside of costing more bandwidth.
	// The more optimal way, which is the real aggregation, will be implemented when the crypto
	// API is available.
	aggStakingSig := make([]crypto.Signature, len(sigs))
	signerIDs := make([]flow.Identifier, len(sigs))
	for i, sig := range sigs {
		aggStakingSig[i] = sig.StakingSignature
		signerIDs[i] = sig.SignerID
	}
	return aggStakingSig, signerIDs
}
