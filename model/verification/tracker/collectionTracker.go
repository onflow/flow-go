package tracker

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// CollectionTracker represents a request tracker for a collection
type CollectionTracker struct {
	BlockID      flow.Identifier
	CollectionID flow.Identifier
	Counter      uint // keeps track of number of retries
}

func (c *CollectionTracker) ID() flow.Identifier {
	return c.CollectionID
}

func (c *CollectionTracker) Checksum() flow.Identifier {
	return flow.MakeID(c)
}
