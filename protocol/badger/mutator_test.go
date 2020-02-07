// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestBootStrapValid(t *testing.T) {

	identities := flow.IdentityList{
		{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.RoleCollection, Stake: 1},
		{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.RoleConsensus, Stake: 2},
		{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.RoleExecution, Stake: 3},
		{NodeID: flow.Identifier{0x04}, Address: "a4", Role: flow.RoleVerification, Stake: 4},
	}

	genesis := flow.Genesis(identities)
	blockID := genesis.ID()

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		mutator := &Mutator{state: &State{db: db}}
		err := mutator.Bootstrap(genesis)
		require.Nil(t, err)

		var boundary uint64
		err = db.View(operation.RetrieveBoundary(&boundary))
		require.Nil(t, err)

		var storedID flow.Identifier
		err = db.View(operation.RetrieveNumber(0, &storedID))
		require.Nil(t, err)

		var storedHeader flow.Header
		err = db.View(operation.RetrieveHeader(genesis.ID(), &storedHeader))
		require.Nil(t, err)

		var storedCommit flow.StateCommitment
		err = db.View(operation.LookupCommit(blockID, &storedCommit))
		require.Nil(t, err)

		assert.Zero(t, boundary)
		assert.Equal(t, blockID, storedID)
		assert.Equal(t, genesis.Header, storedHeader)

		for _, identity := range identities {

			var delta int64
			err = db.View(operation.RetrieveDelta(genesis.Header.Number, identity.Role, identity.NodeID, &delta))
			require.Nil(t, err)

			assert.Equal(t, int64(identity.Stake), delta)
		}
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
