package badger

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

var participants = unittest.IdentityListFixture(5, unittest.WithAllRoles())

// bootstraps protocol state with the given snapshot and invokes the callback
// with the result of the constructor
func bootstrap(t *testing.T, rootSnapshot protocol.Snapshot, f func(*State, error)) {
	metrics := metrics.NewNoopCollector()
	dir := unittest.TempDir(t)
	defer os.RemoveAll(dir)
	db := unittest.BadgerDB(t, dir)
	defer db.Close()
	headers, _, seals, _, _, blocks, setups, commits, statuses, results := util.StorageLayer(t, db)
	state, err := Bootstrap(metrics, db, headers, seals, results, blocks, setups, commits, statuses, rootSnapshot)
	f(state, err)
}

func TestBootstrapDuplicateID(t *testing.T) {
	participants := flow.IdentityList{
		{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
		{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
		{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
		{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4}, // dupe
	}
	root := unittest.RootSnapshotFixture(participants)
	bootstrap(t, root, func(state *State, err error) {
		assert.Error(t, err)
	})
}

func TestBootstrapZeroStake(t *testing.T) {
	participants := flow.IdentityList{
		{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 0},
		{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
		{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
		{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
	}
	root := unittest.RootSnapshotFixture(participants)
	bootstrap(t, root, func(state *State, err error) {
		assert.Error(t, err)
	})
}

func TestBootstrapMissingRole(t *testing.T) {
	requiredRoles := []flow.Role{
		flow.RoleConsensus,
		flow.RoleCollection,
		flow.RoleExecution,
		flow.RoleVerification,
	}

	for _, role := range requiredRoles {
		t.Run(fmt.Sprintf("no %s nodes", role.String()), func(t *testing.T) {
			participants := unittest.IdentityListFixture(5, unittest.WithAllRolesExcept(role))
			root := unittest.RootSnapshotFixture(participants)
			bootstrap(t, root, func(state *State, err error) {
				assert.Error(t, err)
			})
		})
	}
}

func TestBootstrapExistingAddress(t *testing.T) {
	participants := flow.IdentityList{
		{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
		{NodeID: flow.Identifier{0x02}, Address: "a1", Role: flow.RoleConsensus, Stake: 2}, // dupe address
		{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
		{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
	}

	root := unittest.RootSnapshotFixture(participants)
	bootstrap(t, root, func(state *State, err error) {
		assert.Error(t, err)
	})
}

// TODO re-work following tests - need to be different now that we can bootstrap
// from any snapshot
func TestBootstrapWithSeal(t *testing.T) {
	block := unittest.GenesisFixture(participants)
	block.Payload.Seals = []*flow.Seal{unittest.Seal.Fixture()}
	block.Header.PayloadHash = block.Payload.Hash()

	result := unittest.ExecutionResultFixture()
	result.BlockID = block.ID()

	finalState, ok := result.FinalStateCommitment()
	require.True(t, ok)

	seal := unittest.Seal.Fixture()
	seal.BlockID = block.ID()
	seal.ResultID = result.ID()
	seal.FinalState = finalState

	_, err := flow.NewStateRoot(block, result, seal, 0)
	require.Error(t, err)
}

func TestBootstrapMissingServiceEvents(t *testing.T) {
	t.Run("missing setup", func(t *testing.T) {
		root, result, seal := unittest.BootstrapFixture(participants)
		seal.ServiceEvents = seal.ServiceEvents[1:]
		_, err := flow.NewStateRoot(root, result, seal, 0)
		require.Error(t, err)
	})

	t.Run("missing commit", func(t *testing.T) {
		root, result, seal := unittest.BootstrapFixture(participants)
		seal.ServiceEvents = seal.ServiceEvents[:1]
		_, err := flow.NewStateRoot(root, result, seal, 0)
		require.Error(t, err)
	})
}

func TestBootstrapInvalidEpochSetup(t *testing.T) {
	t.Run("invalid final view", func(t *testing.T) {
		root, result, seal := unittest.BootstrapFixture(participants)
		setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
		// set an invalid final view for the first epoch
		setup.FinalView = root.Header.View

		_, err := flow.NewStateRoot(root, result, seal, 0)
		require.Error(t, err)
	})

	t.Run("invalid cluster assignments", func(t *testing.T) {
		root, result, seal := unittest.BootstrapFixture(participants)
		setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
		// create an invalid cluster assignment (node appears in multiple clusters)
		collector := participants.Filter(filter.HasRole(flow.RoleCollection))[0]
		setup.Assignments = append(setup.Assignments, []flow.Identifier{collector.NodeID})

		_, err := flow.NewStateRoot(root, result, seal, 0)
		require.Error(t, err)
	})

	t.Run("empty seed", func(t *testing.T) {
		root, result, seal := unittest.BootstrapFixture(participants)
		setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
		setup.RandomSource = nil

		_, err := flow.NewStateRoot(root, result, seal, 0)
		require.Error(t, err)
	})
}

func TestBootstrapInvalidEpochCommit(t *testing.T) {
	t.Run("inconsistent counter", func(t *testing.T) {
		root, result, seal := unittest.BootstrapFixture(participants)
		setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := seal.ServiceEvents[1].Event.(*flow.EpochCommit)
		// use a different counter for the commit
		commit.Counter = setup.Counter + 1

		_, err := flow.NewStateRoot(root, result, seal, 0)
		require.Error(t, err)
	})

	t.Run("inconsistent cluster QCs", func(t *testing.T) {
		root, result, seal := unittest.BootstrapFixture(participants)
		commit := seal.ServiceEvents[1].Event.(*flow.EpochCommit)
		// add an extra QC to commit
		commit.ClusterQCs = append(commit.ClusterQCs, unittest.QuorumCertificateFixture())

		_, err := flow.NewStateRoot(root, result, seal, 0)
		require.Error(t, err)
	})

	t.Run("missing dkg group key", func(t *testing.T) {
		root, result, seal := unittest.BootstrapFixture(participants)
		commit := seal.ServiceEvents[1].Event.(*flow.EpochCommit)
		commit.DKGGroupKey = nil

		_, err := flow.NewStateRoot(root, result, seal, 0)
		require.Error(t, err)
	})

	t.Run("inconsistent DKG participants", func(t *testing.T) {
		root, result, seal := unittest.BootstrapFixture(participants)
		commit := seal.ServiceEvents[1].Event.(*flow.EpochCommit)
		// add an invalid DKG participant
		collector := participants.Filter(filter.HasRole(flow.RoleCollection))[0]
		commit.DKGParticipants[collector.NodeID] = flow.DKGParticipant{
			KeyShare: unittest.KeyFixture(crypto.BLSBLS12381).PublicKey(),
			Index:    1,
		}

		_, err := flow.NewStateRoot(root, result, seal, 0)
		require.Error(t, err)
	})
}
