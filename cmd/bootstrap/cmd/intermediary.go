package cmd

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
)

// IntermediaryBootstrappingData stores data which needs to be passed between the
// last 2 steps of the bootstrapping process: `rootblock` and `finalize`.
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
	FinalizationSafetyThreshold uint64
	EpochExtensionViewCount     uint64
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

// IntermediaryClusteringData stores the collector cluster assignment and epoch counter.
// This is used for the collection nodes to construct and vote on their cluster root blocks,
// and also to pass data between the clustering command and the rootblock command.
type IntermediaryClusteringData struct {
	EpochCounter uint64
	Assignments  flow.AssignmentList
	Clusters     flow.ClusterList
}
