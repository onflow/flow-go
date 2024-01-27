package proxies

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/state_synchronization"
)

var _ state_synchronization.IndexReporter = (*IndexReporter)(nil)

// IndexReporter is a proxy for the IndexReporter object that can be initialized at any time.
type IndexReporter struct {
	reporter atomic.Pointer[state_synchronization.IndexReporter]
}

func NewIndexReporter() *IndexReporter {
	return &IndexReporter{
		reporter: atomic.Pointer[state_synchronization.IndexReporter]{},
	}
}

// Initialize initializes the underlying IndexReporter
// This method can be called at any time after the IndexReporter object is created. Until it is called,
// all calls to LowestIndexedHeight and HighestIndexedHeight will return ErrDataNotAvailable
func (r *IndexReporter) Initialize(registers state_synchronization.IndexReporter) error {
	if r.reporter.CompareAndSwap(nil, &registers) {
		return nil
	}
	return fmt.Errorf("registers already initialized")
}

// LowestIndexedHeight returns the lowest height indexed by the execution state indexer.
// If the IndexReporter has not been initialized, this method will return ErrDataNotAvailable
func (r *IndexReporter) LowestIndexedHeight() (uint64, error) {
	reporter := r.reporter.Load()
	if reporter == nil {
		return 0, execution.ErrDataNotAvailable
	}

	return (*reporter).LowestIndexedHeight()
}

// HighestIndexedHeight returns the highest height indexed by the execution state indexer.
// Expected errors:
// - ErrDataNotAvailable: if the IndexReporter has not been initialized
func (r *IndexReporter) HighestIndexedHeight() (uint64, error) {
	reporter := r.reporter.Load()
	if reporter == nil {
		return 0, execution.ErrDataNotAvailable
	}

	return (*reporter).HighestIndexedHeight()
}
