package network

import "fmt"

// Misbehavior is a type of misbehavior that can be reported by a node.
// The misbehavior is used to penalize the misbehaving node at the protocol level.
type Misbehavior string

const (
	// StaleMessage is a misbehavior that is reported when an engine receives a message that is deemed stale based on the
	// local view of the engine.
	StaleMessage Misbehavior = "misbehavior-stale-message"

	// HeavyRequest is a misbehavior that is reported when an engine receives a request that takes an unreasonable amount
	// of resources by the engine to process, e.g., a request for a large number of blocks. The decision to consider a
	// request heavy is up to the engine.
	HeavyRequest Misbehavior = "misbehavior-heavy-request"

	// RedundantMessage is a misbehavior that is reported when an engine receives a message that is redundant, i.e., the
	// message is already known to the engine. The decision to consider a message redundant is up to the engine.
	RedundantMessage Misbehavior = "misbehavior-redundant-message"

	// UnsolicitedMessage is a misbehavior that is reported when an engine receives a message that is not solicited by the
	// engine. The decision to consider a message unsolicited is up to the engine.
	UnsolicitedMessage Misbehavior = "misbehavior-unsolicited-message"
)

// defaultPenaltyValue is the default penalty value for misbehaving nodes.
// Each reported infringement will be penalized by this value.
const defaultPenaltyValue = -864

type MisbehaviorReport struct {
	reason  Misbehavior
	penalty int
}

// MisbehaviorReportOpt is an option that can be used to configure a misbehavior report.
type MisbehaviorReportOpt func(r *MisbehaviorReport) error

// WithPenaltyAmplification returns an option that can be used to amplify the penalty value.
// The penalty value is multiplied by the given value. The value should be between 1-100.
// If the value is not in the range, an error is returned.
// The returned error by this option indicates that the option is not applied. In BFT setup, the returned error
// should be treated as a fatal error.
func WithPenaltyAmplification(v int) MisbehaviorReportOpt {
	return func(r *MisbehaviorReport) error {
		if v < 0 || v > 100 {
			return fmt.Errorf("penalty value should be between 1-100: %d", v)
		}
		r.penalty *= v
		return nil
	}
}

// NewMisbehaviorReport creates a new misbehavior report with the given reason and options.
// If no options are provided, the default penalty value is used.
// The returned error by this function indicates that the report is not created. In BFT setup, the returned error
// should be treated as a fatal error.
func NewMisbehaviorReport(reason Misbehavior, opts ...MisbehaviorReportOpt) (*MisbehaviorReport, error) {
	m := &MisbehaviorReport{
		reason:  reason,
		penalty: defaultPenaltyValue,
	}

	for _, opt := range opts {
		if err := opt(m); err != nil {
			return nil, fmt.Errorf("failed to apply misbehavior report option: %w", err)
		}
	}

	return m, nil
}
