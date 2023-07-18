package protocol_state

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type dynamicProtocolStateAdapter struct {
	*initialProtocolStateAdapter
}

var _ protocol.DynamicProtocolState = (*dynamicProtocolStateAdapter)(nil)

func newDynamicProtocolStateAdaptor(entry *flow.RichProtocolStateEntry) (*dynamicProtocolStateAdapter, error) {
	adapter, err := newInitialProtocolStateAdaptor(entry)
	if err != nil {
		return nil, fmt.Errorf("could not create internal protocol state adapter: %w", err)
	}
	return &dynamicProtocolStateAdapter{
		initialProtocolStateAdapter: adapter,
	}, nil
}

func (s *dynamicProtocolStateAdapter) Identities() flow.IdentityList {
	return s.RichProtocolStateEntry.Identities
}

func (s *dynamicProtocolStateAdapter) GlobalParams() protocol.GlobalParams {
	//TODO implement me
	panic("implement me")
}
