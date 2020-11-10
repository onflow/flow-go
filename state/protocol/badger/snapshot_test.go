// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	protocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestHead(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		root, result, seal := unittest.BootstrapFixture(identities)
		err := state.Mutate().Bootstrap(root, result, seal)
		require.NoError(t, err)

		header := root.Header

		t.Run("works with block number", func(t *testing.T) {
			retrieved, err := state.AtHeight(header.Height).Head()
			require.NoError(t, err)
			require.Equal(t, header.ID(), retrieved.ID())
		})

		t.Run("works with block id", func(t *testing.T) {
			retrieved, err := state.AtBlockID(header.ID()).Head()
			require.NoError(t, err)
			require.Equal(t, header.ID(), retrieved.ID())
		})

		t.Run("works with finalized block", func(t *testing.T) {
			retrieved, err := state.Final().Head()
			require.NoError(t, err)
			require.Equal(t, header.ID(), retrieved.ID())
		})
	})
}

func TestIdentities(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		root, result, seal := unittest.BootstrapFixture(identities)
		err := state.Mutate().Bootstrap(root, result, seal)
		require.NoError(t, err)

		actual, err := state.Final().Identities(filter.Any)
		require.NoError(t, err)
		assert.ElementsMatch(t, identities, actual)
	})
}

func TestClusters(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		nClusters := 3
		nCollectors := 7

		collectors := unittest.IdentityListFixture(nCollectors, unittest.WithRole(flow.RoleCollection))
		identities := append(unittest.IdentityListFixture(4, unittest.WithAllRolesExcept(flow.RoleCollection)), collectors...)

		root, result, seal := unittest.BootstrapFixture(identities)
		setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := seal.ServiceEvents[1].Event.(*flow.EpochCommit)
		setup.Assignments = unittest.ClusterAssignment(uint(nClusters), collectors)
		commit.ClusterQCs = make([]*flow.QuorumCertificate, nClusters)
		for i := 0; i < nClusters; i++ {
			commit.ClusterQCs[i] = unittest.QuorumCertificateFixture()
		}
		err := state.Mutate().Bootstrap(root, result, seal)
		require.NoError(t, err)

		expectedClusters, err := flow.NewClusterList(setup.Assignments, collectors)
		require.NoError(t, err)
		actualClusters, err := state.Final().Epochs().Current().Clustering()
		require.NoError(t, err)

		require.Equal(t, nClusters, len(expectedClusters))
		require.Equal(t, len(expectedClusters), len(actualClusters))

		for i := 0; i < nClusters; i++ {
			expected := expectedClusters[i]
			actual := actualClusters[i]

			assert.Equal(t, len(expected), len(actual))
			assert.Equal(t, expected.Fingerprint(), actual.Fingerprint())
		}
	})
}

func TestSeed(t *testing.T) {

	// should not be able to get random beacon seed from a block with no children
	t.Run("no children", func(t *testing.T) {
		util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

			identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
			root, result, seal := unittest.BootstrapFixture(identities)
			err := state.Mutate().Bootstrap(root, result, seal)
			require.NoError(t, err)

			_, err = state.Final().(*protocol.Snapshot).Seed(1, 2, 3, 4)
			t.Log(err)
			assert.Error(t, err)
		})
	})

	// should not be able to get random beacon seed from a block with only invalid
	// or unvalidated children
	t.Run("un-validated child", func(t *testing.T) {
		util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

			identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
			root, result, seal := unittest.BootstrapFixture(identities)

			err := state.Mutate().Bootstrap(root, result, seal)
			require.NoError(t, err)

			// add child
			unvalidatedChild := unittest.BlockWithParentFixture(root.Header)
			unvalidatedChild.Payload.Guarantees = nil
			unvalidatedChild.Header.PayloadHash = unvalidatedChild.Payload.Hash()
			err = state.Mutate().Extend(&unvalidatedChild)
			assert.Nil(t, err)

			_, err = state.Final().(*protocol.Snapshot).Seed(1, 2, 3, 4)
			t.Log(err)
			assert.Error(t, err)
		})
	})

	// should be able to get random beacon seed from a block with a valid child
	t.Run("valid child", func(t *testing.T) {
		t.Skip()
		// TODO
	})
}

// Test querying identities in different epoch phases. During staking phase we
// should see identities from last epoch and current epoch. After staking phase
// we should see identities from current epoch and next epoch. Identities from
// a non-current epoch should have weight 0. Identities that exist in consecutive
// epochs should be de-duplicated.
func TestSnapshot_CrossEpochIdentities(t *testing.T) {

	epoch1Identities := unittest.IdentityListFixture(20, unittest.WithAllRoles())
	addedAtEpoch2 := unittest.IdentityFixture()
	removedAtEpoch2 := epoch1Identities.Sample(1)[0]
	epoch2Identities := append(
		epoch1Identities.Filter(filter.Not(filter.HasNodeID(removedAtEpoch2.NodeID))),
		addedAtEpoch2)

	// TODO
	addedAtEpoch3 := unittest.IdentityFixture()
	removedAtEpoch3 := epoch2Identities.Sample(1)[0]
	epoch3Identities := append(
		epoch2Identities.Filter(filter.Not(filter.HasNodeID(removedAtEpoch3.NodeID))),
		addedAtEpoch3)
	_ = epoch3Identities

	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {
		root, result, seal := unittest.BootstrapFixture(epoch1Identities)
		err := state.Mutate().Bootstrap(root, result, seal)
		require.Nil(t, err)

		// Prepare an epoch builder, which builds epochs with 4 blocks, A,B,C,D
		// See EpochBuilder documentation for details of these blocks.
		//
		// In these tests we refer to blocks as XN, where X is one of A,B,C,D
		// denoting which block of the epoch we're referring to and N is the
		// epoch number.
		epochBuilder := unittest.NewEpochBuilder(t, state)
		// build epoch 1
		// A - height 0 (root block)
		// B - height 1 - staking phase
		// C - height 2 - setup phase
		// D - height 3 - committed phase
		epochBuilder.
			WithSetupOpts(unittest.WithParticipants(epoch1Identities)).
			BuildEpoch().
			Complete()
		// build epoch 2
		// A - height 4
		// B - height 5 - staking phase
		// C - height 6 - setup phase
		// D - height 7 - committed phase
		epochBuilder.
			WithSetupOpts(unittest.WithParticipants(epoch2Identities)).
			BuildEpoch().
			Complete()

		t.Run("should include next epoch", func(t *testing.T) {
			C1 := state.AtHeight(2)
			identities, err := C1.Identities(filter.Any)
			require.Nil(t, err)
			// should contain all current epoch identities
			currentEpochIdentities := identities.Filter(filter.HasNodeID(epoch1Identities.NodeIDs()...))
			require.Equal(t, len(epoch1Identities), len(currentEpochIdentities))
			// all current epoch identities should match configuration from EpochSetup event
			for i := 0; i < len(epoch1Identities); i++ {
				assert.Equal(t, epoch1Identities[i], currentEpochIdentities[i])
			}

			// should contain next epoch identity with 0 weight
			nextEpochIdentity := identities.Filter(filter.HasNodeID(addedAtEpoch2.NodeID))[0]
			assert.Equal(t, uint64(0), nextEpochIdentity.Stake) // should have 0 weight
			nextEpochIdentity.Stake = addedAtEpoch2.Stake
			assert.Equal(t, addedAtEpoch2, nextEpochIdentity) // should be equal besides weight
		})
	})
}
