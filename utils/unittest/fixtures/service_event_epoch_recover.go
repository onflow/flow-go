package fixtures

import "github.com/onflow/flow-go/model/flow"

// EpochRecover is the default options factory for [flow.EpochRecover] generation.
var EpochRecover epochRecoverFactory

type epochRecoverFactory struct{}

type EpochRecoverOption func(*EpochRecoverGenerator, *flow.EpochRecover)

// WithEpochSetup is an option that sets the `EpochSetup` of the recover event.
func (f epochRecoverFactory) WithEpochSetup(setup flow.EpochSetup) EpochRecoverOption {
	return func(g *EpochRecoverGenerator, recover *flow.EpochRecover) {
		recover.EpochSetup = setup
	}
}

// WithEpochCommit is an option that sets the `EpochCommit` of the recover event.
func (f epochRecoverFactory) WithEpochCommit(commit flow.EpochCommit) EpochRecoverOption {
	return func(g *EpochRecoverGenerator, recover *flow.EpochRecover) {
		recover.EpochCommit = commit
	}
}

// EpochRecoverGenerator generates epoch recovery events with consistent randomness.
type EpochRecoverGenerator struct {
	epochRecoverFactory

	random       *RandomGenerator
	epochSetups  *EpochSetupGenerator
	epochCommits *EpochCommitGenerator
}

// NewEpochRecoverGenerator creates a new EpochRecoverGenerator.
func NewEpochRecoverGenerator(
	random *RandomGenerator,
	epochSetups *EpochSetupGenerator,
	epochCommits *EpochCommitGenerator,
) *EpochRecoverGenerator {
	return &EpochRecoverGenerator{
		random:       random,
		epochSetups:  epochSetups,
		epochCommits: epochCommits,
	}
}

// Fixture generates a [flow.EpochRecover] with random data based on the provided options.
func (g *EpochRecoverGenerator) Fixture(opts ...EpochRecoverOption) *flow.EpochRecover {
	counter := g.random.Uint64InRange(1, 1000)

	// Generate compatible EpochSetup and EpochCommit with the same counter
	setup := g.epochSetups.Fixture(EpochSetup.WithCounter(counter))
	commit := g.epochCommits.Fixture(
		EpochCommit.WithCounter(counter),
		EpochCommit.WithDKGFromParticipants(setup.Participants),
		EpochCommit.WithClusterQCsFromAssignments(setup.Assignments),
	)

	recover := &flow.EpochRecover{
		EpochSetup:  *setup,
		EpochCommit: *commit,
	}

	for _, opt := range opts {
		opt(g, recover)
	}

	return recover
}

// List generates a list of [flow.EpochRecover].
func (g *EpochRecoverGenerator) List(n int, opts ...EpochRecoverOption) []*flow.EpochRecover {
	recovers := make([]*flow.EpochRecover, n)
	for i := range n {
		recovers[i] = g.Fixture(opts...)
	}
	return recovers
}
