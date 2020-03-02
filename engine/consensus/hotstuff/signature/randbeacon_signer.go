// +build relic

package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
)

// SigProvider provides symmetry functions to generate and verify signatures
type RandomBeaconSigner struct {
	RandomBeaconSigVerifier
	viewState              *hotstuff.ViewState
	randomBeaconPrivateKey crypto.PrivateKey // private key for random beacon signature
}

// SigShare is the signature share for reconstructing threshold signature
type SigShare struct {
	Signature    crypto.Signature // the signature share
	SignerPubKey crypto.PublicKey // the public key of the signer
}

// NewSigProvider creates an instance of SigProvider
func NewRandomBeaconSigner(viewState *hotstuff.ViewState, randomBeaconPrivateKey crypto.PrivateKey) RandomBeaconSigner {
	return RandomBeaconSigner{
		RandomBeaconSigVerifier: NewRandomBeaconSigVerifier(),
		viewState:               viewState,
		randomBeaconPrivateKey:  randomBeaconPrivateKey,
	}
}

// Sign signs a the message with the node's random beacon key
func (s *RandomBeaconSigner) Sign(msg []byte) (crypto.Signature, error) {
	return s.randomBeaconPrivateKey.Sign(msg, s.randomBeaconHasher)
}

// CanReconstruct returns if the given number of signature shares is enough to reconstruct the random beaccon sigs
func (s *RandomBeaconSigner) CanReconstruct(numOfSigShares int) bool {
	return crypto.EnoughShares(s.dkgGroupSize(), numOfSigShares)
}

// Reconstruct reconstructs a threshold signature from a list of the signature shares.
//
// msg - the message every signature share was signed on.
// sigShares - the list of signature shares to be reconstructed. Note it assumes each signature has been verified.
//
// The return value:
// sig - the reconstructed signature
// error - some unknown error if exists
//
// Preconditions:
//    * ensure enough signatures have been collected (verified by calling `CanReconstruct`)
//    * the sigShares are all distinct
// Violating preconditions will result in an error (but not the reconstruction of an invalid threshold signature)
func (s *RandomBeaconSigner) Reconstruct(msg []byte, sigShares []*SigShare) (crypto.Signature, error) {
	// double check if there are enough shares.
	if !s.CanReconstruct(len(sigShares)) {
		// the should not happen, since it assumes the caller has checked before
		return nil, fmt.Errorf("not enough shares to reconstruct random beacon sig, expect: %v, got: %v", s.dkgGroupSize(), len(sigShares))
	}

	// pick signatures
	sigs := make([]crypto.Signature, 0, len(sigShares))
	signerKeys := make([]crypto.PublicKey, 0, len(sigShares))

	for _, share := range sigShares {
		sigs = append(sigs, share.Signature)
		signerKeys = append(signerKeys, share.SignerPubKey)
	}

	// lookup signer indexes
	found, dkgIndices := s.lookupDKGIndices(signerKeys)
	if !found {
		return nil, fmt.Errorf("signer index for sig shares can't be found")
	}

	// reconstruct the threshold sig
	reconstructedSig, err := crypto.ReconstructThresholdSignature(s.dkgGroupSize(), sigs, dkgIndices)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct threshold sig: %w", err)
	}

	// verify the reconstruct signature is valid
	publicGroupKey := s.viewState.DKGPublicData().GroupPubKey
	valid, err := publicGroupKey.Verify(msg, reconstructedSig, s.randomBeaconHasher)
	if err != nil {
		return nil, fmt.Errorf("cannot verify the reconstructed signature, %w", err)
	}

	if !valid {
		return nil, fmt.Errorf("reconstructed an invalid threshold signature")
	}

	return reconstructedSig, nil
}

func (s *RandomBeaconSigner) dkgGroupSize() int {
	return len(s.viewState.DKGPublicData().SignerIndexMapping)
}

// lookupDKGIndices looks up the signer indices for given slice of public keys
func (s *RandomBeaconSigner) lookupDKGIndices(signerKeys []crypto.PublicKey) (bool, []int) {
	signerIndexMapping := s.viewState.DKGPublicData().SignerIndexMapping
	signerIndexes := make([]int, 0, len(signerKeys))
	for _, signerKey := range signerKeys {
		signerIndex, found := signerIndexMapping[signerKey]
		if !found {
			return false, nil
		}
		signerIndexes = append(signerIndexes, signerIndex)
	}
	return true, signerIndexes
}
