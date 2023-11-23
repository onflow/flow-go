package emulator_test

import (
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/triedb/pathdb"

	"github.com/onflow/flow-go/fvm/evm/testutils"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/common"
	gethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	gethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/model/flow"
)

var rootAddr = flow.Address{0x01}

// BenchmarkStateAccountsBalanceChange is designed to evaluate the impact of state modifications on storage size.
// It measures the bytes used in the underlying storage, aiming to understand how storage size scales with changes in state.
// During the test, each account balance is updated, with a focus on measuring any consequential changes to the state.
// While the specific operation details are not crucial for this benchmark, the primary goal is to analyze how the storage
// size evolves in response to state modifications. Users can specify the number of accounts on which the balance is updated.
// Accounts will be automatically generated, and the benchmark allows users to determine the frequency of balance
// updates across all accounts, including in-between state committing.
func benchmarkStateAccountsBalanceChange(
	t *testing.T,
	numberOfAccounts int,
	numberOfUpdatesPerAccount int,
	debug bool,
	pathDB bool,
) {
	if debug {
		log.Root().SetHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()))
	}

	backend := testutils.GetSimpleValueStore()

	db, err := database.NewMeteredDatabase(backend, rootAddr)
	require.NoError(t, err)

	rootHash, err := db.GetRootHash()
	require.NoError(t, err)

	rawDB := gethRawDB.NewDatabase(db)

	var config *trie.Config
	if pathDB {
		config = &trie.Config{
			PathDB: &pathdb.Config{},
		}
	}

	stateDB := gethState.NewDatabaseWithConfig(rawDB, config)

	accounts := make([]common.Address, numberOfAccounts)
	for i := range accounts {
		accounts[i] = common.Address{byte(i + 100)}
	}

	updateAccounts := func(blockHeight uint64, root common.Hash, balance *big.Int) common.Hash {
		state, err := gethState.New(root, stateDB, nil)
		require.NoError(t, err)

		var hash common.Hash
		for _, addr := range accounts {
			// create account and set code
			state.SetBalance(addr, balance)
			state.SetCode(addr, []byte(fmt.Sprintf("test code %d", blockHeight)))
		}

		hash, err = state.Commit(blockHeight, true)
		require.NoError(t, err)

		err = state.Database().TrieDB().Commit(hash, true)
		require.NoError(t, err)

		err = db.Commit(hash)
		require.NoError(t, err)

		db.DropCache()

		return hash
	}

	for i := 0; i < numberOfUpdatesPerAccount; i++ {
		rootHash = updateAccounts(uint64(i), rootHash, big.NewInt(int64(i)))
	}

	items, size := storageDataMetrics(backend.Data)
	fmt.Println("bytes_written", db.BytesStored())
	fmt.Println("bytes_read", db.BytesRetrieved())
	fmt.Println("storage_items", items)
	fmt.Println("storage_size_bytes", size)
	fmt.Println("storage_size_mb", size/1000000)
}

// calcualte storage data entries count and total storage size
func storageDataMetrics(data map[string][]byte) (entries int, size int) {
	for _, item := range data {
		entries++
		size += len(item)
	}
	return
}

func TestStateBalanceSingleAccountSingle(t *testing.T) {
	benchmarkStateAccountsBalanceChange(t, 10, 10, false, true)
}

func TestStateBalanceSingleAccount(t *testing.T) {
	itterations := 100000
	t.Run("Hash Database", func(t *testing.T) {
		benchmarkStateAccountsBalanceChange(t, 100, itterations, false, false)
	})
	t.Run("Path Database", func(t *testing.T) {
		benchmarkStateAccountsBalanceChange(t, 100, itterations, false, true)
	})
}

/**
itterations := 100000
accounts := 100

=== RUN   TestStateBalanceSingleAccount/Hash_Database
bytes_written 45,536,241,344
bytes_read 81,988,466,951
storage_items 2,467,497
storage_size_bytes 2303955180
storage_size_mb 2303
    --- PASS: TestStateBalanceSingleAccount/Hash_Database (394.44s)

=== RUN   TestStateBalanceSingleAccount/Path_Database
bytes_written 23,433,911,134
bytes_read 23,537,907,384
storage_items 411,266
storage_size_bytes 12040969
storage_size_mb 12
    --- PASS: TestStateBalanceSingleAccount/Path_Database (230.82s)
*/
