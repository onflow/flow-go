// +build relic

package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto"
)

// RandomBeaconSigner provides functions to generate and verify Random Beacon signatures.
// Specifically, it can generate and verify individual signatures shares (e.g. from a vote)
// and reconstructed threshold signatures (e.g. from a Quorum Certificate).
type RandomBeaconSigner struct {
	RandomBeaconSigVerifier
	viewState              hotstuff.ViewState
	randomBeaconPrivateKey crypto.PrivateKey // private key for random beacon signature
}

// RandomBeaconSigner creates an instance of RandomBeaconSigner
func NewRandomBeaconSigner(viewState hotstuff.ViewState, randomBeaconPrivateKey crypto.PrivateKey) *RandomBeaconSigner {
	return &RandomBeaconSigner{
		RandomBeaconSigVerifier: NewRandomBeaconSigVerifier(),
		viewState:               viewState,
		randomBeaconPrivateKey:  randomBeaconPrivateKey,
	}
}

// Sign signs the block with the node's random beacon key
func (s *RandomBeaconSigner) Sign(block *model.Block) (crypto.Signature, error) {
	msg := BlockToBytesForSign(block)
	return s.randomBeaconPrivateKey.Sign(msg, s.randomBeaconHasher)
}

// CanReconstruct returns if the given number of signature shares is enough to reconstruct the random beaccon sigs
func (s *RandomBeaconSigner) CanReconstruct(numOfSigShares int) bool {
	return crypto.EnoughShares(s.dkgGroupSize(), numOfSigShares)
}

// Reconstruct reconstructs a threshold signature from a list of the signature shares.
//
// Inputs:
//    * msg - the message every signature share was signed on.
//    * signatures - the list of signature from which the threshold sig should be reconstructed
//
// Preconditions:
//    * each random beacon sig share has been verified
//    * enough signatures have been collected (can be verified by calling `CanReconstruct`)
//    * all signatures are from different parties
// Violating preconditions will result in an error (but not the reconstruction of an invalid threshold signature).
func (s *RandomBeaconSigner) Reconstruct(block *model.Block, signatures []*model.SingleSignature) (crypto.Signature, error) {
	// double check if there are enough shares.
	if !s.CanReconstruct(len(signatures)) {
		// the should not happen, since it assumes the caller has checked before
		return nil, fmt.Errorf("not enough shares to reconstruct random beacon sig, expect: %d, got: %d", s.dkgGroupSize(), len(signatures))
	}

	// collect random beacon sig share and dkg index for each signer
	sigShares := make([]crypto.Signature, 0, len(signatures))
	dkgIndices := make([]int, 0, len(signatures))
	dkgState := s.viewState.DKGState()
	for _, sig := range signatures {
		sigShares = append(sigShares, sig.RandomBeaconSignature)
		index, err := dkgState.ShareIndex(sig.SignerID)
		if err != nil {
			return nil, fmt.Errorf("could not get DKG index (signer: %x): %w", sig.SignerID, err)
		}
		dkgIndices = append(dkgIndices, int(index))
	}
	// reconstruct the threshold sig
	reconstructedSig, err := crypto.ReconstructThresholdSignature(s.dkgGroupSize(), sigShares, dkgIndices)
	if err != nil {
		return nil, fmt.Errorf("reconstructing threshold sig failed: %w", err)
	}

	// get the DKG public group key
	publicGroupKey, err := dkgState.GroupKey()
	if err != nil {
		return nil, fmt.Errorf("could not get the DKG public group key: %w", err)
	}

	// sanity check: verify the reconstruct signature is valid
	valid, err := s.VerifyRandomBeaconThresholdSig(reconstructedSig, block, publicGroupKey)
	if err != nil {
		return nil, fmt.Errorf("cannot verify the reconstructed signature, %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("reconstructed an invalid threshold signature")
	}

	return reconstructedSig, nil
}

func (s *RandomBeaconSigner) dkgGroupSize() int {
	groupSize, _ := s.viewState.DKGState().GroupSize()
	return int(groupSize)
}
