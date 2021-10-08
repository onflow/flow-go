package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// Params are parameters of the protocol state, divided into parameters of
// this specific instance of the state (varies from node to node) and global
// parameters of the state.
type Params interface {
	InstanceParams
	GlobalParams
}

// InstanceParams represents protocol state parameters that vary between instances.
// For example, two nodes both running in the same spork on Flow Mainnet may have
// different instance params.
type InstanceParams interface {

	// Root returns the root header of the current protocol state. This will be
	// the head of the protocol state snapshot used to bootstrap this state and
	// may differ from node to node for the same protocol state.
	Root() (*flow.Header, error)

	// Seal returns the root block seal of the current protocol state. This will be
	// the seal for the root block used to bootstrap this state and may differ from
	// node to node for the same protocol state.
	Seal() (*flow.Seal, error)
}

// GlobalParams represents protocol state parameters that do not vary between instances.
// Any nodes running in the same spork, on the same network (same chain ID) must
// have the same global params.
type GlobalParams interface {

	// ChainID returns the chain ID for the current Flow network. The chain ID
	// uniquely identifies a Flow network in perpetuity across epochs and sporks.
	ChainID() (flow.ChainID, error)

	// SporkID returns the unique identifier for this network within the current spork.
	// This ID is determined at the beginning of a spork during bootstrapping and is
	// part of the root protocol state snapshot.
	SporkID() (flow.Identifier, error)

	// ProtocolVersion returns the protocol version, the major software version
	// of the protocol software.
	ProtocolVersion() (uint, error)
}
