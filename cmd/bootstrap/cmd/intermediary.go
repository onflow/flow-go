package cmd

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
)

// IntermediaryBootstrappingData stores data which needs to be passed between the
// 2 steps of the bootstrapping process: `rootblock` and `finalize`.
// This structure is created in `rootblock`, written to disk, then read in `finalize`.
type IntermediaryBootstrappingData struct {
	IntermediaryParamsData
	IntermediaryEpochData
}

// IntermediaryParamsData stores the subset of protocol.GlobalParams which can be independently configured
// by the network operator (i.e. which is not dependent on other bootstrapping artifacts,
// like the root block).
// This is used to pass data between the rootblock command and the finalize command.
type IntermediaryParamsData struct {
	ProtocolVersion            uint
	EpochCommitSafetyThreshold uint64
	EpochExtensionViewCount    uint64
}

// IntermediaryEpochData stores the root epoch and the epoch config for the execution state
// and to bootstrap the Protocol State.
// This is used to pass data between the rootblock command and the finalize command.
type IntermediaryEpochData struct {
	// TODO remove redundant inclusion of the fields (currently storing them in cadence as well as in protocol-state representation).
	ExecutionStateConfig epochs.EpochConfig
	RootEpochSetup       *flow.EpochSetup
	RootEpochCommit      *flow.EpochCommit
}
