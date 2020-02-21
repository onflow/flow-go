// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
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

var genesis = flow.Genesis(identities)

func testWithBootstraped(t *testing.T, f func(t *testing.T, mutator *Mutator, db *badger.DB)) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		mutator := &Mutator{state: &State{db: db}}
		err := mutator.Bootstrap(genesis)
		require.Nil(t, err)

		f(t, mutator, db)
	})
}

func TestBootStrapValid(t *testing.T) {

	testWithBootstraped(t, func(t *testing.T, mutator *Mutator, db *badger.DB) {
		var boundary uint64
		err := db.View(operation.RetrieveBoundary(&boundary))
		require.Nil(t, err)

		var storedID flow.Identifier
		err = db.View(operation.RetrieveNumber(0, &storedID))
		require.Nil(t, err)

		var storedHeader flow.Header
		err = db.View(operation.RetrieveHeader(genesis.ID(), &storedHeader))
		require.Nil(t, err)

		var storedCommit flow.StateCommitment
		err = db.View(operation.LookupCommit(storedHeader.ID(), &storedCommit))
		require.Nil(t, err)

		assert.Zero(t, boundary)
		assert.Equal(t, genesis.ID(), storedID)
		assert.Equal(t, genesis.Header, storedHeader)

		for _, identity := range identities {
			var delta int64
			err = db.View(operation.RetrieveDelta(genesis.Header.View, identity.Role, identity.NodeID, &delta))
			require.Nil(t, err)

			assert.Equal(t, int64(identity.Stake), delta)
		}
	})
}


func TestExtendSealedBoundary(t *testing.T) {
	testWithBootstraped(t, func(t *testing.T, mutator *Mutator, db *badger.DB) {

		var sealedBoundary uint64
		err := db.View(operation.RetrieveSealedBoundary(&sealedBoundary))
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), sealedBoundary)

		block := unittest.BlockFixture()
		block.Payload.Identities = nil
		block.Payload.Guarantees = nil
		block.Height = 1
		block.View = 1
		block.ParentID = genesis.ID()
		block.PayloadHash = block.Payload.Hash()
		fmt.Printf("Block# %d        Block.PayloadHash = %s        block.Payload.Hash() = %s        block.ID() = %s \n", block.Height, block.PayloadHash, block.Payload.Hash(), block.ID())

		sealingBlock := unittest.BlockFixture()
		sealingBlock.Seals = make([]*flow.Seal, 1)
		sealingBlock.Height = 2
		sealingBlock.View = 2
		sealingBlock.Identities = nil
		sealingBlock.Guarantees = nil
		sealingBlock.ParentID = block.ID()

		// seal
		seal := &flow.Seal{
			BlockID:       block.ID(),
			PreviousState: genesis.Seals[0].FinalState,
			FinalState:    unittest.StateCommitmentFixture(),
			Signature:     nil,
		}
		sealingBlock.Seals[0] = seal
		sealingBlock.PayloadHash = sealingBlock.Payload.Hash()
		fmt.Printf("Block# %d sealingBlock.PayloadHash = %s sealingBlock.Payload.Hash() = %s sealingBlock.ID() = %s \n", sealingBlock.Height, sealingBlock.PayloadHash, sealingBlock.Payload.Hash(), sealingBlock.ID())

		err = db.Update(func(txn *badger.Txn) error {
			err = procedure.InsertPayload(&block.Payload)(txn)
			if err != nil {
				return err
			}
			err = procedure.InsertBlock(&block)(txn)
			if err != nil {
				return err
			}
			err := operation.InsertSeal(seal)(txn)
			if err != nil {
				return err
			}
			err = procedure.InsertPayload(&sealingBlock.Payload)(txn)
			if err != nil {
				return err
			}
			err = procedure.InsertBlock(&sealingBlock)(txn)
			if err != nil {
				return err
			}
			return nil
		})
		assert.NoError(t, err)

		err = mutator.Extend(block.ID())
		assert.NoError(t, err)

		err = mutator.Extend(sealingBlock.ID())
		assert.NoError(t, err)

		err = db.View(operation.RetrieveSealedBoundary(&sealedBoundary))
		assert.NoError(t, err)
		assert.Equal(t, uint64(1), sealedBoundary)

	})
}

func TestBootstrapDuplicateID(t *testing.T) {
	// TODO
}

func TestBootstrapZeroStake(t *testing.T) {
	// TODO
}

func TestBootstrapExistingRole(t *testing.T) {
	// TODO
}

func TestBootstrapExistingAddress(t *testing.T) {
	// TODO
}

func TestBootstrapNonZeroNumber(t *testing.T) {
	// TODO
}

func TestBootstrapNonZeroParent(t *testing.T) {
	// TODO
}

func TestBootstrapNonEmptyCollections(t *testing.T) {
	// TODO
}

func TestExtendValid(t *testing.T) {
	// TODO
}

func TestExtendDuplicateID(t *testing.T) {
	// TODO
}

func TestExtendZeroStake(t *testing.T) {
	// TODO
}

func TestExtendExistingRole(t *testing.T) {
	// TODO
}

func TestExtendExistingAddress(t *testing.T) {
	// TODO
}

func TestExtendMissingParent(t *testing.T) {
	// TODO
}

func TestExtendNumberTooSmall(t *testing.T) {
	// TODO
}

func TestExtendNotConnected(t *testing.T) {
	// TODO
}
