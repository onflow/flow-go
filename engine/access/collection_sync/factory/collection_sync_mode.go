package factory

import "fmt"

// CollectionSyncMode represents the mode for collection synchronization.
type CollectionSyncMode string

const (
	// CollectionSyncModeExecutionFirst fetches from execution nodes first if execution data syncing is enabled,
	// otherwise fetches from collection nodes.
	CollectionSyncModeExecutionFirst CollectionSyncMode = "execution_first"
	// CollectionSyncModeExecutionAndCollection fetches from both collection nodes and execution nodes.
	CollectionSyncModeExecutionAndCollection CollectionSyncMode = "execution_and_collection"
	// CollectionSyncModeCollectionOnly only fetches from collection nodes.
	CollectionSyncModeCollectionOnly CollectionSyncMode = "collection_only"
)

// String returns the string representation of the CollectionSyncMode.
func (m CollectionSyncMode) String() string {
	return string(m)
}

// ParseCollectionSyncMode parses a string into a CollectionSyncMode.
func ParseCollectionSyncMode(s string) (CollectionSyncMode, error) {
	switch s {
	case string(CollectionSyncModeExecutionFirst):
		return CollectionSyncModeExecutionFirst, nil
	case string(CollectionSyncModeExecutionAndCollection):
		return CollectionSyncModeExecutionAndCollection, nil
	case string(CollectionSyncModeCollectionOnly):
		return CollectionSyncModeCollectionOnly, nil
	default:
		return "", fmt.Errorf("invalid collection sync mode: %s, must be one of [execution_first, execution_and_collection, collection_only]", s)
	}
}

// ShouldCreateFetcher returns whether a collection fetcher should be created based on the sync mode
// and whether execution data sync is enabled.
//
// A fetcher should be created if:
//   - The mode is ExecutionAndCollection (always create, even with execution data sync)
//   - The mode is CollectionOnly (always create)
//   - The mode is ExecutionFirst and execution data sync is disabled
func (m CollectionSyncMode) ShouldCreateFetcher(executionDataSyncEnabled bool) bool {
	return m == CollectionSyncModeExecutionAndCollection ||
		m == CollectionSyncModeCollectionOnly ||
		(m == CollectionSyncModeExecutionFirst && !executionDataSyncEnabled)
}

// ShouldCreateExecutionDataProcessor returns whether an execution data processor should be created
// based on the sync mode and whether execution data sync is enabled.
//
// An execution data processor should be created if:
//   - Execution data sync is enabled AND
//   - The mode is NOT CollectionOnly (since CollectionOnly mode only fetches from collection nodes)
func (m CollectionSyncMode) ShouldCreateExecutionDataProcessor(executionDataSyncEnabled bool) bool {
	return executionDataSyncEnabled && m != CollectionSyncModeCollectionOnly
}
