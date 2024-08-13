package protocol_test

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestToEpochSetup(t *testing.T) {
	expected := unittest.EpochSetupFixture()
	epoch := inmem.NewSetupEpoch(expected, nil)

	got, err := protocol.ToEpochSetup(epoch)
	require.NoError(t, err)
	assert.True(t, expected.EqualTo(got))
}

func TestToEpochCommit(t *testing.T) {
	setup := unittest.EpochSetupFixture()
	expected := unittest.EpochCommitFixture(
		unittest.CommitWithCounter(setup.Counter),
		unittest.WithDKGFromParticipants(setup.Participants),
		unittest.WithClusterQCsFromAssignments(setup.Assignments))
	epoch := inmem.NewCommittedEpoch(setup, nil, expected)

	got, err := protocol.ToEpochCommit(epoch)
	require.NoError(t, err)
	assert.True(t, expected.EqualTo(got))
}

func TestToEpochCommitEjectedParticipant(t *testing.T) {
	setup := unittest.EpochSetupFixture()
	expected := unittest.EpochCommitFixture(
		unittest.CommitWithCounter(setup.Counter),
		unittest.WithDKGFromParticipants(setup.Participants),
		unittest.WithClusterQCsFromAssignments(setup.Assignments))
	epoch := inmem.NewCommittedEpoch(setup, nil, expected)

	// eject participant from committee so DKG is bigger than committee
	ejectedNodeID := setup.Participants.Filter(filter.IsValidDKGParticipant)[0].NodeID
	setup.Participants = setup.Participants.Filter(filter.Not(filter.HasNodeID[flow.IdentitySkeleton](ejectedNodeID)))

	got, err := protocol.ToEpochCommit(epoch)
	require.NoError(t, err)
	assert.True(t, expected.EqualTo(got))
}
