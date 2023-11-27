package protocol_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestToEpochSetup(t *testing.T) {
	expected := unittest.EpochSetupFixture()
	epoch := inmem.NewSetupEpoch(expected)

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
	epoch := inmem.NewCommittedEpoch(setup, expected)

	got, err := protocol.ToEpochCommit(epoch)
	require.NoError(t, err)
	assert.True(t, expected.EqualTo(got))
}
