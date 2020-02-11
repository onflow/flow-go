package flow

// NodeSignature is the outcome of a node signing an entity
type NodeSignature struct {
	// the unique id of node
	NodeID [32]byte
	// the ID of the entity
	Entity Identifier
	// signature bytes
	Signature []byte
}

// AggregatedSignature represents an aggregated signature.
type AggregatedSignature struct {
	// Raw is the raw signature bytes.
	Raw []byte
	// the Identifiers of all the signers
	Signers []Identifier
}

// PartialSignature represents a partial signature.
type PartialSignature struct {
	// Raw is the raw signature bytes.
	Raw []byte
	// The identifier of the signer, which identifies signer's role, stake, etc.
	Signer Identifier
}
