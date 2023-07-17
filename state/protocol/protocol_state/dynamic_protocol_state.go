package protocol_state

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

type dynamicProtocolStateAdapter struct {
	*flow.RichProtocolStateEntry
	dkg protocol.DKG
}

var _ protocol.DynamicProtocolState = (*dynamicProtocolStateAdapter)(nil)

func newDynamicProtocolStateAdaptor(entry *flow.RichProtocolStateEntry) (*dynamicProtocolStateAdapter, error) {
	dkg, err := inmem.EncodableDKGFromEvents(entry.CurrentEpochSetup, entry.CurrentEpochCommit)
	if err != nil {
		return nil, fmt.Errorf("could not construct encodable DKG from events: %w", err)
	}
	return &dynamicProtocolStateAdapter{
		RichProtocolStateEntry: entry,
		dkg:                    inmem.NewDKG(dkg),
	}, nil
}

func (s *dynamicProtocolStateAdapter) Epoch() uint64 {
	return s.CurrentEpochSetup.Counter
}

func (s *dynamicProtocolStateAdapter) Clustering() flow.ClusterList {
	panic("implement me")
}

func (s *dynamicProtocolStateAdapter) EpochSetup() *flow.EpochSetup {
	return s.CurrentEpochSetup
}

func (s *dynamicProtocolStateAdapter) EpochCommit() *flow.EpochCommit {
	return s.CurrentEpochCommit
}

func (s *dynamicProtocolStateAdapter) DKG() protocol.DKG {
	return s.dkg
}

func (s *dynamicProtocolStateAdapter) Identities() flow.IdentityList {
	return s.RichProtocolStateEntry.Identities
}

func (s *dynamicProtocolStateAdapter) GlobalParams() protocol.GlobalParams {
	//TODO implement me
	panic("implement me")
}
