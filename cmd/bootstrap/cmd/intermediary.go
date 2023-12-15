package cmd

import (
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

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
