package alsp

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/alsp/model"
)

// MisbehaviorReport is a report that is sent to the networking layer to penalize the misbehaving node.
// A MisbehaviorReport reports the misbehavior of a node on sending a message to the current node that appears valid
// based on the networking layer but is considered invalid by the current node based on the Flow protocol.
//
// A MisbehaviorReport consists of a reason and a penalty. The reason is a string that describes the misbehavior.
// The penalty is a value that is deducted from the overall score of the misbehaving node. The score is
// decayed at each decay interval. If the overall penalty of the misbehaving node drops below the disallow-listing
// threshold, the node is reported to be disallow-listed by the networking layer, i.e., existing connections to the
// node are closed and the node is no longer allowed to connect till its penalty is decayed back to zero.
type MisbehaviorReport struct {
	id      flow.Identifier     // the ID of the misbehaving node
	reason  network.Misbehavior // the reason of the misbehavior
	penalty float64             // the penalty value of the misbehavior
}

var _ network.MisbehaviorReport = (*MisbehaviorReport)(nil)

// MisbehaviorReportOpt is an option that can be used to configure a misbehavior report.
type MisbehaviorReportOpt func(r *MisbehaviorReport) error

// WithPenaltyAmplification returns an option that can be used to amplify the penalty value.
// The penalty value is multiplied by the given value. The value should be between 1-100.
// If the value is not in the range, an error is returned.
// The returned error by this option indicates that the option is not applied. In BFT setup, the returned error
// should be treated as a fatal error.
func WithPenaltyAmplification(v float64) MisbehaviorReportOpt {
	return func(r *MisbehaviorReport) error {
		if v <= 0 || v > 100 {
			return fmt.Errorf("penalty value should be between 1-100: %v", v)
		}
		r.penalty *= v
		return nil
	}
}

// OriginId returns the ID of the misbehaving node.
func (r MisbehaviorReport) OriginId() flow.Identifier {
	return r.id
}

// Reason returns the reason of the misbehavior.
func (r MisbehaviorReport) Reason() network.Misbehavior {
	return r.reason
}

// Penalty returns the penalty value of the misbehavior.
func (r MisbehaviorReport) Penalty() float64 {
	return r.penalty
}

// NewMisbehaviorReport creates a new misbehavior report with the given reason and options.
// If no options are provided, the default penalty value is used.
// The returned error by this function indicates that the report is not created. In BFT setup, the returned error
// should be treated as a fatal error.
// The default penalty value is 0.01 * misbehaviorDisallowListingThreshold = -86.4
func NewMisbehaviorReport(misbehavingId flow.Identifier, reason network.Misbehavior, opts ...MisbehaviorReportOpt) (*MisbehaviorReport, error) {
	m := &MisbehaviorReport{
		id:      misbehavingId,
		reason:  reason,
		penalty: model.DefaultPenaltyValue,
	}

	for _, opt := range opts {
		if err := opt(m); err != nil {
			return nil, fmt.Errorf("failed to apply misbehavior report option: %w", err)
		}
	}

	return m, nil
}
