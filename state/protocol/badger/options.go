// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

// SetClusters allows us to specify the number of clusters used by the protocol
// state to divide collections nodes into subsets.
func SetClusters(clusters uint) func(*State) {
	return func(s *State) {
		s.clusters = clusters
	}
}

// SetValidationBlocks sets the number of blocks we look back to check
// backwards for duplicate guarantees/seals.
func SetValidationBlocks(validationBlocks uint64) func(*State) {
	return func(s *State) {
		s.validationBlocks = validationBlocks
	}
}
