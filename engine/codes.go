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
	ConsensusPropagation = 10

	Execution           = 100
	SimulationGenerator = 200
	SimulationColdstuff = 201

	// Verification 150-199
	VerificationVerifier = 150
)
