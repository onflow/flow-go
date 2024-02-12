package module

import (
	"github.com/onflow/flow-go/model/flow"
)

type Requester interface {
	// EntityByID will request an entity through the request engine backing
	// the interface. The additional selector will be applied to the subset
	// of valid providers for the entity and allows finer-grained control
	// over which providers to request a given entity from. Use `filter.Any`
	// if no additional restrictions are required. Data integrity of response
	// will be checked upon arrival. This function should be used for requesting
	// entites by their IDs.
	EntityByID(entityID flow.Identifier, selector flow.IdentityFilter[flow.Identity])

	// Query will request data through the request engine backing the interface.
	// The additional selector will be applied to the subset
	// of valid providers for the data and allows finer-grained control
	// over which providers to request data from. Doesn't perform integrity check
	// can be used to get entities without knowing their ID.
	Query(key flow.Identifier, selector flow.IdentityFilter[flow.Identity])

	// Force will force the dispatcher to send all possible batches immediately.
	// It can be used in cases where responsiveness is of utmost importance, at
	// the cost of additional network messages.
	Force()

	WithHandle(func(flow.Identifier, flow.Entity)) Requester
}

type NoopRequester struct{}

func (n NoopRequester) EntityByID(entityID flow.Identifier, selector flow.IdentityFilter[flow.Identity]) {
}

func (n NoopRequester) Query(key flow.Identifier, selector flow.IdentityFilter[flow.Identity]) {}

func (n NoopRequester) Force() {}

func (n NoopRequester) WithHandle(func(flow.Identifier, flow.Entity)) Requester {
	return n
}
