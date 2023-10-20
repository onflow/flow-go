package execution

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
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
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
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
		tree := bootstrapFVM(t)

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
}

func Test_GetAccount(t *testing.T) {
	blockchain := unittest.BlockchainFixture(10)
	first := blockchain[0]
	tree := bootstrapFVM(t)

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

func Test_NewlyCreatedAccount(t *testing.T) {
	blockchain := unittest.BlockchainFixture(10)
	first := blockchain[0]
	tree := bootstrapFVM(t)
	tree, _, newAddress := createAccount(t, tree)

	scripts := newScripts(
		t,
		newBlockHeadersStorage(blockchain),
		treeToRegisterAdapter(tree),
	)

	account, err := scripts.GetAccountAtBlockHeight(context.Background(), newAddress, first.Header.Height)
	require.NoError(t, err)
	assert.Equal(t, newAddress, account.Address)
}

func Test_IntegrationStorage(t *testing.T) {
	entropyProvider := testutil.EntropyProviderFixture(nil)
	entropyBlock := mock.NewEntropyProviderPerBlock(t)
	blockchain := unittest.BlockchainFixture(10)
	headers := newBlockHeadersStorage(blockchain)

	pebbleStorage.RunWithRegistersStorageAtInitialHeights(t, 0, blockchain[0].Header.Height, func(registers *pebbleStorage.Registers) {
		entropyBlock.
			On("AtBlockID", mocks.AnythingOfType("flow.Identifier")).
			Return(entropyProvider).
			Maybe()

		snap := bootstrapFVM(t)
		snap, exeSnap, newAddress := createAccount(t, snap)

		newHeight := blockchain[1].Header.Height
		err := registers.Store(exeSnap.UpdatedRegisters(), newHeight)
		require.NoError(t, err)

		scripts, err := NewScripts(zerolog.Nop(), collector, flow.Emulator, entropyBlock, headers, func(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
			return registers.Get(ID, height)
		})
		require.NoError(t, err)

		acc, err := scripts.GetAccountAtBlockHeight(context.Background(), newAddress, newHeight)
		require.NoError(t, err)
		assert.Equal(t, acc.Address, newAddress)
	})
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
func bootstrapFVM(t *testing.T) snapshot.SnapshotTree {
	ctx := fvm.NewContext(fvm.WithChain(chain))
	vm := fvm.NewVirtualMachine()

	snapshotTree := snapshot.NewSnapshotTree(nil)

	bootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	executionSnapshot, out, err := vm.Run(
		ctx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
		snapshotTree)

	require.NoError(t, err)
	require.NoError(t, out.Err)

	return snapshotTree.Append(executionSnapshot)
}

// createAccount on an existing bootstrapped snapshot tree, execution snapshot and return a new snapshot as well as the newly created account address
func createAccount(t *testing.T, tree snapshot.SnapshotTree) (snapshot.SnapshotTree, *snapshot.ExecutionSnapshot, flow.Address) {
	const createAccountTransaction = `
	transaction {
	  prepare(signer: AuthAccount) {
		let account = AuthAccount(payer: signer)
	  }
	}
	`

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)
	vm := fvm.NewVirtualMachine()

	txBody := flow.NewTransactionBody().
		SetScript([]byte(createAccountTransaction)).
		AddAuthorizer(chain.ServiceAddress())

	executionSnapshot, output, err := vm.Run(
		ctx,
		fvm.Transaction(txBody, 0),
		tree,
	)
	require.NoError(t, err)
	require.NoError(t, output.Err)

	tree = tree.Append(executionSnapshot)

	var accountCreatedEvents []flow.Event
	for _, event := range output.Events {
		if event.Type != flow.EventAccountCreated {
			continue
		}
		accountCreatedEvents = append(accountCreatedEvents, event)
		break
	}
	require.Len(t, accountCreatedEvents, 1)

	data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
	require.NoError(t, err)
	address := flow.ConvertAddress(data.(cadence.Event).Fields[0].(cadence.Address))

	return tree, executionSnapshot, address
}

// converts tree get register function to the required script get register function
func treeToRegisterAdapter(tree snapshot.SnapshotTree) func(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	return func(ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
		return tree.Get(ID)
	}
}
