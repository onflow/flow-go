package protocol_test

import (
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
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
	expected := unittest.EpochCommitFixture(unittest.CommitWithCounter(setup.Counter))
	epoch := inmem.NewCommittedEpoch(setup, expected)

	got, err := protocol.ToEpochCommit(epoch)
	require.NoError(t, err)
	assert.True(t, expected.EqualTo(got))
}
