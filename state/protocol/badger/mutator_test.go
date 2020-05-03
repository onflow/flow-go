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

	"github.com/dapperlabs/flow-go/model/flow"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	stoerr "github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var participants = flow.IdentityList{
	{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
	{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
	{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
	{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
}

func TestBootstrapValid(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		err := proto.Mutate().Bootstrap(commit, genesis)
		require.NoError(t, err)

		var finalized uint64
		err = db.View(operation.RetrieveFinalizedHeight(&finalized))
		require.NoError(t, err)

		var sealed uint64
		err = db.View(operation.RetrieveSealedHeight(&sealed))
		require.NoError(t, err)

		var executed uint64
		err = db.View(operation.RetrieveExecutedHeight(&executed))
		require.NoError(t, err)

		var genesisID flow.Identifier
		err = db.View(operation.LookupBlockHeight(0, &genesisID))
		require.NoError(t, err)

		var sealedID flow.Identifier
		err = db.View(operation.LookupSealedBlock(genesisID, &sealedID))
		require.NoError(t, err)

		var header flow.Header
		err = db.View(operation.RetrieveHeader(genesisID, &header))
		require.NoError(t, err)

		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(genesisID, &sealID))
		require.NoError(t, err)

		var seal flow.Seal
		err = db.View(operation.RetrieveSeal(sealID, &seal))
		require.NoError(t, err)

		assert.Equal(t, genesis.Header.Height, finalized)
		assert.Equal(t, genesis.Header.Height, executed)
		assert.Equal(t, genesis.Header.Height, sealed)
		assert.Equal(t, genesis.ID(), genesisID)
		assert.Equal(t, genesis.ID(), sealedID)
		assert.Equal(t, genesis.Header, &header)
		assert.Equal(t, genesis.ID(), seal.BlockID)
		assert.Equal(t, commit, seal.FinalState)
	})
}

func TestExtendSealedBoundary(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		err := proto.Mutate().Bootstrap(commit, genesis)

		finalCommit, err := proto.Final().Commit()
		require.NoError(t, err)
		assert.Equal(t, commit, finalCommit, "original commit should be genisis commit")

		block := unittest.BlockFixture()
		block.Payload.Identities = nil
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.Height = 1
		block.Header.ParentID = genesis.ID()
		block.Header.PayloadHash = block.Payload.Hash()

		err = db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		seal := &flow.Seal{
			BlockID:      block.ID(),
			ResultID:     flow.ZeroID,
			InitialState: commit,
			FinalState:   unittest.StateCommitmentFixture(),
		}

		sealing := unittest.BlockFixture()
		sealing.Payload.Identities = nil
		sealing.Payload.Guarantees = nil
		sealing.Payload.Seals = []*flow.Seal{seal}
		sealing.Header.Height = 2
		sealing.Header.ParentID = block.ID()
		sealing.Header.PayloadHash = sealing.Payload.Hash()

		err = db.Update(procedure.InsertBlock(sealing.ID(), &sealing))
		require.NoError(t, err)

		err = proto.Mutate().Extend(block.ID())
		require.NoError(t, err)

		err = proto.Mutate().Extend(sealing.ID())
		require.NoError(t, err)

		finalCommit, err = proto.Final().Commit()
		require.NoError(t, err)
		assert.Equal(t, commit, finalCommit, "commit should not change before finalizing")

		err = proto.Mutate().Finalize(block.ID())
		assert.NoError(t, err)

		finalCommit, err = proto.Final().Commit()
		require.NoError(t, err)
		assert.Equal(t, commit, finalCommit, "commit should not change after finalizing non-sealing block")

		err = proto.Mutate().Finalize(sealing.ID())
		assert.NoError(t, err)

		finalCommit, err = proto.Final().Commit()
		require.NoError(t, err)
		assert.Equal(t, seal.FinalState, finalCommit, "commit should change after finalizing sealing block")
	})
}

func TestBootstrapDuplicateID(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)

		err := proto.Mutate().Bootstrap(commit, genesis)
		require.Error(t, err)
	})
}

func TestBootstrapZeroStake(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 0},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)

		err := proto.Mutate().Bootstrap(commit, genesis)
		require.Error(t, err)
	})
}

func TestBootstrapNoCollection(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)

		err := proto.Mutate().Bootstrap(commit, genesis)
		require.Error(t, err)
	})
}

func TestBootstrapNoConsensus(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)

		err := proto.Mutate().Bootstrap(commit, genesis)
		require.Error(t, err)
	})
}

func TestBootstrapNoExecution(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)

		err := proto.Mutate().Bootstrap(commit, genesis)
		require.Error(t, err)
	})
}

func TestBootstrapNoVerification(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
		}

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)

		err := proto.Mutate().Bootstrap(commit, genesis)
		require.Error(t, err)
	})
}

func TestBootstrapExistingAddress(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		participants := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a1", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)

		err := proto.Mutate().Bootstrap(commit, genesis)
		require.Error(t, err)
	})
}

func TestBootstrapNonZeroHeight(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		genesis.Header.Height = 42

		err := proto.Mutate().Bootstrap(commit, genesis)
		require.Error(t, err)
	})
}

func TestBootstrapNonZeroParent(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		genesis.Header.ParentID = unittest.IdentifierFixture()

		err := proto.Mutate().Bootstrap(commit, genesis)
		require.Error(t, err)
	})
}

func TestBootstrapNonEmptyCollections(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		genesis.Payload.Guarantees = unittest.CollectionGuaranteesFixture(1)
		genesis.Header.PayloadHash = genesis.Payload.Hash()

		err := proto.Mutate().Bootstrap(commit, genesis)
		require.Error(t, err)
	})
}

func TestBootstrapWithSeal(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		genesis.Payload.Seals = []*flow.Seal{unittest.BlockSealFixture()}
		genesis.Header.PayloadHash = genesis.Payload.Hash()

		err := proto.Mutate().Bootstrap(commit, genesis)
		require.Error(t, err)
	})
}

func TestExtendValid(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		err := proto.Mutate().Bootstrap(commit, genesis)
		require.NoError(t, err)

		block := unittest.BlockFixture()
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.Height = 1
		block.Header.View = 1
		block.Header.ParentID = genesis.ID()
		block.Header.PayloadHash = block.Payload.Hash()

		err = db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = proto.Mutate().Extend(block.ID())
		require.NoError(t, err)

		finalCommit, err := proto.Final().Commit()
		assert.NoError(t, err)
		assert.Equal(t, commit, finalCommit)
	})
}

func TestExtendMissingParent(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		err := proto.Mutate().Bootstrap(commit, genesis)
		require.NoError(t, err)

		block := unittest.BlockFixture()
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.Height = 2
		block.Header.View = 2
		block.Header.ParentID = unittest.BlockFixture().ID()
		block.Header.PayloadHash = block.Payload.Hash()

		err = db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = proto.Mutate().Extend(block.ID())
		require.Error(t, err)
		assert.True(t, errors.Is(err, stoerr.ErrNotFound))

		// verify seal not indexed
		var seal flow.Identifier
		err = db.View(operation.LookupBlockSeal(block.ID(), &seal))
		require.Error(t, err)
		assert.True(t, errors.Is(err, stoerr.ErrNotFound))
	})
}

func TestExtendHeightTooSmall(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		err := proto.Mutate().Bootstrap(commit, genesis)
		require.NoError(t, err)

		block := unittest.BlockFixture()
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.Height = 1
		block.Header.View = 1
		block.Header.ParentID = genesis.Header.ID()
		block.Header.PayloadHash = block.Payload.Hash()

		err = db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = proto.Mutate().Extend(block.ID())
		require.NoError(t, err)

		// create another block with the same height and view, that is coming after
		block.Header.ParentID = block.Header.ID()
		block.Header.Height = 1
		block.Header.View = 2

		err = db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = proto.Mutate().Extend(block.ID())
		require.Error(t, err)

		// verify seal not indexed
		var seal flow.Identifier
		err = db.View(operation.LookupBlockSeal(block.ID(), &seal))
		require.True(t, errors.Is(err, stoerr.ErrNotFound), err)
	})
}

func TestExtendHeightTooLarge(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		genesis := flow.Genesis(participants)
		mutator := state.Mutate()

		block := unittest.BlockWithParentFixture(genesis.Header)
		block.SetPayload(flow.Payload{})
		// set an invalid height
		block.Header.Height = genesis.Header.Height + 2

		err := db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = mutator.Extend(block.ID())
		unittest.AssertErrSubstringMatch(t, err, errors.New("block needs height equal to ancestor height+1"))
	})
}

func TestExtendBlockNotConnected(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		err := proto.Mutate().Bootstrap(commit, genesis)
		require.NoError(t, err)

		// add 2 blocks, the second finalizing/sealing the state of the first
		block := unittest.BlockFixture()
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.Height = 1
		block.Header.View = 1
		block.Header.ParentID = genesis.Header.ID()
		block.Header.PayloadHash = block.Payload.Hash()
		blockID := block.ID()

		seal := flow.Seal{
			BlockID:      block.ID(),
			InitialState: commit,
			FinalState:   unittest.StateCommitmentFixture(),
		}

		sealing := unittest.BlockFixture()
		sealing.Payload.Guarantees = nil
		sealing.Payload.Seals = []*flow.Seal{&seal}
		sealing.Header.Height = 2
		sealing.Header.View = 2
		sealing.Header.ParentID = block.ID()
		sealing.Header.PayloadHash = sealing.Payload.Hash()
		sealingID := sealing.ID()

		err = db.Update(func(txn *badger.Txn) error {
			err := procedure.InsertBlock(blockID, &block)(txn)
			if err != nil {
				return err
			}
			err = procedure.InsertBlock(sealingID, &sealing)(txn)
			if err != nil {
				return err
			}
			return nil
		})
		assert.NoError(t, err)

		err = proto.Mutate().Extend(block.ID())
		require.NoError(t, err)

		err = proto.Mutate().Finalize(block.ID())
		require.NoError(t, err)

		// create a fork at view/height 1 and try to connect it to genesis
		block.Header.Timestamp = block.Header.Timestamp.Add(time.Second)
		block.Header.ParentID = genesis.ID()

		err = proto.Mutate().Extend(block.ID())
		require.Error(t, err)

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(block.ID(), &sealID))
		require.Error(t, err)
	})
}

func TestExtendSealNotConnected(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		err := proto.Mutate().Bootstrap(commit, genesis)
		require.NoError(t, err)

		block := unittest.BlockFixture()
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.Height = 1
		block.Header.View = 1
		block.Header.ParentID = genesis.Header.ID()
		block.Header.PayloadHash = block.Payload.Hash()

		err = db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = proto.Mutate().Extend(block.ID())
		require.NoError(t, err)

		// create seal for the block
		seal := &flow.Seal{
			BlockID:      block.ID(),
			InitialState: unittest.StateCommitmentFixture(), // not genesis state
			FinalState:   unittest.StateCommitmentFixture(),
		}

		sealing := unittest.BlockFixture()
		sealing.Payload.Identities = nil
		sealing.Payload.Seals = []*flow.Seal{seal}
		sealing.Header.Height = 2
		sealing.Header.View = 2
		sealing.Header.ParentID = block.Header.ID()
		sealing.Header.PayloadHash = sealing.Payload.Hash()

		err = db.Update(procedure.InsertBlock(sealing.ID(), &sealing))
		require.NoError(t, err)

		err = proto.Mutate().Extend(sealing.ID())
		require.EqualError(t, err, "seal execution states do not connect")

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(sealing.ID(), &sealID))
		assert.True(t, errors.Is(err, stoerr.ErrNotFound))
	})
}

func TestExtendWrongIdentity(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, proto *protocol.State) {

		commit := unittest.StateCommitmentFixture()
		genesis := flow.Genesis(participants)
		err := proto.Mutate().Bootstrap(commit, genesis)
		require.NoError(t, err)

		block := unittest.BlockFixture()
		block.Header.Height = 1
		block.Header.View = 1
		block.Header.ParentID = genesis.ID()
		block.Header.PayloadHash = block.Payload.Hash()
		block.Payload.Guarantees = nil

		err = db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = proto.Mutate().Extend(block.ID())
		require.Error(t, err)

		// verify seal not indexed
		var seal flow.Identifier
		err = db.View(operation.LookupBlockSeal(block.ID(), &seal))
		require.Error(t, err)
	})
}

func TestExtendInvalidChainID(t *testing.T) {
	unittest.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		genesis := flow.Genesis(participants)
		mutator := state.Mutate()
		block := unittest.BlockWithParentFixture(genesis.Header)
		block.SetPayload(flow.Payload{})
		// use an invalid chain ID
		block.Header.ChainID = genesis.Header.ChainID + "-invalid"

		err := db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = mutator.Extend(block.ID())
		unittest.AssertErrSubstringMatch(t, err, errors.New("invalid chain ID"))
	})
}
