package model

import (
	"github.com/onflow/flow-go/model/flow"
)

// ProtocolSpamRecord is a record of a misbehaving node. It is used to keep track of the Penalty value of the node
// and the number of times it has been slashed due to its Penalty value dropping below the disallow-listing threshold.
type ProtocolSpamRecord struct {
	// OriginId is the node id of the misbehaving node. It is assumed an authorized (i.e., staked) node at the
	// time of the misbehavior report creation (otherwise, the networking layer should not have dispatched the
	// message to the Flow protocol layer in the first place).
	OriginId flow.Identifier

	// Decay speed of Penalty for this misbehaving node. Each node may have a different Decay speed based on its behavior.
	// Subsequent disallow listings of the node will decrease the Decay speed of the node so it will take longer to be allow-listed.
	Decay float64

	// CutoffCounter is a counter that is used to determine how many times the connections to the node has been cut due to
	// its Penalty value dropping below the disallow-listing threshold.
	// Note that the cutoff connections are recovered after a certain amount of time.
	CutoffCounter uint64

	// DisallowListed indicates whether the node is currently disallow-listed or not. When a node is in the disallow-list,
	// the existing connections to the node are cut and no new connections are allowed to be established, neither incoming
	// nor outgoing.
	DisallowListed bool

	// total Penalty value of the misbehaving node. Should be a negative value.
	Penalty float64
}

// RecordAdjustFunc is a function that is used to adjust the fields of a ProtocolSpamRecord.
// The function is called with the current record and should return the adjusted record.
// Returned error indicates that the adjustment is not applied, and the record should not be updated.
// In BFT setup, the returned error should be treated as a fatal error.
type RecordAdjustFunc func(ProtocolSpamRecord) (ProtocolSpamRecord, error)

// SpamRecordFactoryFunc is a function that creates a new protocol spam record with the given origin id and initial values.
// Args:
// - originId: the origin id of the spam record.
// Returns:
// - ProtocolSpamRecord, the created record.
type SpamRecordFactoryFunc func(flow.Identifier) ProtocolSpamRecord

// SpamRecordFactory returns the default factory function for creating a new protocol spam record.
// Returns:
// - SpamRecordFactoryFunc, the default factory function.
// Note that the default factory function creates a new record with the initial values.
func SpamRecordFactory() SpamRecordFactoryFunc {
	return func(originId flow.Identifier) ProtocolSpamRecord {
		return ProtocolSpamRecord{
			OriginId:       originId,
			Decay:          InitialDecaySpeed,
			DisallowListed: false,
			CutoffCounter:  uint64(0),
			Penalty:        float64(0),
		}
	}
}
