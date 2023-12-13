package bootstrap

// Params stores the subset of protocol.GlobalParams which can be independently configured
// by the network operator (i.e. which is not dependent on other bootstrapping artifacts,
// like the root block).
// This is used to pass data between the rootblock command and the finalize command.
type Params struct {
	ProtocolVersion            uint
	EpochCommitSafetyThreshold uint64
}
