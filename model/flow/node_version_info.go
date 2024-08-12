package flow

// CompatibleRange contains the first and the last height that the version supports.
type CompatibleRange struct {
	// The first block that the version supports.
	StartHeight uint64
	// The last block that the version supports.
	EndHeight uint64
}

// NodeVersionInfo contains information about node, such as semver, commit, sporkID, protocolVersion, etc
type NodeVersionInfo struct {
	Semver               string
	Commit               string
	SporkId              Identifier
	ProtocolVersion      uint64
	SporkRootBlockHeight uint64
	NodeRootBlockHeight  uint64
	CompatibleRange      *CompatibleRange
}
