package access

import (
	"github.com/onflow/flow-go/model/flow"
)

// NodeVersionInfo contains information about node, such as semver, commit, sporkID, protocolVersion, etc
type NodeVersionInfo struct {
	Semver  string
	Commit  string
	SporkId flow.Identifier
	// ProtocolVersion is the deprecated protocol version number.
	// Deprecated: Previously this referred to the major software version as of the most recent spork.
	// Replaced by protocol_state_version.
	ProtocolVersion uint64
	// ProtocolStateVersion is the Protocol State version as of the latest finalized block.
	// This tracks the schema version of the Protocol State and is used to coordinate breaking changes in the Protocol.
	// Version numbers are monotonically increasing.
	ProtocolStateVersion uint64
	SporkRootBlockHeight uint64
	NodeRootBlockHeight  uint64
	CompatibleRange      *CompatibleRange
}
