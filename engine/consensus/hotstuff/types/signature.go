package types

import "github.com/dapperlabs/flow-go/model/flow"

type Signature struct {
	// the raw bytes of the signature.
	Raw []byte

	// looked up by chain compliance layer
	Signer flow.Identity
}
