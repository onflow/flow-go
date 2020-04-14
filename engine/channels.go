// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

// Enum of engine IDs to avoid accidental conflicts.
const (

	// Reserved 000-009

	// Collection 010-029
	CollectionProvider       = 10 // providing collections/transactions to non-collection nodes
	CollectionIngest         = 11 // ingesting transactions and routing to appropriate cluster
	ProtocolClusterConsensus = 20 // cluster-specific consensus protocol

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
