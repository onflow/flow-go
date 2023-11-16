package emulator_test

import (
	"testing"

	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"

	"github.com/onflow/flow-go/model/flow"

	"github.com/ethereum/go-ethereum/common"
	gethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	gethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/stretchr/testify/require"
)

func TestStateWrites(t *testing.T) {

	testutils.RunWithTestBackend(t, func(backend types.Backend) {
		rootAddr := types.Address{0x01}
		testAddr := types.Address{0x02}

		db, err := database.NewDatabase(backend, (flow.Address)(rootAddr.Bytes()))
		require.NoError(t, err)

		root, err := db.GetRootHash()
		require.NoError(t, err)

		stateDB := gethState.NewDatabase(gethRawDB.NewDatabase(db))

		state, err := gethState.New(root, stateDB, nil)
		require.NoError(t, err)

		stateObj := state.GetOrNewStateObject(testAddr.ToCommon())
		stateObj.SetState(stateDB, common.Hash{0x01}, common.Hash{0x02})

		hash, err := state.Commit(true)
		require.NoError(t, err)

		err = state.Database().TrieDB().Commit(hash, false)
		require.NoError(t, err)

	})

}
