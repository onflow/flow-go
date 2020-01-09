// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

// Enum of engine IDs to avoid accidental conflicts.
// Suggested coding:
// 000-009 reserved
// 010-030 collection
// 030-050 observation
// 050-099 consensus
// 100-149: execution
// 150-199: verification
// 200-255 testing
const (
	// Reserved 000-009
	// ...

	// Collection 010-029
	CollectionProvider = 10
	CollectionIngest   = 11
	CollectionProposal = 12

	// Observation 030-049
	// ...

	// Consensus 050-099
	ConsensusProvider    = 50
	ConsensusPropagation = 51

	ConsensusReceiver = CollectionProvider

	// Execution 100-199
	ExecutionExecution       = 100
	ExecutionBlockIngestion  = 101
	ExecutionReceiptProvider = 102

	// Verification 150-199
	VerificationVerifier = 150

	// Testing 200-255
	SimulationGenerator = 200
	SimulationColdstuff = 201
)
