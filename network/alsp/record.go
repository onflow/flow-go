package alsp

import (
	"fmt"

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
	Decay float64

	// CutoffCounter is a counter that is used to determine how many times the misbehaving node has been slashed due to
	// its Penalty value dropping below the disallow-listing threshold.
	CutoffCounter uint64

	// total Penalty value of the misbehaving node. Should be a negative value.
	Penalty float64
}

// RecordAdjustFunc is a function that is used to adjust the fields of a ProtocolSpamRecord.
// The function is called with the current record and should return the adjusted record.
// Returned error indicates that the adjustment is not applied, and the record should not be updated.
// In BFT setup, the returned error should be treated as a fatal error.
type RecordAdjustFunc func(ProtocolSpamRecord) (ProtocolSpamRecord, error)

// NewProtocolSpamRecord creates a new protocol spam record with the given origin id and Penalty value.
// The Decay speed of the record is set to the initial Decay speed. The CutoffCounter value is set to zero.
// The Penalty value should be a negative value.
// If the Penalty value is not a negative value, an error is returned. The error is irrecoverable and indicates a
// bug.
func NewProtocolSpamRecord(originId flow.Identifier, penalty float64) (*ProtocolSpamRecord, error) {
	if penalty >= 0 {
		return nil, fmt.Errorf("penalty value should be negative: %f", penalty)
	}

	return &ProtocolSpamRecord{
		OriginId:      originId,
		Decay:         initialDecaySpeed,
		CutoffCounter: uint64(0),
		Penalty:       penalty,
	}, nil
}
