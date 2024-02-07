package protocol_test

import (
	"testing"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

var participants = unittest.IdentityListFixture(20, unittest.WithAllRoles())

func TestEpochSetupValidity(t *testing.T) {
	t.Run("invalid first/final view", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		// set an invalid final view for the first epoch
		setup.FinalView = setup.FirstView

		err := protocol.IsValidEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("non-canonically ordered identities", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		// randomly shuffle the identities so they are not canonically ordered
		var err error
		setup.Participants, err = setup.Participants.Shuffle()
		require.NoError(t, err)
		err = protocol.IsValidEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("invalid cluster assignments", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		// create an invalid cluster assignment (node appears in multiple clusters)
		collector := participants.Filter(filter.HasRole[flow.Identity](flow.RoleCollection))[0]
		setup.Assignments = append(setup.Assignments, []flow.Identifier{collector.NodeID})

		err := protocol.IsValidEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("short seed", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		setup.RandomSource = unittest.SeedFixture(crypto.KeyGenSeedMinLen - 1)

		err := protocol.IsValidEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("node role missing", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		allWithoutExecutionNodes := setup.Participants.Filter(func(identitySkeleton *flow.IdentitySkeleton) bool {
			return identitySkeleton.Role != flow.RoleExecution
		})
		setup.Participants = allWithoutExecutionNodes

		err := protocol.IsValidEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("network addresses are not unique", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		setup.Participants[0].Address = setup.Participants[1].Address

		err := protocol.IsValidEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("no cluster assignment", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		setup.Assignments = flow.AssignmentList{}

		err := protocol.IsValidEpochSetup(setup, true)
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

		err := protocol.IsValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("inconsistent cluster QCs", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		// add an extra QC to commit
		extraQC := unittest.QuorumCertificateWithSignerIDsFixture()
		commit.ClusterQCs = append(commit.ClusterQCs, flow.ClusterQCVoteDataFromQC(extraQC))

		err := protocol.IsValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("missing dkg group key", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		commit.DKGGroupKey = nil

		err := protocol.IsValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("inconsistent DKG participants", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		// add an extra DKG participant key
		commit.DKGParticipantKeys = append(commit.DKGParticipantKeys, unittest.KeyFixture(crypto.BLSBLS12381).PublicKey())

		err := protocol.IsValidEpochCommit(commit, setup)
		require.Error(t, err)
	})
}

// TestIsValidExtendingEpochSetup tests that implementation enforces the following protocol rules in case they are violated:
// (a) We should only have a single epoch setup event per epoch.
// (b) The setup event should have the counter increased by one
// (c) The first view needs to be exactly one greater than the current epoch final view
// additionally we require other conditions, but they are tested by separate test `TestEpochSetupValidity`.
func TestIsValidExtendingEpochSetup(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		protocolState := unittest.ProtocolStateFixture()
		currentEpochSetup := protocolState.CurrentEpochSetup
		extendingSetup := unittest.EpochSetupFixture(
			unittest.WithFirstView(currentEpochSetup.FinalView+1),
			unittest.WithFinalView(currentEpochSetup.FinalView+1000),
			unittest.SetupWithCounter(currentEpochSetup.Counter+1),
			unittest.WithParticipants(participants.ToSkeleton()),
		)
		err := protocol.IsValidExtendingEpochSetup(extendingSetup, protocolState.ProtocolStateEntry, currentEpochSetup)
		require.NoError(t, err)
	})
	t.Run("(a) We should only have a single epoch setup event per epoch.", func(t *testing.T) {
		protocolState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())
		currentEpochSetup := protocolState.CurrentEpochSetup
		extendingSetup := unittest.EpochSetupFixture(
			unittest.WithFirstView(currentEpochSetup.FinalView+1),
			unittest.WithFinalView(currentEpochSetup.FinalView+1000),
			unittest.SetupWithCounter(currentEpochSetup.Counter+1),
			unittest.WithParticipants(participants.ToSkeleton()),
		)
		err := protocol.IsValidExtendingEpochSetup(extendingSetup, protocolState.ProtocolStateEntry, currentEpochSetup)
		require.Error(t, err)
	})
	t.Run("(b) The setup event should have the counter increased by one", func(t *testing.T) {
		protocolState := unittest.ProtocolStateFixture()
		currentEpochSetup := protocolState.CurrentEpochSetup
		extendingSetup := unittest.EpochSetupFixture(
			unittest.WithFirstView(currentEpochSetup.FinalView+1),
			unittest.WithFinalView(currentEpochSetup.FinalView+1000),
			unittest.SetupWithCounter(currentEpochSetup.Counter+2),
			unittest.WithParticipants(participants.ToSkeleton()),
		)
		err := protocol.IsValidExtendingEpochSetup(extendingSetup, protocolState.ProtocolStateEntry, currentEpochSetup)
		require.Error(t, err)
	})
	t.Run("(c) The first view needs to be exactly one greater than the current epoch final view", func(t *testing.T) {
		protocolState := unittest.ProtocolStateFixture()
		currentEpochSetup := protocolState.CurrentEpochSetup
		extendingSetup := unittest.EpochSetupFixture(
			unittest.WithFirstView(currentEpochSetup.FinalView+2),
			unittest.WithFinalView(currentEpochSetup.FinalView+1000),
			unittest.SetupWithCounter(currentEpochSetup.Counter+1),
			unittest.WithParticipants(participants.ToSkeleton()),
		)
		err := protocol.IsValidExtendingEpochSetup(extendingSetup, protocolState.ProtocolStateEntry, currentEpochSetup)
		require.Error(t, err)
	})
}

// TestIsValidExtendingEpochCommit tests that implementation enforces the following protocol rules in case they are violated:
// (a) The epoch setup event needs to happen before the commit.
// (b) We should only have a single epoch commit event per epoch.
// additionally we require other conditions, but they are tested by separate test `TestEpochCommitValidity`.
func TestIsValidExtendingEpochCommit(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		protocolState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState(), func(entry *flow.RichProtocolStateEntry) {
			entry.NextEpochCommit = nil
			entry.NextEpoch.CommitID = flow.ZeroID
		})

		nextEpochSetup := protocolState.NextEpochSetup
		extendingSetup := unittest.EpochCommitFixture(
			unittest.CommitWithCounter(nextEpochSetup.Counter),
			unittest.WithDKGFromParticipants(nextEpochSetup.Participants),
		)
		err := protocol.IsValidExtendingEpochCommit(extendingSetup, protocolState.ProtocolStateEntry, nextEpochSetup)
		require.NoError(t, err)
	})
	t.Run("(a) The epoch setup event needs to happen before the commit", func(t *testing.T) {
		protocolState := unittest.ProtocolStateFixture()
		currentEpochSetup := protocolState.CurrentEpochSetup
		nextEpochSetup := unittest.EpochSetupFixture(
			unittest.WithFirstView(currentEpochSetup.FinalView+1),
			unittest.WithFinalView(currentEpochSetup.FinalView+1000),
			unittest.SetupWithCounter(currentEpochSetup.Counter+1),
			unittest.WithParticipants(participants.ToSkeleton()),
		)
		extendingSetup := unittest.EpochCommitFixture(
			unittest.CommitWithCounter(nextEpochSetup.Counter),
			unittest.WithDKGFromParticipants(nextEpochSetup.Participants),
		)
		err := protocol.IsValidExtendingEpochCommit(extendingSetup, protocolState.ProtocolStateEntry, nextEpochSetup)
		require.Error(t, err)
	})
	t.Run("We should only have a single epoch commit event per epoch", func(t *testing.T) {
		protocolState := unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState())

		nextEpochSetup := protocolState.NextEpochSetup
		extendingSetup := unittest.EpochCommitFixture(
			unittest.CommitWithCounter(nextEpochSetup.Counter),
			unittest.WithDKGFromParticipants(nextEpochSetup.Participants),
		)
		err := protocol.IsValidExtendingEpochCommit(extendingSetup, protocolState.ProtocolStateEntry, nextEpochSetup)
		require.Error(t, err)
	})
}
