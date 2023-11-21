package emulator_test

import (
	"fmt"
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

// BenchmarkStateAccountsBalanceChange is designed to evaluate the impact of state modifications on storage size.
// It measures the bytes used in the underlying storage, aiming to understand how storage size scales with changes in state.
// During the test, each account balance is updated, with a focus on measuring any consequential changes to the state.
// While the specific operation details are not crucial for this benchmark, the primary goal is to analyze how the storage
// size evolves in response to state modifications. Users can specify the number of accounts on which the balance is updated.
// Accounts will be automatically generated, and the benchmark allows users to determine the frequency of balance
// updates across all accounts, including in-between state committing.
func benchmarkStateAccountsBalanceChange(b *testing.B, numberOfAccounts int, numberOfUpdatesPerAccount int, debug bool) {
	if debug {
		log.Root().SetHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()))
	}

	f, err := os.OpenFile("./bench_10000", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(b, err)

	store := testutils.GetSimpleValueStore()
	db, err := database.NewMeteredDatabase(store, rootAddr)
	require.NoError(b, err)

	rootHash, err := db.GetRootHash()
	require.NoError(b, err)

	stateDB := gethState.NewDatabase(gethRawDB.NewDatabase(db))

	accounts := make([]common.Address, numberOfAccounts)
	for i := range accounts {
		accounts[i] = common.Address{byte(i)}
	}

	updateAccounts := func(root common.Hash, balance *big.Int) common.Hash {
		state, err := gethState.New(root, stateDB, nil)
		require.NoError(b, err)

		var hash common.Hash
		for _, addr := range accounts {
			state.SetBalance(addr, balance)
			hash, err = state.Commit(true)
			require.NoError(b, err)
		}

		err = state.Database().TrieDB().Commit(hash, true)
		require.NoError(b, err)

		err = db.Commit(hash)
		require.NoError(b, err)

		db.DropCache()

		_, count := store.Metrics()
		_, err = f.WriteString(fmt.Sprintf("%d\n", count))
		require.NoError(b, err)

		return hash
	}

	for i := 0; i < numberOfUpdatesPerAccount; i++ {
		rootHash = updateAccounts(rootHash, big.NewInt(int64(i)))
		size, items := store.Metrics()

		b.ReportMetric(float64(db.BytesWritten()), "bytes_written")
		b.ReportMetric(float64(db.BytesRead()), "bytes_read")
		b.ReportMetric(float64(size), "store_size_bytes")
		b.ReportMetric(float64(size)/1000000, "store_size_mb")
		b.ReportMetric(float64(items), "store_items_count")
	}

	f.Close()
}

/*
42284067 bytes_read
29274306 bytes_written
1593 store_items_count
1681820 store_size_bytes
1.68 store_size_mb
*/
func BenchmarkStateBalanceSingleAccount(b *testing.B) {
	benchmarkStateAccountsBalanceChange(b, 1, 10000, false)
}

/*
25492266 bytes_read
17193574 bytes_written
2515 store_items_count
2307331 store_size_bytes
2.30 store_size_mb
*/
func BenchmarkStateBalanceMultipleAccount(b *testing.B) {
	benchmarkStateAccountsBalanceChange(b, 100, 100, false)
}
