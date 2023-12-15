package cmd

import (
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

// IntermediaryBootstrappingData stores data which needs to be passed between the
// 2 steps of the bootstrapping process: `rootblock` and `finalize`.
// This structure is created in `rootblock`, written to disk, then read in `finalize`.
// TODO unused
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
}

// IntermediaryEpochData stores the root epoch and the epoch config for the execution state.
// This is used to pass data between the rootblock command and the finalize command.
type IntermediaryEpochData struct {
	ExecutionStateConfig   epochs.EpochConfig
	ProtocolStateRootEpoch inmem.EncodableEpoch
}
