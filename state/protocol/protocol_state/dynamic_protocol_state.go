package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// dynamicProtocolStateAdapter implements protocol.DynamicProtocolState by wrapping an initialProtocolStateAdapter.
type dynamicProtocolStateAdapter struct {
	*initialProtocolStateAdapter
	params protocol.GlobalParams
}

var _ protocol.DynamicProtocolState = (*dynamicProtocolStateAdapter)(nil)

func newDynamicProtocolStateAdapter(entry *flow.RichProtocolStateEntry, params protocol.GlobalParams) *dynamicProtocolStateAdapter {
	adapter := newInitialProtocolStateAdapter(entry)
	return &dynamicProtocolStateAdapter{
		initialProtocolStateAdapter: adapter,
		params:                      params,
	}
}

func (s *dynamicProtocolStateAdapter) EpochStatus() *flow.EpochStatus {
	var nextEpoch flow.EventIDs
	if s.NextEpochProtocolState != nil {
		nextEpoch = s.NextEpochProtocolState.CurrentEpochEventIDs
	}
	return &flow.EpochStatus{
		PreviousEpoch:                   s.PreviousEpochEventIDs,
		CurrentEpoch:                    s.CurrentEpochEventIDs,
		NextEpoch:                       nextEpoch,
		InvalidServiceEventIncorporated: s.InvalidStateTransitionAttempted,
	}
}

func (s *dynamicProtocolStateAdapter) Identities() flow.IdentityList {
	return s.RichProtocolStateEntry.Identities
}

func (s *dynamicProtocolStateAdapter) GlobalParams() protocol.GlobalParams {
	return s.params
}
