// (c) 2020 Dapper Labs - ALL RIGHTS RESERVED
package committees

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// CommitteeMetricsWrapper implements the hotstuff.Committee interface.
// It wraps a hotstuff.Committee instance and measures the time which the HotStuff's core logic
// spends in the hotstuff.Committee component, i.e. the time determining consensus committee
// relations. The measured time durations are reported as values for the
// CommitteeProcessingDuration metric.
type CommitteeMetricsWrapper struct {
	committee hotstuff.Committee
	metrics   module.HotstuffMetrics
}

var _ hotstuff.VoterCommittee = (*CommitteeMetricsWrapper)(nil)
var _ hotstuff.Committee = (*CommitteeMetricsWrapper)(nil)

func NewMetricsWrapper(committee hotstuff.Committee, metrics module.HotstuffMetrics) *CommitteeMetricsWrapper {
	return &CommitteeMetricsWrapper{
		committee: committee,
		metrics:   metrics,
	}
}

func (w CommitteeMetricsWrapper) IdentitiesByBlock(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {
	processStart := time.Now()
	identities, err := w.committee.IdentitiesByBlock(blockID, selector)
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return identities, err
}

func (w CommitteeMetricsWrapper) IdentityByBlock(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error) {
	processStart := time.Now()
	identity, err := w.committee.IdentityByBlock(blockID, participantID)
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return identity, err
}

func (w CommitteeMetricsWrapper) IdentitiesByEpoch(view uint64, selector flow.IdentityFilter) (flow.IdentityList, error) {
	processStart := time.Now()
	identities, err := w.committee.IdentitiesByEpoch(view, selector)
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return identities, err
}

func (w CommitteeMetricsWrapper) IdentityByEpoch(view uint64, participantID flow.Identifier) (*flow.Identity, error) {
	processStart := time.Now()
	identity, err := w.committee.IdentityByEpoch(view, participantID)
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

func (w CommitteeMetricsWrapper) DKG(view uint64) (hotstuff.DKG, error) {
	processStart := time.Now()
	dkg, err := w.committee.DKG(view)
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return dkg, err
}
