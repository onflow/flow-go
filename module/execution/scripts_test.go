package execution

import (
	"context"
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/query/mock"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

var (
	chain     = flow.Emulator.Chain()
	collector = metrics.NewExecutionCollector(&trace.NoopTracer{})
)

func Test_ExecuteScript(t *testing.T) {
	blockchain := unittest.BlockchainFixture(10)
	first := blockchain[0]
	scripts := newScripts(
		t,
		newBlockHeadersStorage(blockchain),
		func(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
			return nil, nil
		},
	)

	number := int64(42)
	code := []byte(fmt.Sprintf("pub fun main(): Int { return %d; }", number))

	result, err := scripts.ExecuteAtBlockHeight(context.Background(), code, nil, first.Header.Height)
	require.NoError(t, err)
	value, err := jsoncdc.Decode(nil, result)
	require.NoError(t, err)
	assert.Equal(t, number, value.(cadence.Int).Value.Int64())
}

func Test_GetAccount(t *testing.T) {
	blockchain := unittest.BlockchainFixture(10)
	first := blockchain[0]
	tree := bootstrapFVM()

	scripts := newScripts(
		t,
		newBlockHeadersStorage(blockchain),
		func(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
			values := make([]flow.RegisterValue, len(IDs))

			for i, ID := range IDs {
				val, err := tree.Get(ID)
				if err != nil {
					return nil, err
				}
				values[i] = val
			}

			return values, nil
		},
	)

	address := chain.ServiceAddress()
	account, err := scripts.GetAccount(context.Background(), address, first.Header.Height)
	require.NoError(t, err)
	assert.Equal(t, address, account.Address)
	assert.NotZero(t, account.Balance)
	assert.NotZero(t, len(account.Contracts))
}

func newScripts(
	t *testing.T,
	headers storage.Headers,
	registers func(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error),
) *Scripts {
	entropyProvider := testutil.EntropyProviderFixture(nil)
	entropyBlock := mock.NewEntropyProviderPerBlock(t)

	entropyBlock.
		On("AtBlockID", mocks.AnythingOfType("flow.Identifier")).
		Return(entropyProvider).
		Maybe()

	scripts, err := NewScripts(zerolog.Nop(), collector, flow.Emulator, entropyBlock, headers, registers)
	require.NoError(t, err)

	return scripts
}

func newBlockHeadersStorage(blocks []*flow.Block) storage.Headers {
	blocksByHeight := make(map[uint64]*flow.Block)
	for _, b := range blocks {
		blocksByHeight[b.Header.Height] = b
	}

	return synctest.MockBlockHeaderStorage(synctest.WithByHeight(blocksByHeight))
}

func bootstrapFVM() snapshot.SnapshotTree {
	opts := []fvm.Option{
		fvm.WithChain(chain),
	}
	ctx := fvm.NewContext(opts...)
	vm := fvm.NewVirtualMachine()

	snapshotTree := snapshot.NewSnapshotTree(nil)

	bootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	executionSnapshot, _, _ := vm.Run(
		ctx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
		snapshotTree)

	return snapshotTree.Append(executionSnapshot)
}
