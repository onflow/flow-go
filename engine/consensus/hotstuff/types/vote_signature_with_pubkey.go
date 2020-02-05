package types

import "github.com/dapperlabs/flow-go/model/flow"

// VoteSignatureWithPubKey is the type to tire the signature
// and the signer public key, as well as the signer's index together.
type VoteSignatureWithPubKey struct {
	VoteSignature *flow.PartialSignature
	PubKey        PubKey
}
