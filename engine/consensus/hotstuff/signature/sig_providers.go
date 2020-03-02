package signature


import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/module"
)

// RandomBeaconAwareSigProvider implements the interfaces:
//   * hotstuff.SigVerifier
//   * hotstuff.SigAggregator
//   * hotstuff.Signer
// Implementation fully supports verifying, aggregating and signing of random beacon signature shares and
// the reconstructed threshold signatures.
// This implementation of SigVerifier is intended for use by full consensus nodes.
type RandomBeaconAwareSigProvider struct {
	StakingSigProvider
	RandomBeaconSigner
}

func NewRandomBeaconAwareSigProvider(
	viewState *hotstuff.ViewState,
	stakingSigTag string,
	me module.Local,
	randomBeaconPrivateKey crypto.PrivateKey,
) hotstuff.SigAggregator {
	return &RandomBeaconAwareSigProvider{
		StakingSigProvider:      NewStakingSigProvider(viewState, stakingSigTag, me),
		RandomBeaconSigner: NewRandomBeaconSigner(viewState, randomBeaconPrivateKey),
	}
}

// VerifyRandomBeaconSig verifies a random beacon signature share for a block using provided signer's public key
func (p *RandomBeaconAwareSigProvider) VerifyRandomBeaconSig(sigShare crypto.Signature, block *model.Block, signerPubKey crypto.PublicKey) (bool, error) {
	return p.RandomBeaconSigner.VerifyRandomBeaconSig(sigShare, block, signerPubKey)
}

// VerifyRandomBeaconThresholdSig is OBLIVIOUS to RANDOM BEACON; returns always true
func (p *RandomBeaconAwareSigProvider) VerifyRandomBeaconThresholdSig(sig crypto.Signature, block *model.Block, groupPubKey crypto.PublicKey) (bool, error) {
	return p.RandomBeaconSigner.VerifyRandomBeaconThresholdSig(sig, block, groupPubKey)
}

// Aggregate aggregates the given signature that signed on the given block
func (p *RandomBeaconAwareSigProvider) Aggregate(block *model.Block, sigs []*model.SingleSignature) (*model.AggregatedSignature, error) {
	aggSig, err := p.StakingSigProvider.Aggregate(block, sigs) // generate staking part of AggregatedSignature
	if err != nil {
		return nil, fmt.Errorf("cannot aggregate staking signatures: %w", err)
	}

	// construct Random Beacon Threshold signature:
	sigShares, err := p.getSignerIDsAndSigShares(block.BlockID, sigs)
	if err != nil {
		return nil, fmt.Errorf("cannot get random beacon key shares: %w", err)
	}
	// reconstruct random beacon sig
	msg := BlockToBytesForSign(block)
	reconstructedRandomBeaconSig, err := p.RandomBeaconSigner.Reconstruct(msg, sigShares)
	if err != nil {
		return nil, fmt.Errorf("cannot reconstruct random beacon sig: %w", err)
	}

	aggSig.RandomBeaconSignature = reconstructedRandomBeaconSig // add Random Beacon Threshold signature to aggregated sig
	return aggSig, nil
}

// getSignerIDsAndSigShares converts signerIDs into random beacon pubkey shares
func (p *RandomBeaconAwareSigProvider) getSignerIDsAndSigShares(blockID flow.Identifier, sigs []*model.SingleSignature) ([]*SigShare, error) {
	if len(sigs) == 0 { // sanity check
		return nil, fmt.Errorf("signatures should not be empty")
	}

	// lookup signer by signer ID and make SigShare
	sigShares := make([]*SigShare, len(sigs), 0)
	for _, sig := range sigs {
		// TODO: confirm if combining into one query is possible and faster
		signer, err := p.viewState.IdentityForConsensusParticipant(blockID, sig.SignerID)
		if err != nil {
			return nil, fmt.Errorf("cannot get identity for ID %s: %w", sig.SignerID, err)
		}

		sigShare := SigShare{
			Signature:    sig.RandomBeaconSignature,
			SignerPubKey: signer.RandomBeaconPubKey,
		}
		sigShares[i] = &sigShare
	}

	return sigShares, nil
}


// CanReconstruct checks whether the given sig shares are enough to reconstruct the threshold signature.
// It assumes the DKG group size never change.
func (p *RandomBeaconAwareSigProvider) CanReconstruct(numOfSigShares int) bool {
	return p.RandomBeaconSigner.CanReconstruct(numOfSigShares)
}

// VoteFor signs a Block and returns the Vote for that Block.
// Implementation is OBLIVIOUS to RANDOM BEACON! The RandomBeaconSignature in the return vote wil be nil.
func (p *RandomBeaconAwareSigProvider) VoteFor(block *model.Block) (*model.Vote, error) {
	msg := BlockToBytesForSign(block)                  // convert to bytes to be signed
	stakingSig, err := p.me.Sign(msg, p.stakingHasher) // generate staking signature
	if err != nil {
		return nil, fmt.Errorf("vote generation failed: error signing block %p: %w", block.BlockID, err)
	}
	sig := model.SingleSignature{
		StakingSignature:      stakingSig,
		RandomBeaconSignature: nil,
		SignerID:              p.me.NodeID(),
	}
	return &model.Vote{
		BlockID:   block.BlockID,
		View:      block.View,
		Signature: &sig,
	}, nil
}

// Propose signs a Block and returns the block as model.Proposal.
// Implementation is OBLIVIOUS to RANDOM BEACON! The RandomBeaconSignature in the return Proposal wil be nil.
func (p *RandomBeaconAwareSigProvider) Propose(block *model.Block) (*model.Proposal, error) {
	msg := BlockToBytesForSign(block)                  // convert to bytes to be signed
	stakingSig, err := p.me.Sign(msg, p.stakingHasher) // generate staking signature
	if err != nil {
		return nil, fmt.Errorf("proposal generation failed: error signing block %p: %w", block.BlockID, err)
	}
	return &model.Proposal{
		Block:                 block,
		StakingSignature:      stakingSig,
		RandomBeaconSignature: nil,
	}, nil
}





// StakingSigProvider implements the interfaces:
//   * hotstuff.SigVerifier
//   * hotstuff.SigAggregator
//   * hotstuff.Signer
// Implementation is OBLIVIOUS to RANDOM BEACON! Verification of any Random-Beacon related signatures will _always_ succeed.
// This implementation is intended for use in the collector's cluster-internal consensus.
type StakingSigProvider struct {
	StakingSigner
}

func NewStakingSigProvider(viewState *hotstuff.ViewState, stakingSigTag string, me module.Local) StakingSigProvider {
	return StakingSigProvider{
		StakingSigner: NewStakingSigner(viewState, stakingSigTag, me),
	}
}

// VerifyRandomBeaconSig is OBLIVIOUS to RANDOM BEACON; returns always true
func (p *StakingSigProvider) VerifyRandomBeaconSig(sigShare crypto.Signature, block *model.Block, signerPubKey crypto.PublicKey) (bool, error) {
	return true, nil
}

// VerifyRandomBeaconThresholdSig is OBLIVIOUS to RANDOM BEACON; returns always true
func (p *StakingSigProvider) VerifyRandomBeaconThresholdSig(sig crypto.Signature, block *model.Block, groupPubKey crypto.PublicKey) (bool, error) {
	return true, nil
}

// CanReconstruct is OBLIVIOUS to RANDOM BEACON; returns always true
func (p *StakingSigProvider) CanReconstruct(numOfSigShares int) bool {
	return true
}

// VoteFor signs a Block and returns the Vote for that Block.
// Implementation is OBLIVIOUS to RANDOM BEACON! The RandomBeaconSignature in the return vote wil be nil.
func (p *StakingSigProvider) VoteFor(block *model.Block) (*model.Vote, error) {
	msg := BlockToBytesForSign(block)                  // convert to bytes to be signed
	stakingSig, err := p.me.Sign(msg, p.stakingHasher) // generate staking signature
	if err != nil {
		return nil, fmt.Errorf("vote generation failed: error signing block %p: %w", block.BlockID, err)
	}
	sig := model.SingleSignature{
		StakingSignature:      stakingSig,
		RandomBeaconSignature: nil,
		SignerID:              p.me.NodeID(),
	}
	return &model.Vote{
		BlockID:   block.BlockID,
		View:      block.View,
		Signature: &sig,
	}, nil
}

// Propose signs a Block and returns the block as model.Proposal.
// Implementation is OBLIVIOUS to RANDOM BEACON! The RandomBeaconSignature in the return Proposal wil be nil.
func (p *StakingSigProvider) Propose(block *model.Block) (*model.Proposal, error) {
	msg := BlockToBytesForSign(block)                  // convert to bytes to be signed
	stakingSig, err := p.me.Sign(msg, p.stakingHasher) // generate staking signature
	if err != nil {
		return nil, fmt.Errorf("proposal generation failed: error signing block %p: %w", block.BlockID, err)
	}
	return &model.Proposal{
		Block:                 block,
		StakingSignature:      stakingSig,
		RandomBeaconSignature: nil,
	}, nil
}
