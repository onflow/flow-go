// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

import "fmt"

// Enum of channel IDs to avoid accidental conflicts.
const (

	// Reserved 000-009

	// Collection 010-029
	CollectionProvider             = 10 // providing collections/transactions to non-collection nodes
	CollectionIngest               = 11 // ingesting transactions and routing to appropriate cluster
	ProtocolClusterConsensus       = 20 // cluster-specific consensus protocol
	ProtocolClusterSynchronization = 21 // cluster-specific consensus synchronization

	// Observation 030-049

	// Consensus 050-099
	BlockProvider           = 50 // providing blocks to non-consensus nodes
	BlockPropagation        = 51 // propagating entities to be included in blocks between consensus nodes
	ProtocolConsensus       = 60 // consensus protocol
	ProtocolSynchronization = 66 // synchronization protocol

	// Execution 100-199
	ExecutionReceiptProvider = 100
	ExecutionStateProvider   = 101
	ExecutionComputer        = 102
	ChunkDataPackProvider    = 103
	ExecutionSync            = 104

	// Verification 150-199
	ApprovalProvider = 150

	// Testing 200-255
	SimulationColdstuff = 200
)

func ChannelName(channelID uint8) string {
	switch channelID {
	case CollectionProvider:
		return "CollectionProvider"
	case CollectionIngest:
		return "CollectionIngest"
	case ProtocolClusterConsensus:
		return "ProtocolClusterConsensus"
	case ProtocolClusterSynchronization:
		return "ProtocolClusterSynchronization"
	case BlockProvider:
		return "BlockProvider"
	case BlockPropagation:
		return "BlockPropagation"
	case ProtocolConsensus:
		return "ProtocolConsensus"
	case ProtocolSynchronization:
		return "ProtocolSynchronization"
	case ExecutionReceiptProvider:
		return "ExecutionReceiptProvider"
	case ExecutionStateProvider:
		return "ExecutionStateProvider"
	case ExecutionComputer:
		return "ExecutionComputer"
	case ChunkDataPackProvider:
		return "ChunkDataPackProvider"
	case ExecutionSync:
		return "ExecutionSync"
	case ApprovalProvider:
		return "ApprovalProvider"
	case SimulationColdstuff:
		return "SimulationColdstuff"
	}
	return fmt.Sprintf("unknown-channel-%d", channelID)
}
