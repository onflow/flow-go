package fetcher

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/module"
)

type AssignedChunkProcessor interface {
	ProcessAssignedChunk(locator *chunks.Locator)

	// WithProcessingNotifier
	WithProcessingNotifier(notifier module.ProcessingNotifier)
}
