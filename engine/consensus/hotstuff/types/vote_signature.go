package types

// VoteSignature is the signature for voting.
// It's an abstraction of the signature and data about who signed it.
type VoteSignature struct {
	RawSignature []byte
	SignerIndex  []byte
}

type VoteSignatureWithPubKey struct {
	VoteSignature *VoteSignature
	PubKey        PubKey
}
