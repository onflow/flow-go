package emulator_test

import (
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/common"
	gethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	gethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

var rootAddr = flow.Address{0x01}

// testSimpleAccountStateUpdate is designed to evaluate the impact of state modifications on storage size.
// It measures the bytes used in the underlying storage, aiming to understand how storage size scales with changes in state.
// During the test, each account balance is updated, with a focus on measuring any consequential changes to the state.
// While the specific operation details are not crucial for this benchmark, the primary goal is to analyze how the storage
// size evolves in response to state modifications. Users can specify the number of accounts on which the balance is updated.
// Accounts will be automatically generated, and the benchmark allows users to determine the frequency of balance
// updates across all accounts, including in-between state committing.
func testSimpleAccountStateUpdate(
	t *testing.T,
	numberOfAccounts int,
	numberOfUpdatesPerAccount int,
	debug bool,
) {
	t.Skip() // don't run in a normal test suite

	if debug {
		log.Root().SetHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()))
	}

	backend := testutils.GetSimpleValueStore()

	db, err := database.NewMeteredDatabase(backend, rootAddr)
	require.NoError(t, err)

	rootHash, err := db.GetRootHash()
	require.NoError(t, err)

	rawDB := gethRawDB.NewDatabase(db)
	stateDB := gethState.NewDatabase(rawDB)

	accounts := make([]common.Address, numberOfAccounts)
	for i := range accounts {
		accounts[i] = common.Address{byte(i + 100)}
	}

	updateAccounts := func(root common.Hash, balance *big.Int) common.Hash {
		state, err := gethState.New(root, stateDB, nil)
		require.NoError(t, err)

		var hash common.Hash
		for _, addr := range accounts {
			// create account and set code
			state.SetBalance(addr, balance)
			hash, err = state.Commit(true)
			require.NoError(t, err)
		}

		err = state.Database().TrieDB().Commit(hash, true)
		require.NoError(t, err)

		err = db.Commit(hash)
		require.NoError(t, err)

		db.DropCache()

		return hash
	}

	for i := 0; i < numberOfUpdatesPerAccount; i++ {
		rootHash = updateAccounts(rootHash, big.NewInt(int64(i)))
	}

	items, size := backend.Metrics()
	t.Logf("bytes_written: %d", db.BytesStored())
	t.Logf("bytes_read: %d", db.BytesRetrieved())
	t.Logf("storage_items: %d", items)
	t.Logf("storage_size_bytes: %d", size)
}

/*
bytes_written 353897818
bytes_read 605494369
storage_items 16855058
storage_size_bytes 15911
*/
func TestStateBalanceSingleAccount(t *testing.T) {
	testSimpleAccountStateUpdate(t, 1, 1000, false)
}

/*
bytes_written 44919544
bytes_read 67108024
storage_items 6006558
storage_size_bytes 6673
*/
func TestStateBalanceMultipleAccounts(t *testing.T) {
	testSimpleAccountStateUpdate(t, 1000, 100, false)
}
