package utils_test

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/offchain/blocks"
	"github.com/onflow/flow-go/fvm/evm/offchain/sync"
	"github.com/onflow/flow-go/fvm/evm/offchain/utils"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func ReplyingCollectionFromScratch(
	t *testing.T,
	chainID flow.ChainID,
	storage types.BackendStorage,
	filePath string,
) {

	rootAddr := evm.StorageAccountAddress(chainID)

	// setup the rootAddress account
	as := environment.NewAccountStatus()
	err := storage.SetValue(rootAddr[:], []byte(flow.AccountStatusKey), as.ToBytes())
	require.NoError(t, err)

	bp, err := blocks.NewBasicProvider(chainID, storage, rootAddr)
	require.NoError(t, err)

	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)

	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	txEvents := make([]events.TransactionEventPayload, 0)

	for scanner.Scan() {
		data := scanner.Bytes()
		var e utils.Event
		err := json.Unmarshal(data, &e)
		require.NoError(t, err)
		if strings.Contains(e.EventType, "BlockExecuted") {
			temp, err := hex.DecodeString(e.EventPayload)
			require.NoError(t, err)
			ev, err := ccf.Decode(nil, temp)
			require.NoError(t, err)
			blockEventPayload, err := events.DecodeBlockEventPayload(ev.(cadence.Event))
			require.NoError(t, err)

			err = bp.OnBlockReceived(blockEventPayload)
			require.NoError(t, err)

			sp := NewTestStorageProvider(storage, blockEventPayload.Height)
			cr := sync.NewReplayer(chainID, rootAddr, sp, bp, zerolog.Logger{}, nil, true)
			res, err := cr.ReplayBlock(txEvents, blockEventPayload)
			require.NoError(t, err)
			// commit all changes
			for k, v := range res.StorageRegisterUpdates() {
				err = storage.SetValue([]byte(k.Owner), []byte(k.Key), v)
				require.NoError(t, err)
			}

			err = bp.OnBlockExecuted(blockEventPayload.Height, res)
			require.NoError(t, err)

			txEvents = make([]events.TransactionEventPayload, 0)
			continue
		}

		temp, err := hex.DecodeString(e.EventPayload)
		require.NoError(t, err)
		ev, err := ccf.Decode(nil, temp)
		require.NoError(t, err)
		txEv, err := events.DecodeTransactionEventPayload(ev.(cadence.Event))
		require.NoError(t, err)
		txEvents = append(txEvents, *txEv)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
