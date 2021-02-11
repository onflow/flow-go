package unittest

import (
	"fmt"

	"github.com/onflow/flow-go/state/protocol/events"
)

type FixedStaker struct {
	events.Noop
	Staked bool
}

func NewFixedStaker(initial bool) *FixedStaker {
	return &FixedStaker{
		Staked: initial,
	}
}

func (f *FixedStaker) Refresh() error {
	return nil
}

func (f *FixedStaker) AmIStaked() bool {
	fmt.Printf("am I staked? %t\n", f.Staked)
	return f.Staked
}
