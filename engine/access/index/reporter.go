package index

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go/module/state_synchronization"
)

var _ state_synchronization.IndexReporter = (*Reporter)(nil)

// Reporter implements a wrapper around `IndexReporter` ensuring that needed data has been synced and is available to the client.
// Note: `Reporter` is created with empty reporter due to the next reasoning:
// When the index is initially bootstrapped, the indexer needs to load an execution state checkpoint from
// disk and index all the data. This process can take more than 1 hour on some systems. Consequently, the Initialize
// pattern is implemented to enable the Access API to start up and serve queries before the index is fully ready. During
// the initialization phase, all calls to retrieve data from this struct should return indexer.ErrIndexNotInitialized.
// The caller is responsible for handling this error appropriately for the method.
type Reporter struct {
	reporter *atomic.Pointer[state_synchronization.IndexReporter]
}

func NewReporter() *Reporter {
	return &Reporter{
		reporter: atomic.NewPointer[state_synchronization.IndexReporter](nil),
	}
}

// Initialize replaces a previously non-initialized reporter. Can be called once.
// No errors are expected during normal operations.
func (s *Reporter) Initialize(indexReporter state_synchronization.IndexReporter) error {
	if s.reporter.CompareAndSwap(nil, &indexReporter) {
		return nil
	}
	return fmt.Errorf("index reporter already initialized")
}

// LowestIndexedHeight returns the lowest height indexed by the execution state indexer.
// Expected errors:
// - indexer.ErrIndexNotInitialized if the IndexReporter has not been initialized
func (s *Reporter) LowestIndexedHeight() (uint64, error) {
	reporter, err := s.getReporter()
	if err != nil {
		return 0, err
	}

	return reporter.LowestIndexedHeight()
}

// HighestIndexedHeight returns the highest height indexed by the execution state indexer.
// Expected errors:
// - indexer.ErrIndexNotInitialized if the IndexReporter has not been initialized
func (s *Reporter) HighestIndexedHeight() (uint64, error) {
	reporter, err := s.getReporter()
	if err != nil {
		return 0, err
	}

	return reporter.HighestIndexedHeight()
}

// checkDataAvailability checks the availability of data at the given height by comparing it with the highest and lowest
// indexed heights. If the height is beyond the indexed range, an error is returned.
// Expected errors:
//   - indexer.ErrIndexNotInitialized if the `IndexReporter` has not been initialized
//   - storage.ErrHeightNotIndexed if the block at the provided height is not indexed yet
//   - all other errors are unexpected
func (s *Reporter) checkDataAvailability(height uint64) error {
	reporter, err := s.getReporter()
	if err != nil {
		return err
	}

	highestHeight, err := reporter.HighestIndexedHeight()
	if err != nil {
		return fmt.Errorf("could not get highest indexed height: %w", err)
	}
	if height > highestHeight {
		return fmt.Errorf("%w: block not indexed yet", storage.ErrHeightNotIndexed)
	}

	lowestHeight, err := reporter.LowestIndexedHeight()
	if err != nil {
		return fmt.Errorf("could not get lowest indexed height: %w", err)
	}
	if height < lowestHeight {
		return fmt.Errorf("%w: block is before lowest indexed height", storage.ErrHeightNotIndexed)
	}

	return nil
}

// getReporter retrieves the current index reporter instance from the atomic pointer.
// Expected errors:
//   - indexer.ErrIndexNotInitialized if the reporter is not initialized
func (s *Reporter) getReporter() (state_synchronization.IndexReporter, error) {
	reporter := s.reporter.Load()
	if reporter == nil {
		return nil, indexer.ErrIndexNotInitialized
	}
	return *reporter, nil
}
