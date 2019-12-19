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
