// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger_test

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
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/state/protocol/util"
	stoerr "github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
<<<<<<< HEAD
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
=======
>>>>>>> switched bootstrapping to input state & output seal
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
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapNonZeroParent(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block := unittest.GenesisFixture(participants)
		block.Header.ParentID = unittest.IdentifierFixture()

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapNonEmptyCollections(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block := unittest.GenesisFixture(participants)
		block.Payload.Guarantees = unittest.CollectionGuaranteesFixture(1)
		block.Header.PayloadHash = block.Payload.Hash()

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestBootstrapWithSeal(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block := unittest.GenesisFixture(participants)
		block.Payload.Seals = []*flow.Seal{unittest.BlockSealFixture()}
		block.Header.PayloadHash = block.Payload.Hash()

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

		err := state.Mutate().Bootstrap(block, result, seal)
		require.Error(t, err)
	})
}

func TestExtendValid(t *testing.T) {
	util.RunWithProtocolState(t, func(db *badger.DB, state *protocol.State) {

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.InitialState = nil
		seal.FinalState = result.FinalStateCommit

		fmt.Printf("root: %x\n", block.ID())

		err := state.Mutate().Bootstrap(block, result, seal)
		require.NoError(t, err)

		finalCommit, err := state.Final().Commit()
		require.NoError(t, err)
		assert.Equal(t, seal.FinalState, finalCommit, "original commit should be root commit")

		first := unittest.BlockFixture()
		first.Payload.Identities = nil
		first.Payload.Guarantees = nil
		first.Payload.Seals = nil
		first.Header.Height = 1
		first.Header.ParentID = block.ID()
		first.Header.PayloadHash = first.Payload.Hash()

		extend := &flow.Seal{
			BlockID:      first.ID(),
			ResultID:     flow.ZeroID,
			InitialState: seal.FinalState,
			FinalState:   unittest.StateCommitmentFixture(),
		}

		second := unittest.BlockFixture()
		second.Payload.Identities = nil
		second.Payload.Guarantees = nil
		second.Payload.Seals = []*flow.Seal{extend}
		second.Header.Height = 2
		second.Header.ParentID = first.ID()
		second.Header.PayloadHash = second.Payload.Hash()

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

		err := state.Mutate().Bootstrap(block, result, seal)
		require.NoError(t, err)

		extend := unittest.BlockFixture()
		extend.Payload.Identities = nil
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
		sealing.Payload.Identities = nil
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

		block := unittest.GenesisFixture(participants)

		result := unittest.ExecutionResultFixture()
		result.BlockID = block.ID()

		seal := unittest.BlockSealFixture()
		seal.BlockID = block.ID()
		seal.ResultID = result.ID()
		seal.FinalState = result.FinalStateCommit

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
