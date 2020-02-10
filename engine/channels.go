// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

// Enum of engine IDs to avoid accidental conflicts.
const (

	// Reserved 000-009

	// Collection 010-029
	CollectionProvider = 10
	CollectionIngest   = 11
	CollectionProposal = 12

	// Observation 030-049

	// Consensus 050-099
	BlockProvider    = 50
	BlockPropagation = 51

	// Execution 100-199
	ReceiptProvider        = 100
	ExecutionStateProvider = 101
	ExecutionExecution     = 102
	ExecutionSync          = 103

	// Verification 150-199
	ApprovalProvider = 150

	// Testing 200-255
	SimulationColdstuff = 200
)
