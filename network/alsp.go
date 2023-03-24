package network

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
)

// Misbehavior is a type of misbehavior that can be reported by a node.
// The misbehavior is used to penalize the misbehaving node at the protocol level concerning the messages that the current
// node has received from the misbehaving node.
type Misbehavior string

func (m Misbehavior) String() string {
	return string(m)
}

// MisbehaviorReporter is an interface that is used to report misbehavior of a node.
// The misbehavior is reported to the networking layer to penalize the misbehaving node.
type MisbehaviorReporter interface {
	// ReportMisbehavior reports the misbehavior of a node on sending a message to the current node that appears valid
	// based on the networking layer but is considered invalid by the current node based on the Flow protocol.
	// The misbehavior is reported to the networking layer to penalize the misbehaving node.
	// Implementation must be thread-safe and non-blocking.
	ReportMisbehavior(*MisbehaviorReport)
}

type MisbehaviorReport interface {
	// OriginId returns the ID of the misbehaving node.
	OriginId() flow.Identifier

	// Reason returns the reason of the misbehavior.
	Reason() Misbehavior

	// Penalty returns the penalty value of the misbehavior.
	Penalty() int
}

type MisbehaviorReportManager interface {
	// HandleReportedMisbehavior handles the misbehavior report that is sent by the networking layer.
	// The implementation of this function should penalize the misbehaving node and report the node to be
	// disallow-listed if the overall penalty of the misbehaving node drops below the disallow-listing threshold.
	// The implementation of this function should be thread-safe and non-blocking.
	HandleReportedMisbehavior(channels.Channel, MisbehaviorReport)
}
