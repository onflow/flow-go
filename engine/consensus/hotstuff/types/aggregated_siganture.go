package types

type AggregatedSignature struct{}

// FromSignatures builds an aggregated signature from a slice of signature and a signerCount
// sigs is the slice of signatures from all the signers
// signers is the flag from the entire identity list for who signed it and who didn't.
func FromSignatures(sigs []*Signature, signerCount uint32) (*AggregatedSignature, error) {
	panic("TODO")
}
