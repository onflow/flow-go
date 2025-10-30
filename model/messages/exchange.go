package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// EntityRequest is a request for a set of entities, each keyed by an
// identifier. The relationship between the identifiers and the entity is not
// specified here. In the typical case, the identifier is simply the ID of the
// entity being requested, but more complex identifier-entity relationships can
// be used as well.
type EntityRequest flow.EntityRequest

// ToInternal converts the untrusted EntityRequest into its trusted internal
// representation.
// No errors are expected during normal operations.
func (e *EntityRequest) ToInternal() (any, error) {
	return (*flow.EntityRequest)(e), nil
}

// EntityResponse is a response to an entity request, containing a set of
// serialized entities and the identifiers used to request them. The returned
// entity set may be empty or incomplete.
type EntityResponse flow.EntityResponse

// ToInternal converts the untrusted EntityResponse into its trusted internal
// representation.
// No errors are expected during normal operations.
func (e *EntityResponse) ToInternal() (any, error) {
	return (*flow.EntityResponse)(e), nil
}
