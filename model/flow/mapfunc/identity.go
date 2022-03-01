package mapfunc

import (
	"github.com/onflow/flow-go/model/flow"
)

func WithStake(stake uint64) flow.IdentityMapFunc {
	return func(identity flow.Identity) flow.Identity {
		identity.Weight = stake
		return identity
	}
}
