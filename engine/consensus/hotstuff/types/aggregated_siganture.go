package types

import "fmt"

type AggregatedSignature struct {
	RawSignature []byte
	Signers      []bool
}

type RawSignature = [32]byte

// FromSignatures builds an aggregated signature from a slice of signature and a signerCount
// sigs is the slice of signatures from all the signers
// signers is the flag from the entire identity list for who signed it and who didn't.
func FromSignatures(sigs []*Signature, signerCount uint32) (*AggregatedSignature, error) {
	rawSigs := make([]RawSignature, len(sigs))
	signers := make([]bool, signerCount)

	for i, sig := range sigs {
		rawSigs[i] = sig.RawSignature

		// double check the SignerIdx must be fall into the valid range: [0, signerCount - 1]
		if sig.SignerIdx >= signerCount {
			return nil, fmt.Errorf("cannot make aggregated signature, due to invalid SignerIdx: %v, or signerCount: %v", sig.SignerIdx, signerCount)
		}

		// set signer to be true at the signer index
		signers[sig.SignerIdx] = true
	}

	aggRawSig := buildAggregatedSignature(rawSigs, signers)

	return &AggregatedSignature{
		RawSignature: aggRawSig,
		Signers:      signers,
	}, nil
}

// buildAggregatedSignature will use cryto library to generate aggregated signature
func buildAggregatedSignature(sigs []RawSignature, signers []bool) []byte {
	panic("TODO")
}

// Verify that the aggregated signature contains a signature from a given public key for signing the given hash
// hash - the hash of signature
// pubkey - the public key to verify if its signature over the hash is included in the aggregated signature
func (a AggregatedSignature) Verify(hash []byte, pubkey [32]byte) bool {
	return verifyAggregatedSignature(a.RawSignature, hash, pubkey)
}

func verifyAggregatedSignature(sig []byte, hash []byte, pubkey [32]byte) bool {
	panic("TODO")
}
