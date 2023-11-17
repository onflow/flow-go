package emulator_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	gethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	gethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func benchmarkStateAccountsBalanceChange(b *testing.B, numberOfAccounts int, numberOfUpdatesPerAccount int) {
	testutils.RunWithTestBackend(b, func(backend types.Backend) {
		rootAddr := flow.Address{0x01}

		db, err := database.NewMeteredDatabase(backend, rootAddr)
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

			return hash
		}

		for i := 0; i < numberOfUpdatesPerAccount; i++ {
			rootHash = updateAccounts(rootHash, big.NewInt(int64(i)))
			b.ReportMetric(float64(db.BytesStored()), "bytes_used")
			b.ReportMetric(float64(db.BytesRetrieved()), "bytes_read")
		}

	})
}

func BenchmarkStateBalanceSingleAccount(b *testing.B) {
	benchmarkStateAccountsBalanceChange(b, 1, 10000)
}

func BenchmarkStateBalanceMultipleAccount(b *testing.B) {
	benchmarkStateAccountsBalanceChange(b, 100, 100)
}
