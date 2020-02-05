package types

// VoteSignatureWithIndexedPubKey is the type to tire the signature
// and the signer public key, as well as the signer's index together.
// The signer Index from the IndexedPubKey field will be needed for
// aggregating the signatures.
type VoteSignatureWithIndexedPubKey struct {
	VoteSignature *VoteSignature
	IndexedPubKey *IndexedPubKey
}
