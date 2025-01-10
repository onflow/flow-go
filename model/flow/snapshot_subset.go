package flow

// ProtocolSnapshotExecutionSubset is a subset of the protocol state snapshot that is needed by the FVM
// for execution.
type ProtocolSnapshotExecutionSubset interface {
	// RandomSource provides a source of entropy that can be
	// expanded into randoms (using a pseudo-random generator).
	// The returned slice should have at least 128 bits of entropy.
	// The function doesn't error in normal operations, any
	// error should be treated as an exception.
	//
	// `protocol.ProtocolSnapshotExecutionSubset` implements `EntropyProvider` interface
	// Note that `ProtocolSnapshotExecutionSubset` possible errors for RandomSource() are:
	// - storage.ErrNotFound if the QC is unknown.
	// - state.ErrUnknownSnapshotReference if the snapshot reference block is unknown
	// However, at this stage, snapshot reference block should be known and the QC should also be known,
	// so no error is expected in normal operations, as required by `EntropyProvider`.
	RandomSource() ([]byte, error)

	// VersionBeacon returns the latest sealed version beacon.
	// If no version beacon has been sealed so far during the current spork, returns nil.
	// The latest VersionBeacon is only updated for finalized blocks. This means that, when
	// querying an un-finalized fork, `VersionBeacon` will have the same value as querying
	// the snapshot for the latest finalized block, even if a newer version beacon is included
	// in a seal along the un-finalized fork.
	//
	// The SealedVersionBeacon must contain at least one entry. The first entry is for a past block height.
	// The remaining entries are for all future block heights. Future version boundaries
	// can be removed, in which case the emitted event will not contain the removed version
	// boundaries.
	VersionBeacon() (*SealedVersionBeacon, error)
}

// ProtocolSnapshotExecutionSubsetProvider is an interface that provides a subset of the protocol state
// at a specific block.
type ProtocolSnapshotExecutionSubsetProvider interface {
	AtBlockID(blockID Identifier) ProtocolSnapshotExecutionSubset
}
