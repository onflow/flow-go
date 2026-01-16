package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// Requester provides an interface to request entities from other nodes in the network.
// When the Requester is instructed to retrieve some entity, the Requester handles sending request messages and retrying requests.
// Requested entities are passed to a Handler function, which is configured elsewhere (see [module/requester.Engine.WithHandle]).
type Requester interface {
	// EntityByID will enqueue the given entity for request by its ID (content hash).
	// The selector will be applied to the subset of valid providers configured globally for the Requester instance.
	// This allows finer-grained control over which providers to request from on a per-entity basis.
	// Use `filter.Any` if no additional restrictions are required.
	// Received entities will be verified for integrity using their ID function.
	// Idempotent w.r.t. `queryKey` (if prior request is still ongoing, we just continue trying).
	// Concurrency safe.
	EntityByID(entityID flow.Identifier, selector flow.IdentityFilter[flow.Identity])

	// EntityBySecondaryKey will enqueue the given entity for request by some secondary identifier (NOT its content hash).
	// The selector will be applied to the subset of valid providers configured globally for the Requester instance.
	// This allows finer-grained control over which providers to request from on a per-entity basis.
	// Use `filter.Any` if no additional restrictions are required.
	// CAUTION: NOT BFT, because received entities WILL NOT be verified for integrity using their ID function.
	// Idempotent w.r.t. `queryKey` (if prior request is still ongoing, we just continue trying).
	// Concurrency safe.
	EntityBySecondaryKey(queryKey flow.Identifier, selector flow.IdentityFilter[flow.Identity])

	// Force will force the dispatcher to send all possible batches immediately.
	// It can be used in cases where responsiveness is of utmost importance, at
	// the cost of additional network messages.
	Force()
}

type NoopRequester struct{}

func (n NoopRequester) EntityByID(entityID flow.Identifier, selector flow.IdentityFilter[flow.Identity]) {
}

func (n NoopRequester) EntityBySecondaryKey(key flow.Identifier, selector flow.IdentityFilter[flow.Identity]) {
}

func (n NoopRequester) Force() {}

func (n NoopRequester) WithHandle(func(flow.Identifier, flow.Entity)) Requester {
	return n
}
