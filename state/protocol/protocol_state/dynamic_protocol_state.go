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

func (s *dynamicProtocolStateAdapter) Identities() flow.IdentityList {
	return s.RichProtocolStateEntry.Identities
}

func (s *dynamicProtocolStateAdapter) GlobalParams() protocol.GlobalParams {
	return s.params
}
