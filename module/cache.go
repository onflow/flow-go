package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// FinalizedHeaderCache is a cache of the latest finalized block header.
type FinalizedHeaderCache interface {
	Get() *flow.Header
}
