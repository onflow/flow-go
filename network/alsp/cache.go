package alsp

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/alsp/model"
)

// SpamRecordCache is a cache of spam records for the ALSP module.
// It is used to keep track of the spam records of the nodes that have been reported for spamming.
type SpamRecordCache interface {
	// Adjust applies the given adjust function to the spam record of the given origin id.
	// Returns the Penalty value of the record after the adjustment.
	// It returns an error if the adjustFunc returns an error or if the record does not exist.
	// Assuming that adjust is always called when the record exists, the error is irrecoverable and indicates a bug.
	Adjust(originId flow.Identifier, adjustFunc model.RecordAdjustFunc) (float64, error)

	// Identities returns the list of identities of the nodes that have a spam record in the cache.
	Identities() []flow.Identifier

	// Remove removes the spam record of the given origin id from the cache.
	// Returns true if the record is removed, false otherwise (i.e., the record does not exist).
	Remove(originId flow.Identifier) bool

	// Get returns the spam record of the given origin id.
	// Returns the record and true if the record exists, nil and false otherwise.
	// Args:
	// - originId: the origin id of the spam record.
	// Returns:
	// - the record and true if the record exists, nil and false otherwise.
	// Note that the returned record is a copy of the record in the cache (we do not want the caller to modify the record).
	Get(originId flow.Identifier) (*model.ProtocolSpamRecord, bool)

	// Size returns the number of records in the cache.
	Size() uint
}
