package execution

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/onflow/flow-go/fvm/errors"

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
	t.Run("Simple Script Execution", func(t *testing.T) {
		blockchain := unittest.BlockchainFixture(10)
		first := blockchain[0]
		tree := bootstrapFVM()

		scripts := newScripts(
			t,
			newBlockHeadersStorage(blockchain),
			treeToRegisterAdapter(tree),
		)

		number := int64(42)
		code := []byte(fmt.Sprintf("pub fun main(): Int { return %d; }", number))

		result, err := scripts.ExecuteAtBlockHeight(context.Background(), code, nil, first.Header.Height)
		require.NoError(t, err)
		value, err := jsoncdc.Decode(nil, result)
		require.NoError(t, err)
		assert.Equal(t, number, value.(cadence.Int).Value.Int64())
	})

	t.Run("Get Block", func(t *testing.T) {
		blockchain := unittest.BlockchainFixture(10)
		first := blockchain[0]
		tree := bootstrapFVM()

		scripts := newScripts(
			t,
			newBlockHeadersStorage(blockchain),
			treeToRegisterAdapter(tree),
		)

		code := []byte(fmt.Sprintf(`pub fun main(): UInt64 {
			getBlock(at: %d)!
			return getCurrentBlock().height 
		}`, first.Header.Height))

		result, err := scripts.ExecuteAtBlockHeight(context.Background(), code, nil, first.Header.Height)
		require.NoError(t, err)
		val, err := jsoncdc.Decode(nil, result)
		require.NoError(t, err)
		// make sure that the returned block height matches the current one set
		assert.Equal(t, first.Header.Height, val.(cadence.UInt64).ToGoValue())
	})

	t.Run("Handle not found Register", func(t *testing.T) {
		blockchain := unittest.BlockchainFixture(10)
		first := blockchain[0]
		scripts := newScripts(
			t,
			newBlockHeadersStorage(blockchain),
			IndexRegisterAdapter(
				func(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
					return nil, nil // intentionally return nil to check edge case
				}),
		)

		// use a non-existing address to trigger register get function
		code := []byte("import Foo from 0x01; pub fun main() { }")

		result, err := scripts.ExecuteAtBlockHeight(context.Background(), code, nil, first.Header.Height)
		require.True(t, strings.Contains(err.Error(), "invalid number of returned values for a single register"))
		require.Nil(t, result)
	})

	t.Run("Valid Argument", func(t *testing.T) {
		blockchain := unittest.BlockchainFixture(10)
		first := blockchain[0]
		tree := bootstrapFVM()

		scripts := newScripts(
			t,
			newBlockHeadersStorage(blockchain),
			treeToRegisterAdapter(tree),
		)

		code := []byte("pub fun main(foo: Int): Int { return foo }")
		arg := cadence.NewInt(2)
		encoded, err := jsoncdc.Encode(arg)
		require.NoError(t, err)

		result, err := scripts.ExecuteAtBlockHeight(
			context.Background(),
			code,
			[][]byte{encoded},
			first.Header.Height,
		)
		require.NoError(t, err)
		assert.Equal(t, encoded, result)
	})

	t.Run("Invalid Argument", func(t *testing.T) {
		blockchain := unittest.BlockchainFixture(10)
		first := blockchain[0]
		tree := bootstrapFVM()

		scripts := newScripts(
			t,
			newBlockHeadersStorage(blockchain),
			treeToRegisterAdapter(tree),
		)

		code := []byte("pub fun main(foo: Int): Int { return foo }")
		invalid := [][]byte{[]byte("i")}

		result, err := scripts.ExecuteAtBlockHeight(context.Background(), code, invalid, first.Header.Height)
		assert.Nil(t, result)
		var coded errors.CodedError
		require.True(t, errors.As(err, &coded))
		fmt.Println(coded.Code(), coded.Error())
		assert.Equal(t, errors.ErrCodeInvalidArgumentError, coded.Code())
	})
}

func Test_GetAccount(t *testing.T) {
	blockchain := unittest.BlockchainFixture(10)
	first := blockchain[0]
	tree := bootstrapFVM()

	scripts := newScripts(
		t,
		newBlockHeadersStorage(blockchain),
		treeToRegisterAdapter(tree),
	)

	address := chain.ServiceAddress()
	account, err := scripts.GetAccountAtBlockHeight(context.Background(), address, first.Header.Height)
	require.NoError(t, err)
	assert.Equal(t, address, account.Address)
	assert.NotZero(t, account.Balance)
	assert.NotZero(t, len(account.Contracts))
}

func newScripts(
	t *testing.T,
	headers storage.Headers,
	registers func(ID flow.RegisterID, height uint64) (flow.RegisterValue, error),
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

// bootstrapFVM starts up an FVM and run bootstrap procedures and returns the snapshot tree of the state.
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

// converts tree get register function to the required script get register function
func treeToRegisterAdapter(tree snapshot.SnapshotTree) func(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	return func(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
		return tree.Get(ID)
	}
}
