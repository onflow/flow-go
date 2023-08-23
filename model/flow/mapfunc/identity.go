package mapfunc

import (
	"github.com/onflow/flow-go/model/flow"
)

func WithInitialWeight(weight uint64) flow.IdentityMapFunc {
	return func(identity flow.Identity) flow.Identity {
		identity.InitialWeight = weight
		return identity
	}
}

func WithWeight(weight uint64) flow.IdentityMapFunc {
	return func(identity flow.Identity) flow.Identity {
		identity.Weight = weight
		return identity
	}
}
