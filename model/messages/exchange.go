package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// EntityRequest is a request for a set of entities, each keyed by an
// identifier. The relationship between the identifiers and the entity is not
// specified here. In the typical case, the identifier is simply the ID of the
// entity being requested, but more complex identifier-entity relationships can
// be used as well.
//
//structwrite:immutable
type EntityRequest struct {
	Nonce     uint64
	EntityIDs []flow.Identifier
}

// UntrustedEntityRequest is an untrusted input-only representation of an EntityRequest,
// used for construction.
//
// An instance of UntrustedEntityRequest should be validated and converted into
// a trusted EntityRequest using NewEntityRequest constructor.
type UntrustedEntityRequest EntityRequest

// NewEntityRequest creates a new instance of EntityRequest.
//
// Parameters:
//   - untrusted: untrusted EntityRequest to be validated
//
// Returns:
//   - *EntityRequest: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewEntityRequest(untrusted UntrustedEntityRequest) (*EntityRequest, error) {
	// TODO: add validation logic
	return &EntityRequest{Nonce: untrusted.Nonce, EntityIDs: untrusted.EntityIDs}, nil
}

// EntityResponse is a response to an entity request, containing a set of
// serialized entities and the identifiers used to request them. The returned
// entity set may be empty or incomplete.
//
//structwrite:immutable
type EntityResponse struct {
	Nonce     uint64
	EntityIDs []flow.Identifier
	Blobs     [][]byte
}

// UntrustedEntityResponse is an untrusted input-only representation of an EntityResponse,
// used for construction.
//
// An instance of UntrustedEntityResponse should be validated and converted into
// a trusted EntityResponse using NewEntityResponse constructor.
type UntrustedEntityResponse EntityResponse

// NewEntityResponse creates a new instance of EntityResponse.
//
// Parameters:
//   - untrusted: untrusted EntityResponse to be validated
//
// Returns:
//   - *EntityResponse: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewEntityResponse(untrusted UntrustedEntityResponse) (*EntityResponse, error) {
	// TODO: add validation logic
	return &EntityResponse{Nonce: untrusted.Nonce, EntityIDs: untrusted.EntityIDs, Blobs: untrusted.Blobs}, nil
}
