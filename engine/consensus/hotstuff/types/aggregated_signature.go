package types

import "github.com/dapperlabs/flow-go/model/flow"

// AggregatedSignature is the aggregated signatures signed by all the signers on the same message.
type AggregatedSignature struct {
	// Raw is the raw signature bytes.
	// It might be a aggregated BLS signature or a combination of aggregated BLS signature
	// and threshold signature.
	Raw []byte

	// the Identifiers of all the signers
	Signers []flow.Identifier
}
