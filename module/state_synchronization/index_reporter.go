package state_synchronization

// IndexReporter provides information about the current state of the execution state indexer.
type IndexReporter interface {
	// LowestIndexedHeight returns the lowest height indexed by the execution state indexer.
	LowestIndexedHeight() (uint64, error)
	// HighestIndexedHeight returns the highest height indexed by the execution state indexer.
	HighestIndexedHeight() (uint64, error)
}
