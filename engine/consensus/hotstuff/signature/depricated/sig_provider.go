// +build relic

package dep

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/protocol"
)

// SigProvider provides symmetry functions to generate and verify signatures
type SigProvider struct {
	myID                   flow.Identifier
	protocolState          protocol.State
	stakingPrivateKey      crypto.PrivateKey // the staking private key
	stakingHasher          crypto.Hasher     // the hasher for signing staking signature
	randomBeaconPrivateKey crypto.PrivateKey // the private key for signing random beacon signature
	randomBeaconHasher     crypto.Hasher     // the hasher for signer random beacon signature
	dkgPubData             *DKGPublicData    // the dkg public data for the only epoch. Should be returned by protocol state if we implement epoch switch
	isRandomBeaconEnabled  bool              // indicates whether random beacon signature is enabled for the cluster
}

// NewSigProvider creates an instance of SigProvider
func NewSigProvider(
	myID flow.Identifier,
	protocolState protocol.State,
	stakingPrivateKey crypto.PrivateKey,
	stakingHasher crypto.Hasher,
	dkgPubData *DKGPublicData,
	randomBeaconPrivateKey crypto.PrivateKey,
	randomBeaconHasher crypto.Hasher,
	isRandomBeaconEnabled bool,
) *SigProvider {
	return &SigProvider{
		myID:                   myID,
		protocolState:          protocolState,
		stakingPrivateKey:      stakingPrivateKey,
		stakingHasher:          stakingHasher,
		dkgPubData:             dkgPubData,
		randomBeaconPrivateKey: randomBeaconPrivateKey,
		randomBeaconHasher:     randomBeaconHasher,
		isRandomBeaconEnabled:  isRandomBeaconEnabled,
	}
}

// VerifyStakingSig verifies a single BLS signature for a block using the given public key
// sig - the signature to be verified
// block - the block that the signature was signed for.
// signerKey - the public key of the signer who signed the block.
func (s *SigProvider) VerifyStakingSig(sig crypto.Signature, block *model.Block, signerKey crypto.PublicKey) (bool, error) {
	// convert into message bytes
	msg := BlockToBytesForSign(block)

	// validate the staking signature
	valid, err := signerKey.Verify(sig, msg, s.stakingHasher)
	if err != nil {
		return false, fmt.Errorf("cannot verify staking sig: %w", err)
	}
	return valid, nil
}

// VerifyRandomBeaconSig verifies a single random beacon signature for a block using the given public key
// sig - the signature to be verified
// block - the block that the signature was signed for.
// randomBeaconSignerIndex - the signer index of signer's random beacon key share.
func (s *SigProvider) VerifyRandomBeaconSig(sig crypto.Signature, block *model.Block, signerPubKey crypto.PublicKey) (bool, error) {
	// skip the validation if random beacon is not enabled
	if !s.isRandomBeaconEnabled {
		return true, nil
	}

	// convert into message bytes
	msg := BlockToBytesForSign(block)

	// validate random beacon sig with public key
	valid, err := signerPubKey.Verify(sig, msg, s.randomBeaconHasher)
	if err != nil {
		return false, fmt.Errorf("cannot verify random beacon signature: %w", err)
	}

	return valid, nil
}

// VerifyAggregatedStakingSignature verifies an aggregated signature.
// aggsig - the aggregated signature to be verified
// block - the block that the signature was signed for.
// signerKeys - the public keys of all the signers who signed the block.
// Note: since the aggregated sig is a slice of all sigs, it assumes each sig
// pair up with the coresponding signer key at the same index. That means, it's
// the caller's responsibility to ensure `aggsig` and `signerKeys` can pair up
// at each index.
func (s *SigProvider) VerifyAggregatedStakingSignature(aggsig []crypto.Signature, block *model.Block, signerKeys []crypto.PublicKey) (bool, error) {
	// for now the aggregated staking signature for BLS signatures is implemented as a slice of all the signatures.
	// to verify it, we basically verify every single signature

	// check that the number of keys and signatures should match
	if len(aggsig) != len(signerKeys) {
		return false, nil
	}

	msg := BlockToBytesForSign(block)

	// check each signature
	for i, sig := range aggsig {
		signerKey := signerKeys[i]

		// validate the staking signature
		valid, err := signerKey.Verify(sig, msg, s.stakingHasher)
		if err != nil {
			return false, fmt.Errorf("cannot verify aggregated staking sig for (%d)-th sig: %w", i, err)
		}
		if !valid {
			return false, nil
		}
	}

	return true, nil
}

// VerifyAggregatedRandomBeaconSignature verifies an aggregated random beacon signature, which is a threshold signature
func (s *SigProvider) VerifyAggregatedRandomBeaconSignature(sig crypto.Signature, block *model.Block) (bool, error) {
	// skip the validation if random beacon is not enabled
	if !s.isRandomBeaconEnabled {
		return true, nil
	}

	// convert into bytes
	msg := BlockToBytesForSign(block)

	// the reconstructed signature is also a BLS signature which can be verified by the group public key
	valid, err := s.dkgPubData.GroupPubKey.Verify(sig, msg, s.randomBeaconHasher)
	if err != nil {
		return false, fmt.Errorf("cannot verify reconstructed random beacon sig: %w", err)
	}

	return valid, nil
}

// CanReconstruct returns if the given number of signature shares is enough to reconstruct the random beaccon sigs
func (s *SigProvider) CanReconstruct(numOfSigShares int) bool {
	// skip the check if random beacon is not enabled
	if !s.isRandomBeaconEnabled {
		return true
	}

	return crypto.EnoughShares(s.dkgPubData.Size(), numOfSigShares)
}

// Aggregate aggregates the given signature that signed on the given block
// block - it is needed in order to double check the reconstruct signature is valid
// And verifying the sig requires the signed message, which is the block
// sigs - the signatures to be aggregated. Assuming each signature has been verified already.
func (s *SigProvider) Aggregate(block *model.Block, sigs []*model.SingleSignature) (*model.AggregatedSignature, error) {

	// check if sigs is empty
	if len(sigs) == 0 {
		return nil, fmt.Errorf("empty signature")
	}

	// aggregate staking sigs
	aggStakingSigs, signerIDs := aggregateStakingSignature(sigs)

	aggsig := model.AggregatedSignature{
		StakingSignatures:     aggStakingSigs,
		RandomBeaconSignature: nil,
		SignerIDs:             signerIDs,
	}

	// add random beacon sig if enabled
	if s.isRandomBeaconEnabled {
		// convert signerIDs into random beacon pubkey shares
		sigShares, err := s.getSignerIDsAndSigShares(block.BlockID, sigs)
		if err != nil {
			return nil, fmt.Errorf("cannot get random beacon key shares: %w", err)
		}

		msg := BlockToBytesForSign(block)

		// reconstruct random beacon sig
		reconstructedRandomBeaconSig, err := Reconstruct(msg, s.dkgPubData, sigShares)
		if err != nil {
			return nil, fmt.Errorf("cannot reconstruct random beacon sig: %w", err)
		}
		aggsig.RandomBeaconSignature = reconstructedRandomBeaconSig
	}

	return &aggsig, nil
}

func aggregateStakingSignature(sigs []*model.SingleSignature) ([]crypto.Signature, []flow.Identifier) {
	// This implementation is a naive way of aggregation the signatures. It will work, with
	// the downside of costing more bandwidth.
	// The more optimal way, which is the real aggregation, will be implemented when the crypto
	// API is available.
	aggsig := make([]crypto.Signature, len(sigs))
	for i, sig := range sigs {
		aggsig[i] = sig.StakingSignature
	}

	// pick signer IDs from signatures
	signerIDs := make([]flow.Identifier, len(sigs))
	for i, sig := range sigs {
		signerIDs[i] = sig.SignerID
	}

	return aggsig, signerIDs
}

// VoteFor signs a Block and returns the Vote for that Block
func (s *SigProvider) VoteFor(block *model.Block) (*model.Vote, error) {
	// convert to bytes to be signed
	msg := BlockToBytesForSign(block)

	// generate staking signature
	stakingSig, err := s.stakingPrivateKey.Sign(msg, s.stakingHasher)
	if err != nil {
		return nil, fmt.Errorf("fail to sign block (%x) to vote: %w", block.BlockID, err)
	}

	sig := model.SingleSignature{
		StakingSignature:      stakingSig,
		RandomBeaconSignature: nil,
		SignerID:              s.myID,
	}

	if s.isRandomBeaconEnabled {
		// generate random beacon signature
		randomBeaconSig, err := s.randomBeaconPrivateKey.Sign(msg, s.randomBeaconHasher)
		if err != nil {
			return nil, fmt.Errorf("fail to sign block (%x) to vote: %w", block.BlockID, err)
		}
		sig.RandomBeaconSignature = randomBeaconSig
	}

	return &model.Vote{
		BlockID:   block.BlockID,
		View:      block.View,
		Signature: &sig,
	}, nil
}

// Propose signs a Block and returns the Proposal
func (s *SigProvider) Propose(block *model.Block) (*model.Proposal, error) {
	// convert to bytes to be signed
	msg := BlockToBytesForSign(block)

	// generate staking signature
	stakingSig, err := s.stakingPrivateKey.Sign(msg, s.stakingHasher)
	if err != nil {
		return nil, fmt.Errorf("fail to sign block (%x) to propose: %w", block.BlockID, err)
	}

	proposal := model.Proposal{
		Block:                 block,
		StakingSignature:      stakingSig,
		RandomBeaconSignature: nil,
	}

	if s.isRandomBeaconEnabled {
		// generate random beacon signature
		randomBeaconSig, err := s.randomBeaconPrivateKey.Sign(msg, s.randomBeaconHasher)
		if err != nil {
			return nil, fmt.Errorf("fail to sign block (%x) to propose: %w", block.BlockID, err)
		}
		proposal.RandomBeaconSignature = randomBeaconSig
	}

	return &proposal, nil
}

func (s *SigProvider) getSignerIDsAndSigShares(blockID flow.Identifier, sigs []*model.SingleSignature) ([]*SigShare, error) {
	// sanity check
	if len(sigs) == 0 {
		return nil, fmt.Errorf("signatures should not be empty")
	}

	// lookup signer by signer ID and make SigShare
	sigShares := make([]*SigShare, len(sigs))
	for i, sig := range sigs {
		// TODO: confirm if combining into one query is possible and faster
		signer, err := s.protocolState.AtBlockID(blockID).Identity(sig.SignerID)
		if err != nil {
			return nil, fmt.Errorf("cannot get identity by signer ID: %v, %w", sig.SignerID, err)
		}

		sigShare := SigShare{
			Signature:    sigs[i].RandomBeaconSignature,
			SignerPubKey: signer.RandomBeaconPubKey,
		}
		sigShares[i] = &sigShare
	}

	return sigShares, nil
}

// BlockToBytesForSign generates the bytes that was signed for a block
// Note: this function should be reused when signing a block or a vote
func BlockToBytesForSign(block *model.Block) []byte {
	// TODO: we are supposed to sign on `encode(BlockID, View)`
	// but what actually signing is `hash(encoding(BlockID, View))`
	// it works, but the hash is useless, because the signing function
	// in crypto library will always hash it.
	// so instead of using MakeID, we could just return the encoded tuple
	// of BlockID and View
	msg := flow.MakeID(struct {
		BlockID flow.Identifier
		View    uint64
	}{
		BlockID: block.BlockID,
		View:    block.View,
	})
	return msg[:]
}
