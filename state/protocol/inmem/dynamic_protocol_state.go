package inmem

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// DynamicProtocolStateAdapter implements protocol.DynamicProtocolState by wrapping an InitialProtocolStateAdapter.
type DynamicProtocolStateAdapter struct {
	*InitialProtocolStateAdapter
	params protocol.GlobalParams
}

var _ protocol.DynamicProtocolState = (*DynamicProtocolStateAdapter)(nil)

func NewDynamicProtocolStateAdapter(entry *flow.RichProtocolStateEntry, params protocol.GlobalParams) *DynamicProtocolStateAdapter {
	adapter := NewInitialProtocolStateAdapter(entry)
	return &DynamicProtocolStateAdapter{
		InitialProtocolStateAdapter: adapter,
		params:                      params,
	}
}

func (s *DynamicProtocolStateAdapter) EpochStatus() *flow.EpochStatus {
	return s.RichProtocolStateEntry.EpochStatus()
}

func (s *DynamicProtocolStateAdapter) Identities() flow.IdentityList {
	return s.RichProtocolStateEntry.Identities
}

func (s *DynamicProtocolStateAdapter) GlobalParams() protocol.GlobalParams {
	return s.params
}
