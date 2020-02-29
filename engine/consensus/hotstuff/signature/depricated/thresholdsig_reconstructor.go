// +build relic

package dep

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
)

// DKGPublicData is the public data for DKG participants who generated their key shares
type DKGPublicData struct {
	GroupPubKey        crypto.PublicKey         // the group public key
	SignerIndexMapping map[crypto.PublicKey]int // the mapping from public key to signer index
	Hasher             crypto.Hasher            // the hasher needed for signature verification
}

// LookupIndex looks up the signer indices for given slice of public keys
func (d *DKGPublicData) LookupIndex(signerKeys []crypto.PublicKey) (bool, []int) {
	signerIndexes := make([]int, 0, len(signerKeys))
	for _, signerKey := range signerKeys {
		signerIndex, found := d.SignerIndexMapping[signerKey]
		if !found {
			return false, nil
		}
		signerIndexes = append(signerIndexes, signerIndex)
	}
	return true, signerIndexes
}

// Size returns the total number of participants
func (d DKGPublicData) Size() int {
	return len(d.SignerIndexMapping)
}

// SigShare is the signature share for reconstructing threshold signature
type SigShare struct {
	Signature    crypto.Signature // the signature share
	SignerPubKey crypto.PublicKey // the public key of the signer
}

// Reconstruct reconstructs a threshold signature from a list of the signature shares.
//
// msg - the message every signature share was signed on.
// dkgPubData - the public data about the DKG participants
// sigShares - the list of signature shares to be reconstructed. Note it assumes each signature has been verified.
//
// The return value:
// sig - the reconstructed signature
// error - some unknown error if exists
//
// It assumes the caller has called CanReconstruct to ensure enough signatures have been collected
// It assumes the sigShares are all distinct.
func Reconstruct(msg []byte, dkgPubData *DKGPublicData, sigShares []*SigShare) (crypto.Signature, error) {
	// double check if there are enough shares.
	if !crypto.EnoughShares(dkgPubData.Size(), len(sigShares)) {
		// the should not happen, since it assumes the caller has checked before
		return nil, fmt.Errorf("not enough shares to reconstruct random beacon sig, expect: %v, got: %v", dkgPubData.Size(), len(sigShares))
	}

	// pick signatures
	sigs := make([]crypto.Signature, 0, len(sigShares))
	signerKeys := make([]crypto.PublicKey, 0, len(sigShares))

	for _, share := range sigShares {
		sigs = append(sigs, share.Signature)
		signerKeys = append(signerKeys, share.SignerPubKey)
	}

	// lookup signer indexes
	found, signerIndexes := dkgPubData.LookupIndex(signerKeys)
	if !found {
		return nil, fmt.Errorf("signer index for sig shares can't be found")
	}

	// reconstruct the threshold sig
	reconstructedSig, err := crypto.ReconstructThresholdSignature(dkgPubData.Size(), sigs, signerIndexes)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct threshold sig: %w", err)
	}

	// verify the reconstruct signature is valid
	valid, err := dkgPubData.GroupPubKey.Verify(msg, reconstructedSig, dkgPubData.Hasher)
	if err != nil {
		return nil, fmt.Errorf("cannot verify the reconstructed signature, %w", err)
	}

	if !valid {
		return nil, fmt.Errorf("reconstructed an invalid threshold signature")
	}

	return reconstructedSig, nil
}
