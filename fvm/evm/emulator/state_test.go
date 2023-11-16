package emulator_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"

	"github.com/onflow/flow-go/model/flow"

	gethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	gethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/stretchr/testify/require"
)

func fullKey(owner, key []byte) string {
	return string(owner) + "~" + string(key)
}

func BenchmarkStateBalance(b *testing.B) {

	testutils.RunWithTestBackend(b, func(backend types.Backend) {

		rootAddr := types.Address{0x01}
		testAddr := types.Address{0x02}.ToCommon()

		//log.Root().SetHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()))

		db, err := database.NewMeteredDatabase(backend, (flow.Address)(rootAddr.Bytes()))
		require.NoError(b, err)

		root, err := db.GetRootHash()
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

			return hash
		}

		hash := root
		for i := 0; i < 1000; i++ {
			hash = updateAccount(hash, big.NewInt(int64(i)))
			b.ReportMetric((float64)(db.BytesStored()), "bytes_used")
		}

	})

}
