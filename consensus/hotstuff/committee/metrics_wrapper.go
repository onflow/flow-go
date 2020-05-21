// (c) 2020 Dapper Labs - ALL RIGHTS RESERVED
package committee

import (
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

// CommitteeMetricsWrapper measures the time which the HotStuff's core logic
// spends in the hotstuff.Committee component, i.e. the time determining consensus
// committee relations.
type CommitteeMetricsWrapper struct {
	committee hotstuff.Committee
	metrics   module.HotstuffMetrics
}

func NewMetricsWrapper(committee hotstuff.Committee, metrics module.HotstuffMetrics) hotstuff.Committee {
	return &CommitteeMetricsWrapper{
		committee: committee,
		metrics:   metrics,
	}
}

func (w CommitteeMetricsWrapper) Identities(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {
	processStart := time.Now()
	identities, err := w.committee.Identities(blockID, selector)
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return identities, err
}

func (w CommitteeMetricsWrapper) Identity(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error) {
	processStart := time.Now()
	identity, err := w.committee.Identity(blockID, participantID)
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return identity, err
}

func (w CommitteeMetricsWrapper) LeaderForView(view uint64) (flow.Identifier, error) {
	processStart := time.Now()
	id, err := w.committee.LeaderForView(view)
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return id, err
}

func (w CommitteeMetricsWrapper) Self() flow.Identifier {
	processStart := time.Now()
	id := w.committee.Self()
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return id
}
