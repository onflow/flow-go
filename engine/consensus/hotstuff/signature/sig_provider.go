package signature

// SigProvider is a combination of signature related components.
// Since signature's creation and verification are separated into
// different interfaces - signer, verifier, and aggregator. Different
// implementation for each interfaces can be found. In ensure the
// the signing, verification and the aggregation work together,
// a single module is required to implement all the interface methods
type SigProvider interface {
	Signer
	SigVerifier
	AggregatorMaker
}
