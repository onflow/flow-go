// +build relic

package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/protocol"
)

// SigProvider provides symmetry functions to generate and verify signatures
type SigProvider struct {
	StakingSigVerifier
	RandomBeaconSigVerifier

	myID                   flow.Identifier
	protocolState          protocol.State
	stakingPrivateKey      crypto.PrivateKey // private staking key
	randomBeaconPrivateKey crypto.PrivateKey // private key for random beacon signature
}

func NewSigProvider(
	myID flow.Identifier,
	protocolState protocol.State,
	stakingSigTag string,
	stakingPrivateKey crypto.PrivateKey,
	dkgPubData *DKGPublicData,
	randomBeaconSigTag string,
	randomBeaconPrivateKey crypto.PrivateKey,
) *SigProvider {
	return &SigProvider{
		StakingSigVerifier: NewStakingSigVerifier(stakingSigTag),
		RandomBeaconSigVerifier: NewRandomBeaconSigVerifier(dkgPubData, randomBeaconSigTag),
		myID:                   myID,
		protocolState: protocolState,
		stakingPrivateKey:      stakingPrivateKey,
		randomBeaconPrivateKey: randomBeaconPrivateKey,
	}
}

// CanReconstruct returns if the given number of signature shares is enough to reconstruct the random beaccon sigs
func (s *SigProvider) CanReconstruct(numOfSigShares int) bool {
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

	// convert signerIDs into random beacon pubkey shares
	sigShares, found, err := s.getSignerIDsAndSigShares(block.BlockID, sigs)
	if err != nil {
		return nil, fmt.Errorf("cannot get random beacon key shares: %w", err)
	}

	if !found {
		// has unstaked nodes
		return nil, fmt.Errorf("no staked nodes found: %w", err)
	}

	msg := BlockToBytesForSign(block)

	// reconstruct random beacon sig
	reconstructedRandomBeaconSig, err := Reconstruct(msg, s.dkgPubData, sigShares)
	if err != nil {
		return nil, fmt.Errorf("cannot reconstruct random beacon sig: %w", err)
	}

	return &model.AggregatedSignature{
		StakingSignatures:     aggStakingSigs,
		RandomBeaconSignature: reconstructedRandomBeaconSig,
		SignerIDs:             signerIDs,
	}, nil
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

	// generate random beacon signature
	randomBeaconSig, err := s.randomBeaconPrivateKey.Sign(msg, s.randomBeaconHasher)
	if err != nil {
		return nil, fmt.Errorf("fail to sign block (%x) to vote: %w", block.BlockID, err)
	}

	return &model.Vote{
		BlockID: block.BlockID,
		View:    block.View,
		Signature: &model.SingleSignature{
			StakingSignature:      stakingSig,
			RandomBeaconSignature: randomBeaconSig,
			SignerID:              s.myID,
		},
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

	// generate random beacon signature
	randomBeaconSig, err := s.randomBeaconPrivateKey.Sign(msg, s.randomBeaconHasher)
	if err != nil {
		return nil, fmt.Errorf("fail to sign block (%x) to propose: %w", block.BlockID, err)
	}

	return &model.Proposal{
		Block:                 block,
		StakingSignature:      stakingSig,
		RandomBeaconSignature: randomBeaconSig,
	}, nil
}

func (s *SigProvider) getSignerIDsAndSigShares(blockID flow.Identifier, sigs []*model.SingleSignature) ([]*SigShare, bool, error) {
	// sanity check
	if len(sigs) == 0 {
		return nil, false, fmt.Errorf("signatures should not be empty")
	}

	// lookup signer by signer ID and make SigShare
	sigShares := make([]*SigShare, len(sigs))
	for i, sig := range sigs {
		// TODO: confirm if combining into one query is possible and faster
		signer, err := s.protocolState.AtBlockID(blockID).Identity(sig.SignerID)
		if err != nil {
			return nil, false, fmt.Errorf("cannot get identity by signer ID: %v, %w", sig.SignerID, err)
		}

		sigShare := SigShare{
			Signature:    sigs[i].RandomBeaconSignature,
			SignerPubKey: signer.RandomBeaconPubKey,
		}
		sigShares[i] = &sigShare
	}

	return sigShares, true, nil
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
