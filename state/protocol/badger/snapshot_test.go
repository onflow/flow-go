// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestHead(t *testing.T) {
	identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	stateRoot := fixtureStateRootWithParticipants(t, identities)
	util.RunWithBootstrapState(t, stateRoot, func(db *badger.DB, state *bprotocol.State) {
		header := stateRoot.Block().Header

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
	identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	stateRoot := fixtureStateRootWithParticipants(t, identities)
	util.RunWithBootstrapState(t, stateRoot, func(db *badger.DB, state *bprotocol.State) {

		t.Run("no filter", func(t *testing.T) {
			actual, err := state.Final().Identities(filter.Any)
			require.Nil(t, err)
			assert.ElementsMatch(t, identities, actual)
		})

		t.Run("single identity", func(t *testing.T) {
			expected := identities.Sample(1)[0]
			actual, err := state.Final().Identity(expected.NodeID)
			require.Nil(t, err)
			assert.Equal(t, expected, actual)
		})

		t.Run("filtered", func(t *testing.T) {
			filters := []flow.IdentityFilter{
				filter.HasRole(flow.RoleCollection),
				filter.HasNodeID(identities.SamplePct(0.1).NodeIDs()...),
				filter.HasStake(true),
			}

			for _, filterfunc := range filters {
				expected := identities.Filter(filterfunc)
				actual, err := state.Final().Identities(filterfunc)
				require.Nil(t, err)
				assert.ElementsMatch(t, expected, actual)
			}
		})
	})
}

func TestClusters(t *testing.T) {
	nClusters := 3
	nCollectors := 7

	collectors := unittest.IdentityListFixture(nCollectors, unittest.WithRole(flow.RoleCollection))
	identities := append(unittest.IdentityListFixture(4, unittest.WithAllRolesExcept(flow.RoleCollection)), collectors...)

	stateRoot := fixtureStateRootWithParticipants(t, identities)
	seal := stateRoot.Seal()

	setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
	commit := seal.ServiceEvents[1].Event.(*flow.EpochCommit)
	setup.Assignments = unittest.ClusterAssignment(uint(nClusters), collectors)
	commit.ClusterQCs = make([]*flow.QuorumCertificate, nClusters)
	for i := 0; i < nClusters; i++ {
		commit.ClusterQCs[i] = unittest.QuorumCertificateFixture()
	}

	util.RunWithBootstrapState(t, stateRoot, func(db *badger.DB, state *bprotocol.State) {
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
	identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	stateRoot := fixtureStateRootWithParticipants(t, identities)

	// should not be able to get random beacon seed from a block with no children
	t.Run("no children", func(t *testing.T) {
		util.RunWithBootstrapState(t, stateRoot, func(db *badger.DB, state *bprotocol.State) {
			_, err := state.Final().(*bprotocol.Snapshot).Seed(1, 2, 3, 4)
			t.Log(err)
			assert.Error(t, err)
		})
	})

	identities = unittest.IdentityListFixture(5, unittest.WithAllRoles())
	stateRoot = fixtureStateRootWithParticipants(t, identities)

	// should not be able to get random beacon seed from a block with only invalid
	// or unvalidated children
	t.Run("un-validated child", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, stateRoot, func(db *badger.DB, state *bprotocol.FollowerState) {
			// add child
			unvalidatedChild := unittest.BlockWithParentFixture(stateRoot.Block().Header)
			unvalidatedChild.Payload.Guarantees = nil
			unvalidatedChild.Header.PayloadHash = unvalidatedChild.Payload.Hash()
			err := state.Extend(&unvalidatedChild)
			assert.Nil(t, err)

			_, err = state.Final().(*bprotocol.Snapshot).Seed(1, 2, 3, 4)
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

// test that we can query current/next/previous epochs from a snapshot
func TestSnapshot_EpochQuery(t *testing.T) {
	identities := unittest.CompleteIdentitySet()
	stateRoot := fixtureStateRootWithParticipants(t, identities)
	util.RunWithFullProtocolState(t, stateRoot, func(db *badger.DB, state *bprotocol.MutableState) {
		seal := stateRoot.Seal()
		epoch1Counter := seal.ServiceEvents[0].Event.(*flow.EpochSetup).Counter
		epoch2Counter := epoch1Counter + 1

		// Prepare an epoch builder, which builds epochs with 6 blocks, A,B,C,D,E,F
		// See EpochBuilder documentation for details of these blocks.
		//
		epochBuilder := unittest.NewEpochBuilder(t, state)
		// build blocks WITHIN epoch 1 - PREPARING epoch 2
		// A - height 0 (root block)
		// B - height 1 - staking phase
		// C - height 2 -
		// D - height 3 - setup phase
		// E - height 4 -
		// F - height 5 - committed phase
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()
		// build blocks WITHIN epoch 2 - PREPARING epoch 3
		// A - height 6 - first block of epoch 2
		// B - height 7 - staking phase
		// C - height 8 -
		// D - height 9 - setup phase
		// D - height 10 -
		// D - height 11 - committed phase
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		epoch1Heights := []uint64{0, 1, 2, 3, 4, 5}
		epoch2Heights := []uint64{6, 7, 8, 9, 10, 11}

		// we should be able to query the current epoch from any block
		t.Run("Current", func(t *testing.T) {
			t.Run("epoch 1", func(t *testing.T) {
				for _, height := range epoch1Heights {
					counter, err := state.AtHeight(height).Epochs().Current().Counter()
					require.Nil(t, err)
					assert.Equal(t, epoch1Counter, counter)
				}
			})

			t.Run("epoch 2", func(t *testing.T) {
				for _, height := range epoch2Heights {
					counter, err := state.AtHeight(height).Epochs().Current().Counter()
					require.Nil(t, err)
					assert.Equal(t, epoch2Counter, counter)
				}
			})
		})

		// we should be unable to query next epoch before it is defined by EpochSetup
		// event, afterward we should be able to query next epoch
		t.Run("Next", func(t *testing.T) {
			t.Run("epoch 1: before next epoch available", func(t *testing.T) {
				for _, height := range epoch1Heights[:3] {
					_, err := state.AtHeight(height).Epochs().Next().Counter()
					assert.Error(t, err)
					assert.True(t, errors.Is(err, protocol.ErrNextEpochNotSetup))
				}
			})

			t.Run("epoch 2: after next epoch available", func(t *testing.T) {
				for _, height := range epoch1Heights[3:] {
					counter, err := state.AtHeight(height).Epochs().Next().Counter()
					require.Nil(t, err)
					assert.Equal(t, epoch2Counter, counter)
				}
			})
		})

		// we should get a sentinel error when querying previous epoch from the
		// first epoch after the root block, otherwise we should always be able
		// to query previous epoch
		t.Run("Previous", func(t *testing.T) {
			t.Run("epoch 1", func(t *testing.T) {
				for _, height := range epoch1Heights {
					_, err := state.AtHeight(height).Epochs().Previous().Counter()
					assert.Error(t, err)
					assert.True(t, errors.Is(err, protocol.ErrNoPreviousEpoch))
				}
			})

			t.Run("epoch 2", func(t *testing.T) {
				for _, height := range epoch2Heights {
					counter, err := state.AtHeight(height).Epochs().Previous().Counter()
					require.Nil(t, err)
					assert.Equal(t, epoch1Counter, counter)
				}
			})
		})
	})
}

// test that querying the first view of an epoch returns the appropriate value
func TestSnapshot_EpochFirstView(t *testing.T) {
	identities := unittest.CompleteIdentitySet()
	stateRoot := fixtureStateRootWithParticipants(t, identities)
	util.RunWithFullProtocolState(t, stateRoot, func(db *badger.DB, state *bprotocol.MutableState) {
		root, seal := stateRoot.Block(), stateRoot.Seal()

		// Prepare an epoch builder, which builds epochs with 6 blocks, A,B,C,D,E,F
		// See EpochBuilder documentation for details of these blocks.
		epochBuilder := unittest.NewEpochBuilder(t, state)
		// build blocks WITHIN epoch 1 - PREPARING epoch 2
		// A - height 0 - (root block)
		// B - height 1 - staking phase
		// C - height 2
		// D - height 3 - setup phase
		// E - height 4
		// F - height 5 - committed phase
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()
		// build blocks WITHIN epoch 2 - PREPARING epoch 3
		// A - height 6  - first block of epoch 2
		// B - height 7  - staking phase
		// C - height 8
		// D - height 9  - setup phase
		// E - height 10
		// F - height 11 - committed phase
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// figure out the expected first views of the epochs
		epoch1FirstView := root.Header.View
		epoch2FirstView := seal.ServiceEvents[0].Event.(*flow.EpochSetup).FinalView + 1

		epoch1Heights := []uint64{0, 1, 2, 3, 4, 5}
		epoch2Heights := []uint64{6, 7, 8, 9, 10, 11}

		// check first view for snapshots within epoch 1, with respect to a
		// snapshot in either epoch 1 or epoch 2 (testing Current and Previous)
		t.Run("epoch 1", func(t *testing.T) {

			// test w.r.t. epoch 1 snapshot
			t.Run("Current", func(t *testing.T) {
				for _, height := range epoch1Heights {
					actualFirstView, err := state.AtHeight(height).Epochs().Current().FirstView()
					require.Nil(t, err)
					assert.Equal(t, epoch1FirstView, actualFirstView)
				}
			})

			// test w.r.t. epoch 2 snapshot
			t.Run("Previous", func(t *testing.T) {
				for _, height := range epoch2Heights {
					actualFirstView, err := state.AtHeight(height).Epochs().Previous().FirstView()
					require.Nil(t, err)
					assert.Equal(t, epoch1FirstView, actualFirstView)
				}
			})
		})

		// check first view for snapshots within epoch 2, with respect to a
		// snapshot in either epoch 1 or epoch 2 (testing Next and Current)
		t.Run("epoch 2", func(t *testing.T) {

			// test w.r.t. epoch 1 snapshot
			t.Run("Next", func(t *testing.T) {
				for _, height := range epoch1Heights[3:] {
					actualFirstView, err := state.AtHeight(height).Epochs().Next().FirstView()
					require.Nil(t, err)
					assert.Equal(t, epoch2FirstView, actualFirstView)
				}
			})

			// test w.r.t. epoch 2 snapshot
			t.Run("Current", func(t *testing.T) {
				for _, height := range epoch2Heights {
					actualFirstView, err := state.AtHeight(height).Epochs().Current().FirstView()
					require.Nil(t, err)
					assert.Equal(t, epoch2FirstView, actualFirstView)
				}
			})
		})
	})
}

// Test querying identities in different epoch phases. During staking phase we
// should see identities from last epoch and current epoch. After staking phase
// we should see identities from current epoch and next epoch. Identities from
// a non-current epoch should have weight 0. Identities that exist in consecutive
// epochs should be de-duplicated.
func TestSnapshot_CrossEpochIdentities(t *testing.T) {

	// start with 20 identities in epoch 1
	epoch1Identities := unittest.IdentityListFixture(20, unittest.WithAllRoles())
	// 1 identity added at epoch 2 that was not present in epoch 1
	addedAtEpoch2 := unittest.IdentityFixture()
	// 1 identity removed in epoch 2 that was present in epoch 1
	removedAtEpoch2 := epoch1Identities.Sample(1)[0]
	// epoch 2 has partial overlap with epoch 1
	epoch2Identities := append(
		epoch1Identities.Filter(filter.Not(filter.HasNodeID(removedAtEpoch2.NodeID))),
		addedAtEpoch2)
	// epoch 3 has no overlap with epoch 2
	epoch3Identities := unittest.IdentityListFixture(10, unittest.WithAllRoles())

	stateRoot := fixtureStateRootWithParticipants(t, epoch1Identities)
	util.RunWithFullProtocolState(t, stateRoot, func(db *badger.DB, state *bprotocol.MutableState) {

		// Prepare an epoch builder, which builds epochs with 6 blocks, A,B,C,D,E,F
		// See EpochBuilder documentation for details of these blocks.
		epochBuilder := unittest.NewEpochBuilder(t, state)
		// build blocks WITHIN epoch 1 - PREPARING epoch 2
		// A - height 0 - (root block)
		// B - height 1 - staking phase
		// C - height 2
		// D - height 3 - setup phase
		// E - height 4
		// F - height 5 - committed phase
		epochBuilder.
			UsingSetupOpts(unittest.WithParticipants(epoch2Identities)).
			BuildEpoch().
			CompleteEpoch()
		// build blocks WITHIN epoch 2 - PREPARING epoch 3
		// A - height 6  - first block of epoch 2
		// B - height 7  - staking phase
		// C - height 8
		// D - height 9  - setup phase
		// E - height 10
		// F - height 11 - committed phase
		epochBuilder.
			UsingSetupOpts(unittest.WithParticipants(epoch3Identities)).
			BuildEpoch().
			CompleteEpoch()

		t.Run("should be able to query at root block", func(t *testing.T) {
			snapshot := state.AtHeight(0)
			identities, err := snapshot.Identities(filter.Any)
			require.Nil(t, err)

			// should have the right number of identities
			assert.Equal(t, len(epoch1Identities), len(identities))
			// should have all epoch 1 identities
			assert.ElementsMatch(t, epoch1Identities, identities)
		})

		t.Run("should include next epoch after staking phase", func(t *testing.T) {

			// get a snapshot from setup phase and commit phase of epoch 1
			snapshots := []protocol.Snapshot{state.AtHeight(3), state.AtHeight(5)}

			for _, snapshot := range snapshots {
				phase, err := snapshot.Phase()
				require.Nil(t, err)

				t.Run("phase: "+phase.String(), func(t *testing.T) {
					identities, err := snapshot.Identities(filter.Any)
					require.Nil(t, err)

					// should have the right number of identities
					assert.Equal(t, len(epoch1Identities)+1, len(identities))
					// all current epoch identities should match configuration from EpochSetup event
					assert.ElementsMatch(t, epoch1Identities, identities.Filter(epoch1Identities.Selector()))

					// should contain single next epoch identity with 0 weight
					nextEpochIdentity := identities.Filter(filter.HasNodeID(addedAtEpoch2.NodeID))[0]
					assert.Equal(t, uint64(0), nextEpochIdentity.Stake) // should have 0 weight
					nextEpochIdentity.Stake = addedAtEpoch2.Stake
					assert.Equal(t, addedAtEpoch2, nextEpochIdentity) // should be equal besides weight
				})
			}
		})

		t.Run("should include previous epoch in staking phase", func(t *testing.T) {

			// get a snapshot from staking phase of epoch 2
			snapshot := state.AtHeight(7)
			identities, err := snapshot.Identities(filter.Any)
			require.Nil(t, err)

			// should have the right number of identities
			assert.Equal(t, len(epoch2Identities)+1, len(identities))
			// all current epoch identities should match configuration from EpochSetup event
			assert.ElementsMatch(t, epoch2Identities, identities.Filter(epoch2Identities.Selector()))

			// should contain single previous epoch identity with 0 weight
			lastEpochIdentity := identities.Filter(filter.HasNodeID(removedAtEpoch2.NodeID))[0]
			assert.Equal(t, uint64(0), lastEpochIdentity.Stake) // should have 0 weight
			lastEpochIdentity.Stake = removedAtEpoch2.Stake     // overwrite weight
			assert.Equal(t, removedAtEpoch2, lastEpochIdentity) // should be equal besides weight
		})

		t.Run("should not include previous epoch after staking phase", func(t *testing.T) {

			// get a snapshot from setup phase and commit phase of epoch 2
			snapshots := []protocol.Snapshot{state.AtHeight(9), state.AtHeight(11)}

			for _, snapshot := range snapshots {
				phase, err := snapshot.Phase()
				require.Nil(t, err)

				t.Run("phase: "+phase.String(), func(t *testing.T) {
					identities, err := snapshot.Identities(filter.Any)
					require.Nil(t, err)

					// should have the right number of identities
					assert.Equal(t, len(epoch2Identities)+len(epoch3Identities), len(identities))
					// all current epoch identities should match configuration from EpochSetup event
					assert.ElementsMatch(t, epoch2Identities, identities.Filter(epoch2Identities.Selector()))

					// should contain next epoch identities with 0 weight
					for _, expected := range epoch3Identities {
						actual, exists := identities.ByNodeID(expected.NodeID)
						require.True(t, exists)
						assert.Equal(t, uint64(0), actual.Stake) // should have 0 weight
						actual.Stake = expected.Stake            // overwrite weight
						assert.Equal(t, expected, actual)        // should be equal besides weight
					}
				})
			}
		})
	})
}

// test that we can retrieve identities after a spork where the parent ID of the
// root block is non-nil
func TestSnapshot_PostSporkIdentities(t *testing.T) {
	expected := unittest.CompleteIdentitySet()
	root, result, seal := unittest.BootstrapFixture(expected, func(block *flow.Block) {
		block.Header.ParentID = unittest.IdentifierFixture()
	})

	stateRoot, err := bprotocol.NewStateRoot(root, result, seal, 0)
	require.NoError(t, err)

	util.RunWithBootstrapState(t, stateRoot, func(db *badger.DB, state *bprotocol.State) {
		actual, err := state.Final().Identities(filter.Any)
		require.Nil(t, err)
		assert.ElementsMatch(t, expected, actual)
	})
}
