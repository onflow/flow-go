package types

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Signature is a signature from a single signer, which can be aggregaged
// into aggregated signature.
type SingleSignature struct {
	// The raw bytes of the signature.
	Raw []byte

	// The identifier of the signer, which identifies signer's role, stake, etc.
	SignerID flow.Identifier
}

// AggregatedSignature is the aggregated signatures signed by all the signers on the same message.
type AggregatedSignature struct {
	// Raw is the raw signature bytes.
	// It might be a aggregated BLS signature or a combination of aggregated BLS signature
	// and threshold signature.
	Raw []crypto.Signature

	// the Identifiers of all the signers
	SignerIDs []flow.Identifier
}
