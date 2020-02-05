package types

// VoteSignatureWithPubKey is the type to tire the signature
// and the signer public key, as well as the signer's index together.
type VoteSignatureWithPubKey struct {
	VoteSignature *VoteSignature
	PubKey        PubKey
}
