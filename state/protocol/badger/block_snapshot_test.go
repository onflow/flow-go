// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/state/protocol/util"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestHead(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		// setup
		header := unittest.BlockHeaderFixture()
		header.Height = 42

		err := db.Update(operation.InsertHeader(header.ID(), &header))
		require.NoError(t, err)

		err = db.Update(operation.IndexBlockHeight(header.Height, header.ID()))
		require.NoError(t, err)

		// add a second, outdated boundary to ensure the latest is taken
		err = db.Update(operation.InsertFinalizedHeight(header.Height - 1))
		require.NoError(t, err)

		err = db.Update(operation.UpdateFinalizedHeight(header.Height))
		require.NoError(t, err)

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
		actualClusters, err := state.Final().Clusters()
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

			_, err = state.Final().(*protocol.BlockSnapshot).Seed(1, 2, 3, 4)
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

			_, err = state.Final().(*protocol.BlockSnapshot).Seed(1, 2, 3, 4)
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
