package internal

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// ReportedMisbehaviorWork is an internal data structure for "temporarily" storing misbehavior reports in the queue
// till they are processed by the worker.
type ReportedMisbehaviorWork struct {
	// OriginID is the ID of the peer that the misbehavior report is about.
	OriginID flow.Identifier

	// Reason is the reason of the misbehavior.
	Reason network.Misbehavior

	// Penalty is the penalty value of the misbehavior.
	Penalty float64
}
