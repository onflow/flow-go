package messages

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// EntityRequest is a request for a set of entities, each keyed by an
// identifier. The relationship between the identifiers and the entity is not
// specified here. In the typical case, the identifier is simply the ID of the
// entity being requested, but more complex identifier-entity relationships can
// be used as well.
//
type EntityRequest struct {
	Nonce     uint64
	EntityIDs []flow.Identifier
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedEntityRequest EntityRequest

//
func NewEntityRequest(untrusted UntrustedEntityRequest) (*EntityRequest, error) {
	if len(untrusted.EntityIDs) == 0 {
		return nil, fmt.Errorf("entity IDs must not be empty")
	}
	for i, entityID := range untrusted.EntityIDs {
		if entityID == flow.ZeroID {
			return nil, fmt.Errorf("entity ID at index %d must not be zero", i)
		}
	}
	return &EntityRequest{
		Nonce:     untrusted.Nonce,
		EntityIDs: untrusted.EntityIDs,
	}, nil
}

// EntityResponse is a response to an entity request, containing a set of
// serialized entities and the identifiers used to request them. The returned
// entity set may be empty or incomplete.
//
type EntityResponse struct {
	Nonce     uint64
	EntityIDs []flow.Identifier
	Blobs     [][]byte
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedEntityResponse EntityResponse

//
func NewEntityResponse(untrusted UntrustedEntityResponse) (*EntityResponse, error) {
	if len(untrusted.EntityIDs) != len(untrusted.Blobs) {
		return nil, fmt.Errorf("entity IDs length (%d) must match blobs length (%d)", len(untrusted.EntityIDs), len(untrusted.Blobs))
	}
	for i, entityID := range untrusted.EntityIDs {
		if entityID == flow.ZeroID {
			return nil, fmt.Errorf("entity ID at index %d must not be zero", i)
		}
	}
	return &EntityResponse{
		Nonce:     untrusted.Nonce,
		EntityIDs: untrusted.EntityIDs,
		Blobs:     untrusted.Blobs,
	}, nil
}
