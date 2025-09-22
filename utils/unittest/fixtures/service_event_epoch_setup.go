package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// EpochSetup is the default options factory for [flow.EpochSetup] generation.
var EpochSetup epochSetupFactory

type epochSetupFactory struct{}

type EpochSetupOption func(*EpochSetupGenerator, *flow.EpochSetup)

// WithCounter is an option that sets the `Counter` of the epoch setup.
func (f epochSetupFactory) WithCounter(counter uint64) EpochSetupOption {
	return func(g *EpochSetupGenerator, setup *flow.EpochSetup) {
		setup.Counter = counter
	}
}

// WithFirstView is an option that sets the `FirstView` of the epoch setup.
func (f epochSetupFactory) WithFirstView(view uint64) EpochSetupOption {
	return func(g *EpochSetupGenerator, setup *flow.EpochSetup) {
		setup.FirstView = view
	}
}

// WithFinalView is an option that sets the `FinalView` of the epoch setup.
func (f epochSetupFactory) WithFinalView(view uint64) EpochSetupOption {
	return func(g *EpochSetupGenerator, setup *flow.EpochSetup) {
		setup.FinalView = view
	}
}

// WithParticipants is an option that sets the `Participants` of the epoch setup.
func (f epochSetupFactory) WithParticipants(participants flow.IdentitySkeletonList) EpochSetupOption {
	return func(g *EpochSetupGenerator, setup *flow.EpochSetup) {
		setup.Participants = participants
		setup.Assignments = unittest.ClusterAssignment(1, participants)
	}
}

// WithRandomSource is an option that sets the `RandomSource` of the epoch setup.
func (f epochSetupFactory) WithRandomSource(source []byte) EpochSetupOption {
	return func(g *EpochSetupGenerator, setup *flow.EpochSetup) {
		setup.RandomSource = source
	}
}

// WithDKGFinalViews is an option that sets the `DKGPhase1FinalView`, `DKGPhase2FinalView`, and `DKGPhase3FinalView` of the epoch setup.
func (f epochSetupFactory) WithDKGFinalViews(phase1, phase2, phase3 uint64) EpochSetupOption {
	return func(g *EpochSetupGenerator, setup *flow.EpochSetup) {
		setup.DKGPhase1FinalView = phase1
		setup.DKGPhase2FinalView = phase2
		setup.DKGPhase3FinalView = phase3
	}
}

// WithTargetDuration is an option that sets the `TargetDuration` of the epoch setup.
func (f epochSetupFactory) WithTargetDuration(duration uint64) EpochSetupOption {
	return func(g *EpochSetupGenerator, setup *flow.EpochSetup) {
		setup.TargetDuration = duration
	}
}

// WithTargetEndTime is an option that sets the `TargetEndTime` of the epoch setup.
func (f epochSetupFactory) WithTargetEndTime(endTime uint64) EpochSetupOption {
	return func(g *EpochSetupGenerator, setup *flow.EpochSetup) {
		setup.TargetEndTime = endTime
	}
}

// WithAssignments is an option that sets the `Assignments` of the epoch setup.
func (f epochSetupFactory) WithAssignments(assignments flow.AssignmentList) EpochSetupOption {
	return func(g *EpochSetupGenerator, setup *flow.EpochSetup) {
		setup.Assignments = assignments
	}
}

// EpochSetupGenerator generates epoch setup events with consistent randomness.
type EpochSetupGenerator struct {
	epochSetupFactory

	random     *RandomGenerator
	timeGen    *TimeGenerator
	identities *IdentityGenerator
}

// NewEpochSetupGenerator creates a new EpochSetupGenerator.
func NewEpochSetupGenerator(random *RandomGenerator, timeGen *TimeGenerator, identities *IdentityGenerator) *EpochSetupGenerator {
	return &EpochSetupGenerator{
		random:     random,
		timeGen:    timeGen,
		identities: identities,
	}
}

// Fixture generates a [flow.EpochSetup] with random data based on the provided options.
func (g *EpochSetupGenerator) Fixture(opts ...EpochSetupOption) *flow.EpochSetup {
	baseView := uint64(g.random.Uint32())
	finalView := uint64(g.random.Uint32())
	if finalView < baseView {
		finalView = baseView
	}
	if finalView < baseView+1000 {
		finalView = baseView + 1000
	}

	participants := g.identities.List(5, g.identities.WithAllRoles())
	participants = participants.Sort(flow.Canonical[flow.Identity])

	setup := &flow.EpochSetup{
		Counter:            uint64(g.random.Uint32()),
		FirstView:          baseView,
		FinalView:          finalView,
		DKGPhase1FinalView: baseView + 100,
		DKGPhase2FinalView: baseView + 200,
		DKGPhase3FinalView: baseView + 300,
		TargetDuration:     60 * 60,
		TargetEndTime:      uint64(g.timeGen.Fixture().UnixMilli()),
		Participants:       participants.ToSkeleton(),
		RandomSource:       g.random.RandomBytes(flow.EpochSetupRandomSourceLength),
	}

	for _, opt := range opts {
		opt(g, setup)
	}

	if setup.Assignments == nil {
		setup.Assignments = unittest.ClusterAssignment(1, setup.Participants)
	}

	return setup
}

// List generates a list of [flow.EpochSetup].
func (g *EpochSetupGenerator) List(n int, opts ...EpochSetupOption) []*flow.EpochSetup {
	setups := make([]*flow.EpochSetup, n)
	for i := range n {
		setups[i] = g.Fixture(opts...)
	}
	return setups
}
