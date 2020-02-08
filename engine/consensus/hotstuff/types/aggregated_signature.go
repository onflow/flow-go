package types

import "github.com/dapperlabs/flow-go/model/flow"

type AggregatedSignature struct {
	// Raw is the raw signature bytes.
	// It might be a aggregated BLS signature or a combination of aggregated BLS signature
	// and threshold signature.
	Raw []byte

	// Signers are the identities of all the signers.
	Signers []*flow.Identity
}
