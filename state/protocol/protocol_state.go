package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// InitialProtocolState returns constant data for given epoch, this interface can be obtained
// only for epochs that have progressed to epoch commit event.
type InitialProtocolState interface {
	// Epoch returns counter of epoch
	Epoch() uint64
	// Clustering returns initial clustering from epoch setup
	Clustering() flow.ClusterList
	// EpochSetup returns original epoch setup event that was used to initialize protocol state.
	EpochSetup() flow.EpochSetup
	// EpochCommit returns original epoch commit event that was used to update protocol state.
	EpochCommit() flow.EpochCommit
	// DKG returns information about DKG that was obtained from EpochCommit event.
	DKG() DKG
}
