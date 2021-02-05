package chunks

import (
	"github.com/onflow/flow-go/model/flow"
)

// Locator is used to locate a chunk by providing the execution result the chunk belongs to as well as the chunk index within that execution result.
type Locator struct {
	ResultID flow.Identifier // execution result id that chunk belongs to
	Index    uint64          // index of chunk in the execution result
}

// ID returns a unique id for chunk locator.
func (c Locator) ID() flow.Identifier {
	return flow.MakeID(c)
}

// Checksum provides a cryptographic commitment for a chunk locator content.
func (c Locator) Checksum() flow.Identifier {
	return flow.MakeID(c)
}

type LocatorList []*Locator
