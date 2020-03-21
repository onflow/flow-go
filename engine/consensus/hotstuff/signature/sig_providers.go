package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/model/encoding"
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
	me module.Local,
	randomBeaconPrivateKey crypto.PrivateKey,
) RandomBeaconAwareSigProvider {
	// TODO: ensure viewState.DKGPublicData() != nil
	return RandomBeaconAwareSigProvider{
		// encoding.ConsensusVoteTag is used here since only consensus node will use random beacon sig
		StakingSigProvider: NewStakingSigProvider(viewState, encoding.ConsensusVoteTag, me),
		RandomBeaconSigner: NewRandomBeaconSigner(viewState, randomBeaconPrivateKey),
	}
}

// VerifyRandomBeaconSig verifies a random beacon signature share for a block using provided signer's public key.
func (p *RandomBeaconAwareSigProvider) VerifyRandomBeaconSig(sigShare crypto.Signature, block *model.Block, signerPubKey crypto.PublicKey) (bool, error) {
	return p.RandomBeaconSigner.VerifyRandomBeaconSig(sigShare, block, signerPubKey)
}

// VerifyRandomBeaconThresholdSig verifies a (reconstructed) random beacon threshold signature for a block.
func (p *RandomBeaconAwareSigProvider) VerifyRandomBeaconThresholdSig(sig crypto.Signature, block *model.Block, groupPubKey crypto.PublicKey) (bool, error) {
	return p.RandomBeaconSigner.VerifyRandomBeaconThresholdSig(sig, block, groupPubKey)
}

// Aggregate aggregates the provided signatures.
func (p *RandomBeaconAwareSigProvider) Aggregate(block *model.Block, sigs []*model.SingleSignature) (*model.AggregatedSignature, error) {
	aggSig, err := p.StakingSigProvider.Aggregate(block, sigs) // generate staking part of AggregatedSignature
	if err != nil {
		return nil, fmt.Errorf("cannot aggregate staking signatures: %w", err)
	}
	// construct Random Beacon Threshold signature:
	reconstructedRandomBeaconSig, err := p.RandomBeaconSigner.Reconstruct(block, sigs)
	if err != nil {
		return nil, fmt.Errorf("cannot reconstruct random beacon sig: %w", err)
	}
	aggSig.RandomBeaconSignature = reconstructedRandomBeaconSig // add Random Beacon Threshold signature to aggregated sig
	return aggSig, nil
}

// CanReconstruct returns if the given number of signature shares is enough to reconstruct the random beaccon sigs
func (p *RandomBeaconAwareSigProvider) CanReconstruct(numOfSigShares int) bool {
	return p.RandomBeaconSigner.CanReconstruct(numOfSigShares)
}

// VoteFor signs a Block and returns the Vote for that Block. Returned vote includes proper Random Beacon sig share.
func (p *RandomBeaconAwareSigProvider) VoteFor(block *model.Block) (*model.Vote, error) {
	vote, err := p.StakingSigProvider.VoteFor(block) // generate part of vote with staking signature only
	if err != nil {
		return nil, fmt.Errorf("generating staking signature for vote failed: %w", err)
	}
	// generate random beacon signature
	randomBeaconShare, err := p.RandomBeaconSigner.Sign(block)
	if err != nil {
		return nil, fmt.Errorf("generating random beacon signature for vote failed: %w", err)
	}
	vote.Signature.RandomBeaconSignature = randomBeaconShare
	return vote, nil
}

// Propose signs a Block and returns the block Proposal. Returned proposal includes proper Random Beacon sig share.
func (p *RandomBeaconAwareSigProvider) Propose(block *model.Block) (*model.Proposal, error) {
	proposal, err := p.StakingSigProvider.Propose(block) // generate part of vote with staking signature only
	if err != nil {
		return nil, fmt.Errorf("generating staking signature for proposal failed: %w", err)
	}
	// generate random beacon signature
	randomBeaconShare, err := p.RandomBeaconSigner.Sign(block)
	if err != nil {
		return nil, fmt.Errorf("generating random beacon signature for proposal failed: %w", err)
	}
	proposal.RandomBeaconSignature = randomBeaconShare
	return proposal, nil
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
	stakingSig, err := p.Sign(block) // generate staking signature
	if err != nil {
		return nil, fmt.Errorf("vote generation failed: error signing block %s: %w", block.BlockID, err)
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
	stakingSig, err := p.Sign(block) // generate staking signature
	if err != nil {
		return nil, fmt.Errorf("proposal generation failed: error signing block %s: %w", block.BlockID, err)
	}
	return &model.Proposal{
		Block:                 block,
		StakingSignature:      stakingSig,
		RandomBeaconSignature: nil,
	}, nil
}
