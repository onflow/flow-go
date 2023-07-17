package protocol_state

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

type dynamicProtocolStateAdaptor struct {
	*flow.RichProtocolStateEntry
	dkg protocol.DKG
}

func newDynamicProtocolStateAdaptor(entry *flow.RichProtocolStateEntry) (*dynamicProtocolStateAdaptor, error) {
	dkg, err := inmem.EncodableDKGFromEvents(entry.CurrentEpochSetup, entry.CurrentEpochCommit)
	if err != nil {
		return nil, fmt.Errorf("could not construct encodable DKG from events: %w", err)
	}
	return &dynamicProtocolStateAdaptor{
		RichProtocolStateEntry: entry,
		dkg:                    inmem.NewDKG(dkg),
	}, nil
}

var _ protocol.DynamicProtocolState = (*dynamicProtocolStateAdaptor)(nil)

func (s *dynamicProtocolStateAdaptor) Epoch() uint64 {
	return s.CurrentEpochSetup.Counter
}

func (s *dynamicProtocolStateAdaptor) Clustering() flow.ClusterList {
	panic("implement me")
}

func (s *dynamicProtocolStateAdaptor) EpochSetup() *flow.EpochSetup {
	return s.CurrentEpochSetup
}

func (s *dynamicProtocolStateAdaptor) EpochCommit() *flow.EpochCommit {
	return s.CurrentEpochCommit
}

func (s *dynamicProtocolStateAdaptor) DKG() protocol.DKG {
	return s.dkg
}

func (s *dynamicProtocolStateAdaptor) Identities() flow.IdentityList {
	return s.RichProtocolStateEntry.Identities
}

func (s *dynamicProtocolStateAdaptor) GlobalParams() protocol.GlobalParams {
	//TODO implement me
	panic("implement me")
}
