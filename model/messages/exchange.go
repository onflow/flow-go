package messages

import (
	"github.com/dapperlabs/flow-go/model/flow"
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

// EntityResponse is a response to an entity request, containing a set of
// serialized entities and the identifiers used to request them. The returned
// entity set may be empty or incomplete.
type EntityResponse struct {
	Nonce     uint64
	EntityIDs []flow.Identifier
	Blobs     [][]byte
}
