package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Requester interface {

	// EntityByID will request an entity through the request engine backing
	// the interface. The additional selector will be applied to the subset
	// of valid providers for the entity and allows finer-grained control
	// over which providers to request a given entity from. Use `filter.Any`
	// if no additional restrictions are required.
	EntityByID(entityID flow.Identifier, selector flow.IdentityFilter) error
}
