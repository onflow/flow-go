package badger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"
)

var participants = unittest.IdentityListFixture(20, unittest.WithAllRoles())

func TestEpochSetupValidity(t *testing.T) {
	t.Run("invalid first/final view", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		// set an invalid final view for the first epoch
		setup.FinalView = setup.FirstView

		err := verifyEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("non-canonically ordered identities", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		// randomly shuffle the identities so they are not canonically ordered
		setup.Participants = setup.Participants.DeterministicShuffle(time.Now().UnixNano())

		err := verifyEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("invalid cluster assignments", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		// create an invalid cluster assignment (node appears in multiple clusters)
		collector := participants.Filter(filter.HasRole(flow.RoleCollection))[0]
		setup.Assignments = append(setup.Assignments, []flow.Identifier{collector.NodeID})

		err := verifyEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("short seed", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		setup.RandomSource = unittest.SeedFixture(crypto.SeedMinLenDKG - 1)

		err := verifyEpochSetup(setup, true)
		require.Error(t, err)
	})
}

func TestBootstrapInvalidEpochCommit(t *testing.T) {
	t.Run("inconsistent counter", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		// use a different counter for the commit
		commit.Counter = setup.Counter + 1

		err := isValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("inconsistent cluster QCs", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		// add an extra QC to commit
		extraQC := unittest.QuorumCertificateWithSignerIDsFixture()
		commit.ClusterQCs = append(commit.ClusterQCs, flow.ClusterQCVoteDataFromQC(extraQC))

		err := isValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("missing dkg group key", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		commit.DKGGroupKey = nil

		err := isValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("inconsistent DKG participants", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		// add an extra DKG participant key
		commit.DKGParticipantKeys = append(commit.DKGParticipantKeys, unittest.KeyFixture(crypto.BLSBLS12381).PublicKey())

		err := isValidEpochCommit(commit, setup)
		require.Error(t, err)
	})
}

// TestEntityExpirySnapshotValidation tests that we perform correct sanity checks when
// bootstrapping consensus nodes and access nodes we expect that we only bootstrap snapshots
// with sufficient history.
func TestEntityExpirySnapshotValidation(t *testing.T) {
	t.Run("spork-root-snapshot", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		err := ValidRootSnapshotContainsEntityExpiryRange(rootSnapshot)
		require.NoError(t, err)
	})
	t.Run("not-enough-history", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		rootSnapshot.Encodable().Head.Height += 10 // advance height to be not spork root snapshot
		err := ValidRootSnapshotContainsEntityExpiryRange(rootSnapshot)
		require.Error(t, err)
	})
	t.Run("enough-history-spork-just-started", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		// advance height to be not spork root snapshot, but still lower than transaction expiry
		rootSnapshot.Encodable().Head.Height += flow.DefaultTransactionExpiry / 2
		// add blocks to sealing segment
		rootSnapshot.Encodable().SealingSegment.ExtraBlocks = unittest.BlockFixtures(int(flow.DefaultTransactionExpiry / 2))
		err := ValidRootSnapshotContainsEntityExpiryRange(rootSnapshot)
		require.NoError(t, err)
	})
	t.Run("enough-history-long-spork", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		// advance height to be not spork root snapshot
		rootSnapshot.Encodable().Head.Height += flow.DefaultTransactionExpiry * 2
		// add blocks to sealing segment
		rootSnapshot.Encodable().SealingSegment.ExtraBlocks = unittest.BlockFixtures(int(flow.DefaultTransactionExpiry) - 1)
		err := ValidRootSnapshotContainsEntityExpiryRange(rootSnapshot)
		require.NoError(t, err)
	})
	t.Run("more-history-than-needed", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		// advance height to be not spork root snapshot
		rootSnapshot.Encodable().Head.Height += flow.DefaultTransactionExpiry * 2
		// add blocks to sealing segment
		rootSnapshot.Encodable().SealingSegment.ExtraBlocks = unittest.BlockFixtures(flow.DefaultTransactionExpiry * 2)
		err := ValidRootSnapshotContainsEntityExpiryRange(rootSnapshot)
		require.NoError(t, err)
	})
}
