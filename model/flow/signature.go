package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// NodeSignature is the outcome of a node signing an entity
type NodeSignature struct {
	// the unique id of node
	NodeID [32]byte
	// the ID of the entity
	Entity Identifier
	// signature bytes
	Signature []byte
}

// AggregatedSignature represents an aggregated BLS signature.
// TODO should be replaced with BLS signature from crypto library
type AggregatedSignature []crypto.Signature

// PartialSignature represents a partial BLS signature.
type PartialSignature struct {
	// Raw is the raw signature bytes.
	Raw []byte
	// SignerIndex is the index of the signer. Tracking the identity list this
	// index corresponds to is the responsibility of the user of this type.
	SignerIndex uint
}
