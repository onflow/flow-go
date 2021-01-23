package consensus

// ResultForest is a mempool holding receipts, which is aware of the tree structure
// formed by the results. Internally it utilizes the LevelledForrest.
// Safe for concurrent access.
type ResultForest struct {
	//sync.RWMutex
	//forest forest.LevelledForest
}
