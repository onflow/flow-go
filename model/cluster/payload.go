package cluster

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// Payload is the payload for blocks in collection node cluster consensus.
// It contains only a single collection.
//
//structwrite:immutable - mutations allowed only within the constructor
type Payload struct {

	// Collection is the collection being created.
	Collection flow.Collection

	// ReferenceBlockID is the ID of a reference block on the main chain. It
	// is defined as the ID of the reference block with the lowest height
	// from all transactions within the collection. If the collection is empty,
	// the proposer may choose any reference block, so long as it is finalized
	// and within the epoch the cluster is associated with. If a cluster was
	// assigned for epoch E, then all of its reference blocks must have a view
	// in the range [E.FirstView, E.FinalView]. However, if epoch fallback is
	// triggered in epoch E, then any reference block with view ≥ E.FirstView
	// may be used.
	//
	// This determines when the collection expires, using the same expiry rules
	// as transactions. It is also used as the reference point for committee
	// state (staking, etc.) when validating the containing block.
	//
	// The root block of a cluster chain has an empty reference block ID, as it
	// is created in advance of its members (necessarily) being authorized network
	// members. It is invalid for any non-root block to have an empty reference
	// block ID.
	ReferenceBlockID flow.Identifier
}

// NewEmptyPayload returns a payload with an empty collection and the given
// reference block ID.
func NewEmptyPayload(refID flow.Identifier) *Payload {
	return &Payload{
		Collection:       *flow.NewEmptyCollection(),
		ReferenceBlockID: refID,
	}
}

// UntrustedPayload is an untrusted input-only representation of a cluster Payload,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedPayload should be validated and converted into
// a trusted cluster Payload using NewPayload constructor.
type UntrustedPayload Payload

// NewPayload creates a payload given a reference block ID and a
// list of transaction hashes.
// Construction cluster Payload allowed only within the constructor.
//
// All errors indicate a valid Payload cannot be constructed from the input.
func NewPayload(untrusted UntrustedPayload) (*Payload, error) {
	collection, err := flow.NewCollection(flow.UntrustedCollection(untrusted.Collection))
	if err != nil {
		return nil, fmt.Errorf("could not construct collection: %w", err)
	}
	return &Payload{
		Collection:       *collection,
		ReferenceBlockID: untrusted.ReferenceBlockID,
	}, nil
}

// NewRootPayload creates a root payload for a root cluster block.
//
// This constructor must be used **only** for constructing the root payload,
// which is the only case where zero values are allowed.
func NewRootPayload(untrusted UntrustedPayload) (*Payload, error) {
	if untrusted.ReferenceBlockID != flow.ZeroID {
		return nil, fmt.Errorf("ReferenceBlockID must be empty")
	}

	if len(untrusted.Collection.Transactions) != 0 {
		return nil, fmt.Errorf("Collection must be empty")
	}

	return &Payload{
		Collection:       untrusted.Collection,
		ReferenceBlockID: untrusted.ReferenceBlockID,
	}, nil
}

// Hash returns the hash of the payload.
func (p Payload) Hash() flow.Identifier {
	return flow.MakeID(p)
}
