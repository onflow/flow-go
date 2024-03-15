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

// WithEpochParticipationStatus returns an anonymous function that assigns the given epoch participation status value
// to `Identity.EpochParticipationStatus`. This function is primarily intended for testing, as
// Identity structs should be immutable by convention.
func WithEpochParticipationStatus(status flow.EpochParticipationStatus) flow.IdentityMapFunc[flow.Identity] {
	return func(identity flow.Identity) flow.Identity {
		identity.EpochParticipationStatus = status
		return identity
	}
}
