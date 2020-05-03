// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var identities = flow.IdentityList{
	{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
	{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
	{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
	{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
}

func testWithBootstrapped(t *testing.T, f func(t *testing.T, mutator *Mutator, db *badger.DB, genesis *flow.Block, commit flow.StateCommitment)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}
		genesis := unittest.GenesisFixture(identities)
		commit := unittest.StateCommitmentFixture()
		err := mutator.Bootstrap(commit, genesis)
		require.Nil(t, err)

		f(t, mutator, db, genesis, commit)
	})
}

func TestBootstrapValid(t *testing.T) {
	testWithBootstrapped(t, func(t *testing.T, mutator *Mutator, db *badger.DB, genesis *flow.Block, commit flow.StateCommitment) {
		var finalized uint64
		err := db.View(operation.RetrieveFinalizedHeight(&finalized))
		require.Nil(t, err)

		var storedID flow.Identifier
		err = db.View(operation.LookupBlockHeight(0, &storedID))
		require.Nil(t, err)

		var storedHeader flow.Header
		err = db.View(operation.RetrieveHeader(genesis.ID(), &storedHeader))
		require.Nil(t, err)

		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(storedHeader.ID(), &sealID))
		require.NoError(t, err)
		require.Equal(t, genesis.ID(), sealID)

		var storedCommit flow.StateCommitment
		err = db.View(operation.LookupStateCommitment(storedHeader.ID(), &storedCommit))
		require.Equal(t, commit, storedCommit)

		assert.Zero(t, finalized)
		assert.Equal(t, genesis.ID(), storedID)
		assert.Equal(t, genesis.Header, &storedHeader)
	})
}

func TestExtendSealedBoundary(t *testing.T) {
	testWithBootstrapped(t, func(t *testing.T, mutator *Mutator, db *badger.DB, genesis *flow.Block, commit flow.StateCommitment) {
		state := State{db: db}

		finalCommit, err := state.Final().Commit()
		assert.NoError(t, err)
		assert.Equal(t, commit, finalCommit)

		block := unittest.BlockFixture()
		block.Payload.Identities = nil
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.Height = 1
		block.Header.ParentID = genesis.ID()
		block.Header.PayloadHash = block.Payload.Hash()

		// seal
		newSeal := flow.Seal{
			BlockID:      block.ID(),
			ResultID:     flow.ZeroID, // TODO: we should probably check this
			InitialState: commit,
			FinalState:   unittest.StateCommitmentFixture(),
		}

		sealing := unittest.BlockFixture()
		sealing.Payload.Identities = nil
		sealing.Payload.Guarantees = nil
		sealing.Payload.Seals = []*flow.Seal{&newSeal}
		sealing.Header.Height = 2
		sealing.Header.ParentID = block.ID()
		sealing.Header.PayloadHash = sealing.Payload.Hash()

		err = db.Update(func(txn *badger.Txn) error {
			err = procedure.InsertBlock(block.ID(), &block)(txn)
			if err != nil {
				return err
			}
			err = procedure.InsertBlock(sealing.ID(), &sealing)(txn)
			if err != nil {
				return err
			}
			return nil
		})
		assert.NoError(t, err)

		err = mutator.Extend(block.ID())
		assert.NoError(t, err)

		err = mutator.Extend(sealing.ID())
		assert.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		assert.NoError(t, err)

		// Sealed only moves after a block is finalized
		// so here we still want to check for genesis seal
		assert.Equal(t, commit, finalCommit)

		err = mutator.Finalize(sealing.ID())
		assert.NoError(t, err)

		finalCommit, err = state.Final().Commit()
		assert.NoError(t, err)

		assert.Equal(t, newSeal.FinalState, finalCommit)
	})
}

func TestBootstrapDuplicateID(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}

		commit := unittest.StateCommitmentFixture()
		identities := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}
		genesis := unittest.GenesisFixture(identities)

		err := mutator.Bootstrap(commit, genesis)
		require.EqualError(t, err, "genesis identities not valid: duplicate node identifier "+
			"(0100000000000000000000000000000000000000000000000000000000000000)")
	})
}

func TestBootstrapZeroStake(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}

		commit := unittest.StateCommitmentFixture()
		identities := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 0},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}
		genesis := unittest.GenesisFixture(identities)

		err := mutator.Bootstrap(commit, genesis)
		require.EqualError(t, err, "genesis identities not valid: zero stake identity "+
			"(0100000000000000000000000000000000000000000000000000000000000000)")
	})
}

func TestBootstrapNoCollection(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}

		commit := unittest.StateCommitmentFixture()
		identities := flow.IdentityList{
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}
		genesis := unittest.GenesisFixture(identities)

		err := mutator.Bootstrap(commit, genesis)
		require.EqualError(t, err, "genesis identities not valid: need at least one collection node")
	})
}

func TestBootstrapNoConsensus(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}

		commit := unittest.StateCommitmentFixture()
		identities := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}
		genesis := unittest.GenesisFixture(identities)

		err := mutator.Bootstrap(commit, genesis)
		require.EqualError(t, err, "genesis identities not valid: need at least one consensus node")
	})
}

func TestBootstrapNoExecution(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}

		commit := unittest.StateCommitmentFixture()
		identities := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}
		genesis := unittest.GenesisFixture(identities)

		err := mutator.Bootstrap(commit, genesis)
		require.EqualError(t, err, "genesis identities not valid: need at least one execution node")
	})
}

func TestBootstrapNoVerification(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}

		commit := unittest.StateCommitmentFixture()
		identities := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
		}
		genesis := unittest.GenesisFixture(identities)

		err := mutator.Bootstrap(commit, genesis)
		require.EqualError(t, err, "genesis identities not valid: need at least one verification node")
	})
}

func TestBootstrapExistingAddress(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}

		commit := unittest.StateCommitmentFixture()
		identities := flow.IdentityList{
			{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
			{NodeID: flow.Identifier{0x02}, Address: "a1", Role: flow.RoleConsensus, Stake: 2},
			{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
			{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
		}
		genesis := unittest.GenesisFixture(identities)

		err := mutator.Bootstrap(commit, genesis)
		require.EqualError(t, err, "genesis identities not valid: duplicate node address (6131)")
	})
}

func TestBootstrapNonZeroHeight(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}

		commit := unittest.StateCommitmentFixture()
		genesis := unittest.GenesisFixture(identities)
		genesis.Header.Height = 42

		err := mutator.Bootstrap(commit, genesis)
		require.EqualError(t, err, "genesis header not valid: invalid initial height (42 != 0)")
	})
}

func TestBootstrapNonZeroParent(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}

		commit := unittest.StateCommitmentFixture()
		genesis := unittest.GenesisFixture(identities)
		genesis.Header.ParentID = unittest.BlockFixture().ID()

		err := mutator.Bootstrap(commit, genesis)
		require.EqualError(t, err, "genesis header not valid: genesis parent must be zero hash")
	})
}

func TestBootstrapNonEmptyCollections(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}

		commit := unittest.StateCommitmentFixture()
		genesis := unittest.GenesisFixture(identities)
		genesis.Payload.Guarantees = unittest.CollectionGuaranteesFixture(1)
		genesis.Header.PayloadHash = genesis.Payload.Hash()

		err := mutator.Bootstrap(commit, genesis)
		require.EqualError(t, err, "genesis identities not valid: genesis block must have zero guarantees")
	})
}

func TestBootstrapWithSeal(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}

		commit := unittest.StateCommitmentFixture()
		genesis := unittest.GenesisFixture(identities)
		genesis.Payload.Seals = []*flow.Seal{unittest.BlockSealFixture()}
		genesis.Header.PayloadHash = genesis.Payload.Hash()

		err := mutator.Bootstrap(commit, genesis)
		require.EqualError(t, err, "genesis identities not valid: genesis block must have zero seals")
	})
}

func TestExtendValid(t *testing.T) {
	testWithBootstrapped(t, func(t *testing.T, mutator *Mutator, db *badger.DB, genesis *flow.Block, commit flow.StateCommitment) {
		block := unittest.BlockFixture()
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.Height = 1
		block.Header.View = 1
		block.Header.ParentID = genesis.ID()
		block.Header.PayloadHash = block.Payload.Hash()

		err := db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = mutator.Extend(block.ID())
		require.NoError(t, err)

		finalCommit, err := mutator.state.Final().Commit()
		assert.NoError(t, err)
		assert.Equal(t, commit, finalCommit)
	})
}

func TestExtendMissingParent(t *testing.T) {
	testWithBootstrapped(t, func(t *testing.T, mutator *Mutator, db *badger.DB, genesis *flow.Block, commit flow.StateCommitment) {
		block := unittest.BlockFixture()
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.Height = 2
		block.Header.View = 2
		block.Header.ParentID = unittest.BlockFixture().ID()
		block.Header.PayloadHash = block.Payload.Hash()

		err := db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = mutator.Extend(block.ID())
		require.Error(t, err)
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// verify seal not indexed
		var seal flow.Identifier
		err = db.View(operation.LookupBlockSeal(block.ID(), &seal))
		require.Error(t, err)
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

func TestExtendHeightTooSmall(t *testing.T) {
	testWithBootstrapped(t, func(t *testing.T, mutator *Mutator, db *badger.DB, genesis *flow.Block, commit flow.StateCommitment) {
		block := unittest.BlockFixture()
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.Height = 1
		block.Header.View = 1
		block.Header.ParentID = genesis.Header.ID()
		block.Header.PayloadHash = block.Payload.Hash()

		err := db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = mutator.Extend(block.ID())
		require.NoError(t, err)

		// create another block with the same height and view, that is coming after
		block.Header.ParentID = block.Header.ID()
		block.Header.Height = 1
		block.Header.View = 2

		err = db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = mutator.Extend(block.ID())
		require.Error(t, err)

		// verify seal not indexed
		var seal flow.Identifier
		err = db.View(operation.LookupBlockSeal(block.ID(), &seal))
		require.True(t, errors.Is(err, storage.ErrNotFound), err)
	})
}

func TestExtendHeightTooLarge(t *testing.T) {
	testWithBootstrapped(t, func(t *testing.T, mutator *Mutator, db *badger.DB, genesis *flow.Block, commit flow.StateCommitment) {
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
	testWithBootstrapped(t, func(t *testing.T, mutator *Mutator, db *badger.DB, genesis *flow.Block, commit flow.StateCommitment) {
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

		err := db.Update(func(txn *badger.Txn) error {
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

		err = mutator.Extend(block.ID())
		require.NoError(t, err)

		err = mutator.Finalize(block.ID())
		require.NoError(t, err)

		// create a fork at view/height 1 and try to connect it to genesis
		block.Header.Timestamp = block.Header.Timestamp.Add(time.Second)
		block.Header.ParentID = genesis.ID()

		err = mutator.Extend(block.ID())
		require.EqualError(t, err, fmt.Sprintf("extend header not valid: block doesn't connect to finalized state (0 < 1), ancestorID (%v)", genesis.ID()))

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(block.ID(), &sealID))
		assert.EqualError(t, err, "key not found")
	})
}

func TestExtendSealNotConnected(t *testing.T) {
	testWithBootstrapped(t, func(t *testing.T, mutator *Mutator, db *badger.DB, genesis *flow.Block, commit flow.StateCommitment) {

		block := unittest.BlockFixture()
		block.Payload.Guarantees = nil
		block.Payload.Seals = nil
		block.Header.Height = 1
		block.Header.View = 1
		block.Header.ParentID = genesis.Header.ID()
		block.Header.PayloadHash = block.Payload.Hash()

		err := db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = mutator.Extend(block.ID())
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

		err = mutator.Extend(sealing.ID())
		require.EqualError(t, err, "seal execution states do not connect")

		// verify seal not indexed
		var sealID flow.Identifier
		err = db.View(operation.LookupBlockSeal(sealing.ID(), &sealID))
		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})
}

func TestExtendWrongIdentity(t *testing.T) {
	testWithBootstrapped(t, func(t *testing.T, mutator *Mutator, db *badger.DB, genesis *flow.Block, commit flow.StateCommitment) {
		block := unittest.BlockFixture()
		block.Header.Height = 1
		block.Header.View = 1
		block.Header.ParentID = genesis.ID()
		block.Header.PayloadHash = block.Payload.Hash()
		block.Payload.Guarantees = nil

		err := db.Update(procedure.InsertBlock(block.ID(), &block))
		require.NoError(t, err)

		err = mutator.Extend(block.ID())
		require.EqualError(t, err, "block integrity check failed")

		// verify seal not indexed
		var seal flow.Identifier
		err = db.View(operation.LookupBlockSeal(block.ID(), &seal))
		assert.EqualError(t, err, "key not found")
	})
}

func TestExtendInvalidChainID(t *testing.T) {
	testWithBootstrapped(t, func(t *testing.T, mutator *Mutator, db *badger.DB, genesis *flow.Block, commit flow.StateCommitment) {
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
