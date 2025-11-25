package protocol

// BlockTimer constructs and validates block timestamps.
type BlockTimer interface {
	// Build generates a timestamp based on definition of valid timestamp.
	Build(parentTimestamp uint64) uint64
	// Validate checks validity of a block's time stamp.
	// Error returns
	//  * `model.InvalidBlockTimestampError` if time stamp is invalid.
	//  * all other errors are unexpected and potentially symptoms of internal implementation bugs or state corruption (fatal).
	Validate(parentTimestamp, currentTimestamp uint64) error
}
