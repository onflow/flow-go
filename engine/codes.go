// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

// Enum of engine IDs to avoid accidental conflicts.
const (
	// Reserved 000-009
	// ...

	// Collection 010-029
	CollectionIngest   = 10
	CollectionProposal = 11
	CollectionProvider = 12

	// Observation 030-049
	// ...

	// Consensus 050-099
	ConsensusPropagation = 50

	// Execution 100-199
	// ...

	// Verification 150-199
	VerificationVerifier = 150

	// Testing 200-255
	SimulationGenerator = 200
	SimulationColdstuff = 201
)
