// (c) 2020 Dapper Labs - ALL RIGHTS RESERVED
package committee

import (
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
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

func NewMetricsWrapper(committee hotstuff.Committee, metrics module.HotstuffMetrics) *CommitteeMetricsWrapper {
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

func (w CommitteeMetricsWrapper) DKGSize(blockID flow.Identifier) (uint, error) {
	processStart := time.Now()
	size, err := w.committee.DKGSize(blockID)
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return size, err
}

func (w CommitteeMetricsWrapper) DKGGroupKey(blockID flow.Identifier) (crypto.PublicKey, error) {
	processStart := time.Now()
	groupKey, err := w.committee.DKGGroupKey(blockID)
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return groupKey, err
}

func (w CommitteeMetricsWrapper) DKGIndex(blockID flow.Identifier, nodeID flow.Identifier) (uint, error) {
	processStart := time.Now()
	index, err := w.committee.DKGIndex(blockID, nodeID)
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return index, err
}

func (w CommitteeMetricsWrapper) DKGKeyShare(blockID flow.Identifier, nodeID flow.Identifier) (crypto.PublicKey, error) {
	processStart := time.Now()
	groupKey, err := w.committee.DKGKeyShare(blockID, nodeID)
	w.metrics.CommitteeProcessingDuration(time.Since(processStart))
	return groupKey, err
}
