package unittest

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

// EpochBuilder is a testing utility for building epochs into chain state.
type EpochBuilder struct {
	t          *testing.T
	state      protocol.State
	setupOpts  []func(*flow.EpochSetup)  // options to apply to the EpochSetup event
	commitOpts []func(*flow.EpochCommit) // options to apply to the EpochCommit event
}

func NewEpochBuilder(t *testing.T, state protocol.State) *EpochBuilder {

	builder := &EpochBuilder{
		t:     t,
		state: state,
	}
	return builder
}

// UsingSetupOpts sets options for the epoch setup event. For options
// targeting the same field, those added here will take precedence
// over defaults.
func (builder *EpochBuilder) UsingSetupOpts(opts ...func(*flow.EpochSetup)) *EpochBuilder {
	builder.setupOpts = opts
	return builder
}

// UsingCommitOpts sets options for the epoch setup event. For options
// targeting the same field, those added here will take precedence
// over defaults.
func (builder *EpochBuilder) UsingCommitOpts(opts ...func(*flow.EpochCommit)) *EpochBuilder {
	builder.commitOpts = opts
	return builder
}

// Build builds and finalizes a sequence of blocks comprising a minimal full
// epoch (epoch N). We assume the latest finalized block is within staking phase
// in epoch N.
//
// |   EPOCH N    |
// A -> B -> C -> D
//
// A is the latest finalized block. B contains seals up to block A, if needed. It
// contains no service events or seals. C contains a seal for block B containing
// the EpochSetup service event. Block D contains a seal for block C containing
// the EpochCommit service event.
//
// To build a sequence of epochs, we call BuildEpoch, then CompleteEpoch, and so on.
//
func (builder *EpochBuilder) BuildEpoch() *EpochBuilder {

	// prepare default values for the service events based on the current state
	identities, err := builder.state.Final().Identities(filter.Any)
	require.Nil(builder.t, err)
	epoch := builder.state.Final().Epochs().Current()
	counter, err := epoch.Counter()
	require.Nil(builder.t, err)
	finalView, err := epoch.FinalView()
	require.Nil(builder.t, err)

	// retrieve block A
	A, err := builder.state.Final().Head()
	require.Nil(builder.t, err)

	// check that block A satisfies initial condition
	phase, err := builder.state.Final().Phase()
	require.Nil(builder.t, err)
	require.Equal(builder.t, flow.EpochPhaseStaking, phase)

	// retrieve the sealed height to determine what seals to include in B
	sealed, err := builder.state.Sealed().Head()
	require.Nil(builder.t, err)

	var seals []*flow.Seal
	for height := sealed.Height + 1; height <= A.Height; height++ {
		next, err := builder.state.AtHeight(height).Head()
		require.Nil(builder.t, err)
		seals = append(seals, SealFixture(SealWithBlockID(next.ID())))
	}

	// build block B, sealing up to and including block A
	B := BlockWithParentFixture(A)
	B.SetPayload(flow.Payload{Seals: seals})
	err = builder.state.Mutate().Extend(&B)
	require.Nil(builder.t, err)
	// finalize and validate block B
	err = builder.state.Mutate().Finalize(B.ID())
	require.Nil(builder.t, err)
	err = builder.state.Mutate().MarkValid(B.ID())
	require.Nil(builder.t, err)

	// defaults for the EpochSetup event
	setupDefaults := []func(*flow.EpochSetup){
		WithParticipants(identities),
		SetupWithCounter(counter + 1),
		WithFinalView(finalView + 1000),
	}

	// build block C
	// C contains a seal for block B and the EpochSetup event
	setup := EpochSetupFixture(append(setupDefaults, builder.setupOpts...)...)
	C := BlockWithParentFixture(B.Header)
	sealForB := SealFixture(
		SealWithBlockID(B.ID()),
		WithServiceEvents(setup.ServiceEvent()),
	)
	C.SetPayload(flow.Payload{
		Seals: []*flow.Seal{sealForB},
	})
	err = builder.state.Mutate().Extend(&C)
	require.Nil(builder.t, err)
	// finalize and validate block C
	err = builder.state.Mutate().Finalize(C.ID())
	require.Nil(builder.t, err)
	err = builder.state.Mutate().MarkValid(C.ID())
	require.Nil(builder.t, err)

	// defaults for the EpochCommit event
	commitDefaults := []func(*flow.EpochCommit){
		CommitWithCounter(counter + 1),
		WithDKGFromParticipants(setup.Participants),
	}

	// build block D
	// D contains a seal for block C and the EpochCommit event
	commit := EpochCommitFixture(append(commitDefaults, builder.commitOpts...)...)
	D := BlockWithParentFixture(C.Header)
	sealForC := SealFixture(
		SealWithBlockID(C.ID()),
		WithServiceEvents(commit.ServiceEvent()),
	)
	D.SetPayload(flow.Payload{
		Seals: []*flow.Seal{sealForC},
	})
	err = builder.state.Mutate().Extend(&D)
	require.Nil(builder.t, err)
	// finalize and validate block D
	err = builder.state.Mutate().Finalize(D.ID())
	require.Nil(builder.t, err)
	err = builder.state.Mutate().MarkValid(D.ID())
	require.Nil(builder.t, err)

	return builder
}

// CompleteEpoch caps off the current epoch by building the first block of the next
// epoch. We must be in the Committed phase to call CompleteEpoch. Once the epoch
// has been capped off, we can build the next epoch with BuildEpoch.
func (builder *EpochBuilder) CompleteEpoch() {

	phase, err := builder.state.Final().Phase()
	require.Nil(builder.t, err)
	require.Equal(builder.t, flow.EpochPhaseCommitted, phase)
	finalView, err := builder.state.Final().Epochs().Current().FinalView()
	require.Nil(builder.t, err)

	final, err := builder.state.Final().Head()
	require.Nil(builder.t, err)

	// A is the first block of the next epoch (see diagram in BuildEpoch)
	A := BlockWithParentFixture(final)
	// first view is not necessarily exactly final view of previous epoch
	A.Header.View = finalView + (rand.Uint64() % 4) + 1
	A.SetPayload(flow.EmptyPayload())
	err = builder.state.Mutate().Extend(&A)
	require.Nil(builder.t, err)
	err = builder.state.Mutate().Finalize(A.ID())
	require.Nil(builder.t, err)
	err = builder.state.Mutate().MarkValid(A.ID())
	require.Nil(builder.t, err)
}
