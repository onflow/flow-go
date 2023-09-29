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
	"github.com/onflow/flow-go/fvm"
	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func Test_ExecuteSimpleScript(t *testing.T) {
	blockchain := unittest.BlockchainFixture(10)
	headers := newBlockHeadersStorage(blockchain)
	first := blockchain[0]

	entropyProvider := envMock.NewEntropyProvider(t)
	entropyBlock := mock.NewEntropyProviderPerBlock(t)

	entropyBlock.
		On("AtBlockID", mocks.AnythingOfType("flow.Identifier")).
		Return(entropyProvider)

	registers := func(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
		return nil, nil
	}

	scripts, err := NewScripts(zerolog.Nop(), &trace.NoopTracer{}, flow.Emulator, entropyBlock, headers, registers)
	require.NoError(t, err)

	number := int64(42)
	code := []byte(fmt.Sprintf("pub fun main(): Int { return %d; }", number))

	result, err := scripts.ExecuteAtBlockHeight(context.Background(), code, nil, first.Header.Height)
	require.NoError(t, err)
	value, err := jsoncdc.Decode(nil, result)
	require.NoError(t, err)
	assert.Equal(t, number, value.(cadence.Int).Value.Int64())
}

func Test_ComplexScriptWithRegisters(t *testing.T) {

}

func newBlockHeadersStorage(blocks []*flow.Block) storage.Headers {
	blocksByHeight := make(map[uint64]*flow.Block)
	for _, b := range blocks {
		blocksByHeight[b.Header.Height] = b
	}

	return synctest.MockBlockHeaderStorage(synctest.WithByHeight(blocksByHeight))
}

func bootstrapTrieUpdates() *ledger.TrieUpdate {
	opts := []fvm.Option{
		fvm.WithChain(flow.Testnet.Chain()),
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

	// todo deploy a contract for script to import it an check values

	payloads := make([]*ledger.Payload, 0)
	for regID, regVal := range executionSnapshot.WriteSet {
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				{
					Type:  convert.KeyPartOwner,
					Value: []byte(regID.Owner),
				},
				{
					Type:  convert.KeyPartKey,
					Value: []byte(regID.Key),
				},
			},
		}

		payloads = append(payloads, ledger.NewPayload(key, regVal))
	}

	return trieUpdateWithPayloadsFixture(payloads)
}

func trieUpdateWithPayloadsFixture(payloads []*ledger.Payload) *ledger.TrieUpdate {
	keys := make([]ledger.Key, 0)
	values := make([]ledger.Value, 0)
	for _, payload := range payloads {
		key, _ := payload.Key()
		keys = append(keys, key)
		values = append(values, payload.Value())
	}

	update, _ := ledger.NewUpdate(ledger.DummyState, keys, values)
	trie, _ := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
	return trie
}
