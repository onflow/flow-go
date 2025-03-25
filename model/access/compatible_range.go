package access

// CompatibleRange contains the first and the last height that the node's version supports.
type CompatibleRange struct {
	// StartHeight is the first block that the version supports.
	StartHeight uint64

	// EndHeight is the last block that the version supports.
	EndHeight uint64
}
