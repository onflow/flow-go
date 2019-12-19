package flow

import "github.com/dapperlabs/flow-go/model"

// NodeSignature is the outcome of a node signing an entity
type NodeSignature struct {
	// the unique id of node
	NodeID [32]byte
	// the ID of the entity
	Entity model.Identifier
	// signature bytes
	Signature []byte
}
