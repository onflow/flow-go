package hotstuff

import (
	"time"
)

// BlockTimestamp constructs and validates block timestamps. 
// Let τ be the time stamp of the parent block and t be the current clock time of the proposer that is building the child block
// An honest proposer sets the Timestamp of its proposal according to the following rule:
// if t is within the interval [τ + minInterval, τ + maxInterval], then the proposer sets Timestamp := t
// otherwise, the proposer chooses the time stamp from the interval that is closest to its current time t, i.e.
// if t < τ + minInterval, the proposer sets Timestamp := τ + minInterval
// if τ + maxInterval < t, the proposer sets Timestamp := τ + maxInterval
type BlockTimestamp interface {
	// Build generates a timestamp based on definition of valid timestamp.
	Build(parentTimestamp time.Time) time.Time
	// Validate checks validity of a block's time stamp.
	// Error returns
	//  * `model.InvalidBlockTimestampError` if time stamp is invalid. 
	//  * all other errors are unexpected and potentially symptoms of internal implementation bugs or state corruption (fatal). 
	Validate(parentTimestamp, currentTimestamp time.Time) error
}
