package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// EntityRequest is a request for a set of entities, each keyed by an
// identifier. The relationship between the identifiers and the entity is not
// specified here. In the typical case, the identifier is simply the ID of the
// entity being requested, but more complex identifier-entity relationships can
// be used as well.
type EntityRequest struct {
	Nonce     uint64
	EntityIDs []flow.Identifier
}

func (e EntityRequest) ToInternal() (any, error) {
	// TODO(malleability, #7719) implement with validation checks
	return e, nil
}

// EntityResponse is a response to an entity request, containing a set of
// serialized entities and the identifiers used to request them. The returned
// entity set may be empty or incomplete.
type EntityResponse struct {
	Nonce     uint64
	EntityIDs []flow.Identifier
	Blobs     [][]byte
}

func (e EntityResponse) ToInternal() (any, error) {
	// TODO(malleability, #7720) implement with validation checks
	return e, nil
}
