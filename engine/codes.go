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
	CollectionIngest   = 10
	CollectionProposal = 50 // to consensus ingestion
	CollectionProvider = 12

	// Observation 030-049
	// ...

	// Consensus 050-099
	ConsensusIngestion   = 50
	ConsensusPropagation = 51
	ConsensusExpulsion   = 52

	// Execution 100-199
	ExecutionExecution = 100

	// Testing 200-255
	SimulationGenerator = 200
	SimulationColdstuff = 201

	// Verification 150-199
	VerificationVerifier = 150
)
