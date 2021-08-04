package hotstuff

import "github.com/onflow/flow-go/model/flow"

// VoteCollectors holds a map from block ID to vote collector. It manages the concurrent access
// to the map.
type VoteCollectors interface {
	// Get finds a vote collector by the block ID.
	// It returns the vote collector and true if found,
	// It returns nil and false if not found
	GetOrCreate(blockID flow.Identifier) (VoteCollector, bool)

	// Adding a newly created vote collector to the collectors map.
	// It returns whether it was added, which should always be true.
	// Because the EventHandler will ensure it only call with a certain
	// block once. If it returns false, it should be a fatal error
	Add(collector VoteCollector) bool
}
