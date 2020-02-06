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
	// Signers is a "bitmap" of signer indices. Tracking the identity list the
	// indices correspond to is the responsibility of the user of the type.
	Signers []bool
}

// PartialSignature represents a partial signature.
type PartialSignature struct {
	// Raw is the raw signature bytes.
	Raw []byte
	// SignerIndex is the index of the signer. Tracking the identity list this
	// index corresponds to is the responsibility of the user of this type.
	SignerIndex uint32
}
