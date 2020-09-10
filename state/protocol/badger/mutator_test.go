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

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/state/protocol/util"
	stoerr "github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var participants = unittest.IdentityListFixture(5, unittest.WithAllRoles())

func TestBootstrapValid(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block, result, seal := unittest.BootstrapFixture(participants)
		err := state.Mutate().Bootstrap(block, result, seal)
		require.NoError(t, err)

		var finalized uint64
		err = db.View(operation.RetrieveFinalizedHeight(&finalized))
		require.NoError(t, err)

		var sealed uint64
		err = db.View(operation.RetrieveSealedHeight(&sealed))
		require.NoError(t, err)

		var genesisID flow.Identifier
		err = db.View(operation.LookupBlockHeight(0, &genesisID))
		require.NoError(t, err)

		var header flow.Header
		err = db.View(operation.RetrieveHeader(genesisID, &header))
		require.NoError(t, err)

		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(genesisID, &sealID))
		require.NoError(t, err)

		err = db.View(operation.RetrieveSeal(sealID, seal))
		require.NoError(t, err)

		assert.Equal(t, block.Header.Height, finalized)
		assert.Equal(t, block.Header.Height, sealed)
		assert.Equal(t, block.ID(), genesisID)
		assert.Equal(t, block.ID(), seal.BlockID)
		assert.Equal(t, block.Header, &header)
	})
}

func TestBootstrapDuplicateID(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		block, result, seal := unittest.BootstrapFixture(participants)

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapZeroStake(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 0},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		block, result, seal := unittest.BootstrapFixture(participants)

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapNoCollection(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		block, result, seal := unittest.BootstrapFixture(participants)

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapNoConsensus(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		block, result, seal := unittest.BootstrapFixture(participants)

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapNoExecution(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		block, result, seal := unittest.BootstrapFixture(participants)

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapNoVerification(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
		}

		block, result, seal := unittest.BootstrapFixture(participants)

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapExistingAddress(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a1", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		block, result, seal := unittest.BootstrapFixture(participants)

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapNonZeroParent(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block, result, seal := unittest.BootstrapFixture(participants, func(block *flow.Block) {
			block.Header.Height = 13
			block.Header.ParentID = unittest.IdentifierFixture()
		})

		err := state.Mutate().Bootstrap(block, result, seal)
		require.NoError(t, err)
	})
}

func TestBootstrapNonEmptyCollections(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block, result, seal := unittest.BootstrapFixture(participants, func(block *flow.Block) {
			block.Payload.Guarantees = unittest.CollectionGuaranteesFixture(1)
		})

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapWithSeal(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block := unittest.GenesisFixture(participants)
		block.Payload.Seals = []*flow.Seal{unittest.SealFixture()}
		block.Header.PayloadHash = block.Payload.Hash()

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.SealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapMissingServiceEvents(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		t.Run("missing setup", func(t *testing.T) {
			root, result, seal := unittest.BootstrapFixture(participants)
			seal.ServiceEvents = seal.ServiceEvents[1:]
			err := state.Mutate().Bootstrap(root, result, seal)
			require.Error(t, err)
		})

		t.Run("missing commit", func(t *testing.T) {
			root, result, seal := unittest.BootstrapFixture(participants)
			seal.ServiceEvents = seal.ServiceEvents[:1]
			err := state.Mutate().Bootstrap(root, result, seal)
			require.Error(t, err)
		})
	})
}

func TestBootstrapInvalidEpochSetup(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		t.Run("invalid final view", func(t *testing.T) {
			root, result, seal := unittest.BootstrapFixture(participants)
			setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
			// set an invalid final view for the first epoch
			setup.FinalView = root.Header.View

			err := state.Mutate().Bootstrap(root, result, seal)
			require.Error(t, err)
		})

		t.Run("invalid cluster assignments", func(t *testing.T) {
			root, result, seal := unittest.BootstrapFixture(participants)
			setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
			// create an invalid cluster assignment (node appears in multiple clusters)
			collector := participants.Filter(filter.HasRole(flow.RoleCollection))[0]
			setup.Assignments = append(setup.Assignments, []flow.Identifier{collector.NodeID})

			err := state.Mutate().Bootstrap(root, result, seal)
			require.Error(t, err)
		})

		t.Run("empty seed", func(t *testing.T) {
			root, result, seal := unittest.BootstrapFixture(participants)
			setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
			setup.SourceOfRandomness = nil

			err := state.Mutate().Bootstrap(root, result, seal)
			require.Error(t, err)
		})
	})
}

func TestBootstrapInvalidEpochCommit(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		t.Run("inconsistent counter", func(t *testing.T) {
			root, result, seal := unittest.BootstrapFixture(participants)
			setup := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
			commit := seal.ServiceEvents[1].Event.(*flow.EpochCommit)
			// use a different counter for the commit
			commit.Counter = setup.Counter + 1

			err := state.Mutate().Bootstrap(root, result, seal)
			require.Error(t, err)
		})

		t.Run("inconsistent cluster QCs", func(t *testing.T) {
			root, result, seal := unittest.BootstrapFixture(participants)
			commit := seal.ServiceEvents[1].Event.(*flow.EpochCommit)
			// add an extra QC to commit
			commit.ClusterQCs = append(commit.ClusterQCs, unittest.QuorumCertificateFixture())

			err := state.Mutate().Bootstrap(root, result, seal)
			require.Error(t, err)
		})

		t.Run("missing dkg group key", func(t *testing.T) {
			root, result, seal := unittest.BootstrapFixture(participants)
			commit := seal.ServiceEvents[1].Event.(*flow.EpochCommit)
			commit.DKGGroupKey = nil

			err := state.Mutate().Bootstrap(root, result, seal)
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

			err := state.Mutate().Bootstrap(root, result, seal)
			require.Error(t, err)
		})
	})
}

func TestExtendValid(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block, result, seal := unittest.BootstrapFixture(participants)
		err := state.Mutate().Bootstrap(block, result, seal)
		require.NoError(t, err)

		extend := unittest.BlockFixture()
		extend.Payload.Guarantees = nil
		extend.Payload.Seals = nil
		extend.Header.Height = 1
		extend.Header.View = 1
		extend.Header.ParentID = block.ID()
		extend.Header.PayloadHash = extend.Payload.Hash()

		err = state.Mutate().Extend(&extend)
		require.NoError(t, err)

		finalCommit, err := state.Final().Commit()
		assert.NoError(t, err)
		assert.Equal(t, seal.FinalState, finalCommit)
	})
}

func TestExtendSealedBoundary(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		root, result, seal := unittest.BootstrapFixture(participants)
		t.Logf("root: %x\n", root.ID())

		err := state.Mutate().Bootstrap(root, result, seal)
		require.NoError(t, err)

		finalCommit, err := state.Final().Commit()
		require.NoError(t, err)
		assert.Equal(t, seal.FinalState, finalCommit, "original commit should be root commit")

		first := unittest.BlockWithParentFixture(root.Header)
		first.SetPayload(flow.Payload{})

		extend := &flow.Seal{
			BlockID:      first.ID(),
			ResultID:     flow.ZeroID,
			InitialState: seal.FinalState,
			FinalState:   unittest.StateCommitmentFixture(),
		}

		second := unittest.BlockWithParentFixture(first.Header)
		second.SetPayload(flow.Payload{
			Seals: []*flow.Seal{extend},
		})

		err = state.Mutate().Extend(&first)
		require.NoError(t, err)

		err = state.Mutate().Extend(&second)
		require.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		require.NoError(t, err)
		assert.Equal(t, seal.FinalState, finalCommit, "commit should not change before finalizing")

		err = state.Mutate().Finalize(first.ID())
		assert.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		require.NoError(t, err)
		assert.Equal(t, seal.FinalState, finalCommit, "commit should not change after finalizing non-sealing block")

		err = state.Mutate().Finalize(second.ID())
		assert.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		require.NoError(t, err)
		assert.Equal(t, extend.FinalState, finalCommit, "commit should change after finalizing sealing block")
	})
}

func TestExtendMissingParent(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block, result, seal := unittest.BootstrapFixture(participants)
		err := state.Mutate().Bootstrap(block, result, seal)
		require.NoError(t, err)

		extend := unittest.BlockFixture()
		extend.Payload.Guarantees = nil
		extend.Payload.Seals = nil
		extend.Header.Height = 2
		extend.Header.View = 2
		extend.Header.ParentID = unittest.BlockFixture().ID()
		extend.Header.PayloadHash = extend.Payload.Hash()

		err = state.Mutate().Extend(&extend)
		require.Error(t, err)
		assert.True(t, errors.Is(err, stoerr.ErrNotFound))

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(extend.ID(), &sealID))
		require.Error(t, err)
		assert.True(t, errors.Is(err, stoerr.ErrNotFound))
	})
}

func TestExtendHeightTooSmall(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block, result, seal := unittest.BootstrapFixture(participants)
		err := state.Mutate().Bootstrap(block, result, seal)
		require.NoError(t, err)

		extend := unittest.BlockFixture()
		extend.Payload.Guarantees = nil
		extend.Payload.Seals = nil
		extend.Header.Height = 1
		extend.Header.View = 1
		extend.Header.ParentID = block.Header.ID()
		extend.Header.PayloadHash = extend.Payload.Hash()

		err = state.Mutate().Extend(&extend)
		require.NoError(t, err)

		// create another block with the same height and view, that is coming after
		extend.Header.ParentID = extend.Header.ID()
		extend.Header.Height = 1
		extend.Header.View = 2

		err = state.Mutate().Extend(&extend)
		require.Error(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(extend.ID(), &sealID))
		require.Error(t, err)
		assert.True(t, errors.Is(err, stoerr.ErrNotFound))
	})
}

func TestExtendHeightTooLarge(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		root := unittest.GenesisFixture(participants)

		block := unittest.BlockWithParentFixture(root.Header)
		block.SetPayload(flow.Payload{})
		// set an invalid height
		block.Header.Height = root.Header.Height + 2

		err := state.Mutate().Extend(&block)
		require.Error(t, err)
	})
}

func TestExtendBlockNotConnected(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block, result, seal := unittest.BootstrapFixture(participants)
		err := state.Mutate().Bootstrap(block, result, seal)
		require.NoError(t, err)

		// add 2 blocks, the second finalizing/sealing the state of the first
		extend := unittest.BlockFixture()
		extend.Payload.Guarantees = nil
		extend.Payload.Seals = nil
		extend.Header.Height = 1
		extend.Header.View = 1
		extend.Header.ParentID = block.Header.ID()
		extend.Header.PayloadHash = extend.Payload.Hash()

		err = state.Mutate().Extend(&extend)
		require.NoError(t, err)

		err = state.Mutate().Finalize(extend.ID())
		require.NoError(t, err)

		// create a fork at view/height 1 and try to connect it to root
		extend.Header.Timestamp = extend.Header.Timestamp.Add(time.Second)
		extend.Header.ParentID = block.Header.ID()

		err = state.Mutate().Extend(&extend)
		require.Error(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(extend.ID(), &sealID))
		require.Error(t, err)
		assert.True(t, errors.Is(err, stoerr.ErrNotFound))
	})
}

func TestExtendSealNotConnected(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block, result, seal := unittest.BootstrapFixture(participants)
		err := state.Mutate().Bootstrap(block, result, seal)
		require.NoError(t, err)

		extend := unittest.BlockFixture()
		extend.Payload.Guarantees = nil
		extend.Payload.Seals = nil
		extend.Header.Height = 1
		extend.Header.View = 1
		extend.Header.ParentID = block.Header.ID()
		extend.Header.PayloadHash = extend.Payload.Hash()

		err = state.Mutate().Extend(&extend)
		require.NoError(t, err)

		// create seal for the block
		second := &flow.Seal{
			BlockID:      extend.ID(),
			InitialState: unittest.StateCommitmentFixture(), // not root state
			FinalState:   unittest.StateCommitmentFixture(),
		}

		sealing := unittest.BlockFixture()
		sealing.Payload.Guarantees = nil
		sealing.Payload.Seals = []*flow.Seal{second}
		sealing.Header.Height = 2
		sealing.Header.View = 2
		sealing.Header.ParentID = block.Header.ID()
		sealing.Header.PayloadHash = sealing.Payload.Hash()

		err = state.Mutate().Extend(&sealing)
		require.Error(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(sealing.ID(), &sealID))
		require.Error(t, err)
		assert.True(t, errors.Is(err, stoerr.ErrNotFound))
	})
}

func TestExtendWrongIdentity(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block, result, seal := unittest.BootstrapFixture(participants)
		err := state.Mutate().Bootstrap(block, result, seal)
		require.NoError(t, err)

		extend := unittest.BlockFixture()
		extend.Header.Height = 1
		extend.Header.View = 1
		extend.Header.ParentID = block.ID()
		extend.Header.PayloadHash = extend.Payload.Hash()
		extend.Payload.Guarantees = nil

		err = state.Mutate().Extend(&extend)
		require.Error(t, err)
	})
}

func TestExtendInvalidChainID(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		root := unittest.GenesisFixture(participants)
		block := unittest.BlockWithParentFixture(root.Header)
		block.SetPayload(flow.Payload{})
		// use an invalid chain ID
		block.Header.ChainID = root.Header.ChainID + "-invalid"

		err := state.Mutate().Extend(&block)
		require.Error(t, err)
	})
}

// tests the full flow of transitioning between epochs by finalizing a setup
// event, then a commit event, then finalizing the first block of the next epoch
func TestExtendEpochTransitionValid(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		// first bootstrap with the initial epoch
		root, rootResult, rootSeal := unittest.BootstrapFixture(participants)
		err := state.Mutate().Bootstrap(root, rootResult, rootSeal)
		require.Nil(t, err)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentFixture(root.Header)
		block1.SetPayload(flow.Payload{})
		err = state.Mutate().Extend(&block1)
		require.Nil(t, err)
		err = state.Mutate().Finalize(block1.ID())
		require.Nil(t, err)

		epoch1Setup := rootSeal.ServiceEvents[0].Event.(*flow.EpochSetup)
		epoch1FinalView := epoch1Setup.FinalView

		// add a participant for the next epoch
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		epoch2Participants := append(participants, epoch2NewParticipant).Order(order.ByNodeIDAsc)

		// create the epoch setup event for the second epoch
		epoch2Setup := unittest.EpochSetupFixture(
			unittest.WithParticipants(epoch2Participants),
			unittest.SetupWithCounter(epoch1Setup.Counter+1),
			unittest.WithFinalView(epoch1FinalView+1000),
		)

		// create the seal referencing block1 and including the setup event
		seal1 := unittest.SealFixture(
			unittest.SealWithBlockID(block1.ID()),
			unittest.WithInitalState(rootSeal.FinalState),
			unittest.WithServiceEvents(epoch2Setup.ServiceEvent()),
		)

		// block 2 contains the epoch setup service event
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{
			Seals: []*flow.Seal{seal1},
		})

		// insert the block containing the seal containing the setup event
		err = state.Mutate().Extend(&block2)
		require.Nil(t, err)

		// we should NOT be able to query epoch 2 wrt block 1
		_, err = state.AtBlockID(block1.ID()).EpochSnapshot(epoch2Setup.Counter).InitialIdentities(filter.Any)
		require.Error(t, err)
		_, err = state.AtBlockID(block1.ID()).EpochSnapshot(epoch2Setup.Counter).Clustering()
		require.Error(t, err)

		// we should be able to query epoch 2 wrt block 2
		_, err = state.AtBlockID(block2.ID()).EpochSnapshot(epoch2Setup.Counter).InitialIdentities(filter.Any)
		require.Nil(t, err)
		_, err = state.AtBlockID(block2.ID()).EpochSnapshot(epoch2Setup.Counter).Clustering()
		require.Nil(t, err)

		// only setup event is finalized, not commit, so shouldn't be able to get certain info
		_, err = state.AtBlockID(block2.ID()).EpochSnapshot(epoch2Setup.Counter).DKG()
		assert.Error(t, err)

		epoch2Commit := unittest.EpochCommitFixture(
			unittest.CommitWithCounter(epoch2Setup.Counter),
			unittest.WithDKGFromParticipants(epoch2Participants),
		)

		seal2 := unittest.SealFixture(
			unittest.SealWithBlockID(block2.ID()),
			unittest.WithInitalState(seal1.FinalState),
			unittest.WithServiceEvents(epoch2Commit.ServiceEvent()),
		)

		// block 3 contains the epoch commit service event
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.SetPayload(flow.Payload{
			Seals: []*flow.Seal{seal2},
		})

		err = state.Mutate().Extend(&block3)
		require.Nil(t, err)

		// we should NOT be able to query epoch 2 commit info wrt block 2
		_, err = state.AtBlockID(block2.ID()).EpochSnapshot(epoch2Setup.Counter).DKG()
		assert.Error(t, err)

		// now epoch 2 is fully ready, we can query anything we want about it wrt block 3 (or later)
		_, err = state.AtBlockID(block3.ID()).EpochSnapshot(epoch2Setup.Counter).InitialIdentities(filter.Any)
		require.Nil(t, err)
		_, err = state.AtBlockID(block3.ID()).EpochSnapshot(epoch2Setup.Counter).Clustering()
		require.Nil(t, err)
		_, err = state.AtBlockID(block3.ID()).EpochSnapshot(epoch2Setup.Counter).DKG()
		assert.Nil(t, err)

		// we should still be in epoch 1
		epochCounter, err := state.AtBlockID(block3.ID()).Epoch()
		require.Nil(t, err)
		assert.Equal(t, epoch1Setup.Counter, epochCounter)

		// block 4 has the final view of the epoch
		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.SetPayload(flow.Payload{})
		block4.Header.View = epoch1FinalView

		err = state.Mutate().Extend(&block4)
		require.Nil(t, err)

		// we should still be in epoch 1, since epochs are inclusive of final view
		epochCounter, err = state.AtBlockID(block4.ID()).Epoch()
		require.Nil(t, err)
		assert.Equal(t, epoch1Setup.Counter, epochCounter)

		// block 5 has a view > final view of epoch 1, it will be considered the first block of epoch 2
		block5 := unittest.BlockWithParentFixture(block4.Header)
		block5.SetPayload(flow.Payload{})
		// we should handle view that aren't exactly the first valid view of the epoch
		block5.Header.View = epoch1FinalView + uint64(1+rand.Intn(10))

		err = state.Mutate().Extend(&block5)
		require.Nil(t, err)

		// now, at long last, we are in epoch 2
		epochCounter, err = state.AtBlockID(block5.ID()).Epoch()
		require.Nil(t, err)
		assert.Equal(t, epoch2Setup.Counter, epochCounter)
	})
}

// extending protocol state with an invalid epoch setup service event should cause an error
func TestExtendEpochSetupInvalid(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		// first bootstrap with the initial epoch
		root, rootResult, rootSeal := unittest.BootstrapFixture(participants)
		err := state.Mutate().Bootstrap(root, rootResult, rootSeal)
		require.Nil(t, err)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentFixture(root.Header)
		block1.SetPayload(flow.Payload{})
		err = state.Mutate().Extend(&block1)
		require.Nil(t, err)
		err = state.Mutate().Finalize(block1.ID())
		require.Nil(t, err)

		epoch1Setup := rootSeal.ServiceEvents[0].Event.(*flow.EpochSetup)

		// add a participant for the next epoch
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		epoch2Participants := append(participants, epoch2NewParticipant).Order(order.ByNodeIDAsc)

		// this function will return a VALID setup event and seal, we will modify
		// in different ways in each test case
		createSetup := func() (*flow.EpochSetup, *flow.Seal) {
			setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
			)
			seal := unittest.SealFixture(
				unittest.SealWithBlockID(block1.ID()),
				unittest.WithInitalState(rootSeal.FinalState),
				unittest.WithServiceEvents(setup.ServiceEvent()),
			)
			return setup, seal
		}

		t.Run("wrong counter", func(t *testing.T) {
			setup, seal := createSetup()
			setup.Counter = epoch1Setup.Counter

			block := unittest.BlockWithParentFixture(block1.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})

			err = state.Mutate().Extend(&block)
			require.Error(t, err)
		})

		t.Run("invalid final view", func(t *testing.T) {
			setup, seal := createSetup()

			block := unittest.BlockWithParentFixture(block1.Header)
			setup.FinalView = block.Header.View
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err = state.Mutate().Extend(&block)
			require.Error(t, err)
		})

		t.Run("empty seed", func(t *testing.T) {
			setup, seal := createSetup()
			setup.SourceOfRandomness = nil

			block := unittest.BlockWithParentFixture(block1.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})

			err = state.Mutate().Extend(&block)
			require.Error(t, err)
		})
	})
}

// extending protocol state with an invalid epoch commit service event should cause an error
func TestExtendEpochCommitInvalid(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		// first bootstrap with the initial epoch
		root, rootResult, rootSeal := unittest.BootstrapFixture(participants)
		err := state.Mutate().Bootstrap(root, rootResult, rootSeal)
		require.Nil(t, err)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentFixture(root.Header)
		block1.SetPayload(flow.Payload{})
		err = state.Mutate().Extend(&block1)
		require.Nil(t, err)
		err = state.Mutate().Finalize(block1.ID())
		require.Nil(t, err)

		epoch1Setup := rootSeal.ServiceEvents[0].Event.(*flow.EpochSetup)

		// swap consensus node for a new one for epoch 2
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
		epoch2Participants := append(
			participants.Filter(filter.Not(filter.HasRole(flow.RoleConsensus))),
			epoch2NewParticipant,
		).Order(order.ByNodeIDAsc)

		createSetup := func() (*flow.EpochSetup, *flow.Seal) {
			setup := unittest.EpochSetupFixture(
				unittest.WithParticipants(epoch2Participants),
				unittest.SetupWithCounter(epoch1Setup.Counter+1),
				unittest.WithFinalView(epoch1Setup.FinalView+1000),
			)
			seal := unittest.SealFixture(
				unittest.SealWithBlockID(block1.ID()),
				unittest.WithInitalState(rootSeal.FinalState),
				unittest.WithServiceEvents(setup.ServiceEvent()),
			)
			return setup, seal
		}

		createCommit := func(refBlockID flow.Identifier, initState flow.StateCommitment) (*flow.EpochCommit, *flow.Seal) {
			commit := unittest.EpochCommitFixture(
				unittest.CommitWithCounter(epoch1Setup.Counter+1),
				unittest.WithDKGFromParticipants(epoch2Participants),
			)

			seal := unittest.SealFixture(
				unittest.SealWithBlockID(refBlockID),
				unittest.WithInitalState(initState),
				unittest.WithServiceEvents(commit.ServiceEvent()),
			)
			return commit, seal
		}

		t.Run("without setup", func(t *testing.T) {
			_, seal := createCommit(block1.ID(), rootSeal.FinalState)

			block := unittest.BlockWithParentFixture(block1.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err = state.Mutate().Extend(&block)
			require.Error(t, err)
		})

		// insert the epoch setup
		epoch2Setup, setupSeal := createSetup()
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{
			Seals: []*flow.Seal{setupSeal},
		})
		err = state.Mutate().Extend(&block2)
		require.Nil(t, err)
		err = state.Mutate().Finalize(block2.ID())
		require.Nil(t, err)
		_ = epoch2Setup

		t.Run("inconsistent counter", func(t *testing.T) {
			commit, seal := createCommit(block2.ID(), setupSeal.FinalState)
			commit.Counter = epoch2Setup.Counter + 1

			block := unittest.BlockWithParentFixture(block2.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err := state.Mutate().Extend(&block)
			require.Error(t, err)
		})

		t.Run("inconsistent cluster QCs", func(t *testing.T) {
			commit, seal := createCommit(block2.ID(), setupSeal.FinalState)
			commit.ClusterQCs = append(commit.ClusterQCs, unittest.QuorumCertificateFixture())

			block := unittest.BlockWithParentFixture(block2.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err := state.Mutate().Extend(&block)
			require.Error(t, err)
		})

		t.Run("missing dkg group key", func(t *testing.T) {
			commit, seal := createCommit(block2.ID(), setupSeal.FinalState)
			commit.DKGGroupKey = nil

			block := unittest.BlockWithParentFixture(block2.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err := state.Mutate().Extend(&block)
			require.Error(t, err)
		})

		t.Run("inconsistent DKG participants", func(t *testing.T) {
			commit, seal := createCommit(block2.ID(), setupSeal.FinalState)

			// add the consensus node from epoch *1*, which was removed for epoch 2
			epoch1CONNode := participants.Filter(filter.HasRole(flow.RoleConsensus))[0]
			commit.DKGParticipants[epoch1CONNode.NodeID] = flow.DKGParticipant{
				KeyShare: unittest.KeyFixture(crypto.BLSBLS12381).PublicKey(),
				Index:    1,
			}

			block := unittest.BlockWithParentFixture(block2.Header)
			block.SetPayload(flow.Payload{
				Seals: []*flow.Seal{seal},
			})
			err := state.Mutate().Extend(&block)
			require.Error(t, err)
		})
	})
}

// if we reach the first block of the next epoch before both setup and commit
// service events are finalized, the chain should halt
func TestExtendEpochTransitionWithoutCommit(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		// first bootstrap with the initial epoch
		root, rootResult, rootSeal := unittest.BootstrapFixture(participants)
		err := state.Mutate().Bootstrap(root, rootResult, rootSeal)
		require.Nil(t, err)

		// add a block for the first seal to reference
		block1 := unittest.BlockWithParentFixture(root.Header)
		block1.SetPayload(flow.Payload{})
		err = state.Mutate().Extend(&block1)
		require.Nil(t, err)
		err = state.Mutate().Finalize(block1.ID())
		require.Nil(t, err)

		epoch1Setup := rootSeal.ServiceEvents[0].Event.(*flow.EpochSetup)
		epoch1FinalView := epoch1Setup.FinalView

		// add a participant for the next epoch
		epoch2NewParticipant := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		epoch2Participants := append(participants, epoch2NewParticipant).Order(order.ByNodeIDAsc)

		// create the epoch setup event for the second epoch
		epoch2Setup := unittest.EpochSetupFixture(
			unittest.WithParticipants(epoch2Participants),
			unittest.SetupWithCounter(epoch1Setup.Counter+1),
			unittest.WithFinalView(epoch1FinalView+1000),
		)

		// create the seal referencing block1 and including the setup event
		seal1 := unittest.SealFixture(
			unittest.SealWithBlockID(block1.ID()),
			unittest.WithInitalState(rootSeal.FinalState),
			unittest.WithServiceEvents(epoch2Setup.ServiceEvent()),
		)

		// block 2 contains the epoch setup service event
		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(flow.Payload{
			Seals: []*flow.Seal{seal1},
		})

		// insert the block containing the seal containing the setup event
		err = state.Mutate().Extend(&block2)
		require.Nil(t, err)

		// block 3 will be the first block for epoch 2
		block3 := unittest.BlockWithParentFixture(block2.Header)
		block3.Header.View = epoch2Setup.FinalView + 1

		err = state.Mutate().Extend(&block3)
		require.Error(t, err)
	})
}
