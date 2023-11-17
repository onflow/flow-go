package emulator_test

import (
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	gethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	gethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/log"
	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

func benchmarkStateSingleAccountBalanceChanges(b *testing.B, numberOfBalanceChanges int, debug bool) {
	testutils.RunWithTestBackend(b, func(backend types.Backend) {
		rootAddr := flow.Address{0x01}
		testAddr := common.Address{0x02}

		if debug {
			log.Root().SetHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()))
		}

		db, err := database.NewMeteredDatabase(backend, rootAddr)
		require.NoError(b, err)

		rootHash, err := db.GetRootHash()
		require.NoError(b, err)

		stateDB := gethState.NewDatabase(gethRawDB.NewDatabase(db))

		updateAccount := func(root common.Hash, balance *big.Int) common.Hash {
			state, err := gethState.New(root, stateDB, nil)
			require.NoError(b, err)

			state.SetBalance(testAddr, balance)
			hash, err := state.Commit(true)
			require.NoError(b, err)

			err = state.Database().TrieDB().Commit(hash, true)
			require.NoError(b, err)

			err = db.Commit(hash)
			require.NoError(b, err)

			db.DropCache()

			return hash
		}

		for i := 0; i < numberOfBalanceChanges; i++ {
			rootHash = updateAccount(rootHash, big.NewInt(int64(i)))
			b.ReportMetric(float64(db.BytesStored()), "bytes_used")
			b.ReportMetric(float64(db.BytesRetrieved()), "bytes_read")
		}

	})
}

func BenchmarkStateBalance(b *testing.B) {
	benchmarkStateSingleAccountBalanceChanges(b, 1000, false)
}
