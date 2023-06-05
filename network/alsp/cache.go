package alsp

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/alsp/model"
)

const (
	// DefaultSpamRecordCacheSize is the default size of the spam record cache.
	// It should be as big as the number of authorized nodes in Flow network.
	// Recommendation: for small network sizes 10 * number of authorized nodes to ensure that the cache can hold all the spam records of the authorized nodes.
	DefaultSpamRecordCacheSize = 10 * 1000 // considering max 1000 authorized (staked) nodes in the network.

	// DefaultSpamReportQueueSize is the default size of the queue that stores the spam records to be processed by the
	// worker pool. The queue size should be large enough to handle the spam records during attacks. The recommended
	// size is 100 * number of nodes in the network. By default, the ALSP module will disallow-list the misbehaving
	// node after 100 spam reports are received (if no penalty value are amplified). Therefore, the queue size should
	// be at least 100 * number of nodes in the network.
	DefaultSpamReportQueueSize = 100 * 1000 // considering max 1000 authorized (staked) nodes in the network.

	// DefaultHeartBeatInterval is the default heartbeat interval for the misbehavior report manager.
	// The heartbeat interval is the interval between two consecutive heartbeats. The heartbeat is used to
	// perform the periodic tasks, such as decaying the penalty of the misbehaving nodes.
	// It is always recommended to use this default value as it is part of the ALSP protocol invariants.
	DefaultHeartBeatInterval = 1 * time.Second
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
