package types

import "github.com/dapperlabs/flow-go/model/flow"

// Signature is a signature from a single signer, which can be aggregaged
// into aggregated signature.
type Signature struct {
	// The raw bytes of the signature.
	Raw []byte

	// The identifier of the signer, which identifies signer's role, stake, etc.
	Signer flow.Identifier
}
