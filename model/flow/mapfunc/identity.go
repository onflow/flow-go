package mapfunc

import (
	"github.com/onflow/flow-go/model/flow"
)

// WithInitialWeight returns an anonymous function that assigns the given weight value
// to `Identity.InitialWeight`. This function is primarily intended for testing, as
// Identity structs should be immutable by convention.
func WithInitialWeight(weight uint64) flow.IdentityMapFunc[flow.Identity] {
	return func(identity flow.Identity) flow.Identity {
		identity.InitialWeight = weight
		return identity
	}
}

// WithWeight returns an anonymous function that assigns the given weight value
// to `Identity.Weight`. This function is primarily intended for testing, as
// Identity structs should be immutable by convention.
func WithWeight(weight uint64) flow.IdentityMapFunc[flow.Identity] {
	return func(identity flow.Identity) flow.Identity {
		identity.Weight = weight
		return identity
	}
}
