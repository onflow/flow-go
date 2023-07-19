package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// InitialProtocolState returns constant data for given epoch.
// This interface can be only obtained for epochs that have progressed to epoch commit event.
type InitialProtocolState interface {
	// Epoch returns counter of epoch.
	Epoch() uint64
	// Clustering returns initial clustering from epoch setup.
	// No errors are expected during normal operations.
	Clustering() (flow.ClusterList, error)
	// EpochSetup returns original epoch setup event that was used to initialize the protocol state.
	EpochSetup() *flow.EpochSetup
	// EpochCommit returns original epoch commit event that was used to update the protocol state.
	EpochCommit() *flow.EpochCommit
	// DKG returns information about DKG that was obtained from EpochCommit event.
	// No errors are expected during normal operations.
	DKG() (DKG, error)
}

// DynamicProtocolState extends the InitialProtocolState with data that can change from block to block.
// It can be used to access the identity table at given block.
type DynamicProtocolState interface {
	InitialProtocolState
	// Identities returns identities that can participate in current and next epochs.
	// Set of Authorized identities are different depending on epoch state:
	// staking phase - identities for current epoch + identities from previous epoch (with 0 weight)
	// setup & commit phase - identities for current epoch + identities from next epoch (with 0 weight)
	Identities() flow.IdentityList
	// GlobalParams returns params that are same for all nodes in the network.
	GlobalParams() GlobalParams
}

// ProtocolState is the read-only interface for protocol state, it allows to query information
// on a per-block and per-epoch basis.
type ProtocolState interface {

	// ByEpoch returns an object with static protocol state information by epoch number.
	// To be able to use this interface we need to observe both epoch setup and commit events.
	// Not available for next epoch unless we have observed an EpochCommit event.
	// No errors are expected during normal operations.
	// TODO(yuraolex): check return types
	// TODO(yuraolex): decide if we really need this approach. It's unclear if it's useful to query
	//  by epoch counter. To implement it we need an additional index by epoch counter. Alternatively we need a way to map
	//  epoch counter -> block ID. It gets worse if we consider that we need a way to get the epoch counter itself at caller side.
	//ByEpoch(epoch uint64) (InitialProtocolState, error)

	// AtBlockID returns protocol state at block ID.
	// Resulting protocol state is returned AFTER applying updates that are contained in block.
	// Can be queried for any block that has been added to the block tree.
	// No errors are expected during normal operations.
	// TODO(yuraolex): check return types
	AtBlockID(blockID flow.Identifier) (DynamicProtocolState, error)
	// GlobalParams returns params that are same for all nodes in the network.
	GlobalParams()
}
