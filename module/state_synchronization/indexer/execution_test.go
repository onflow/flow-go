package indexer

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type indexBlockDataTest struct {
	t              *testing.T
	indexer        ExecutionState
	registers      *storagemock.Registers
	events         *storagemock.Events
	ctx            context.Context
	data           *execution_data.BlockExecutionDataEntity
	expectErr      error
	storeMux       sync.Mutex
	latestHeight   func(t *testing.T, height uint64) error
	registersStore func(t *testing.T, entries flow.RegisterEntries, height uint64) error
	eventsStore    func(t *testing.T, ID flow.Identifier, events []flow.EventsList) error
}

func newIndexBlockDataTest(
	t *testing.T,
	headers storage.Headers,
	exeData *execution_data.BlockExecutionDataEntity,
) *indexBlockDataTest {
	registers := storagemock.NewRegisters(t)
	events := storagemock.NewEvents(t)

	indexer := ExecutionState{
		registers: registers,
		headers:   headers,
		events:    events,
	}

	return &indexBlockDataTest{
		t:         t,
		indexer:   indexer,
		registers: registers,
		events:    events,
		ctx:       context.Background(),
		data:      exeData,
		storeMux:  sync.Mutex{},
	}
}

func (i *indexBlockDataTest) setLatestHeight(f func(t *testing.T, height uint64) error) *indexBlockDataTest {
	i.latestHeight = f
	return i
}

func (i *indexBlockDataTest) setStoreRegisters(f func(t *testing.T, entries flow.RegisterEntries, height uint64) error) *indexBlockDataTest {
	i.registersStore = f
	return i
}

func (i *indexBlockDataTest) setStoreEvents(f func(t *testing.T, ID flow.Identifier, events []flow.EventsList) error) *indexBlockDataTest {
	i.eventsStore = f
	return i
}

func (i *indexBlockDataTest) run() {

	if i.registersStore != nil {
		i.registers.
			On("Store", mock.AnythingOfType("flow.RegisterEntries"), mock.AnythingOfType("uint64")).
			Return(func(entries flow.RegisterEntries, height uint64) error {
				i.storeMux.Lock()
				defer i.storeMux.Unlock()
				return i.registersStore(i.t, entries, height)
			})
	}

	height := i.registers.On("SetLatestHeight", mock.AnythingOfType("uint64"))
	if i.latestHeight != nil {
		height.Return(func(height uint64) error {
			return i.latestHeight(i.t, height)
		})
	} else {
		height.Return(func(height uint64) error { return nil })
	}

	if i.eventsStore != nil {
		i.events.
			On("Store", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList")).
			Return(func(ID flow.Identifier, events []flow.EventsList) error {
				return i.eventsStore(i.t, ID, events)
			})
	}

	err := i.indexer.IndexBlockData(i.ctx, i.data)
	if i.expectErr != nil {
		assert.Equal(i.t, i.expectErr, err)
	} else {
		assert.NoError(i.t, err)
	}
}

func TestExecutionState_HeightByBlockID(t *testing.T) {
	blocks := blocksFixture(5)
	indexer := ExecutionState{headers: newBlockHeadersStorage(blocks)}

	for _, b := range blocks {
		ret, err := indexer.HeightByBlockID(b.ID())
		require.NoError(t, err)
		require.Equal(t, b.Header.Height, ret)
	}
}

// test cases:
// - no chunk data
// - no registers data
// - multiple inserts, same height
// - smaller invalid height
// - bigger invalid height
// - error on register updates
// - error on events
// - full register data, events, collections...

func TestExecutionState_IndexBlockData(t *testing.T) {
	blocks := blocksFixture(1)
	block := blocks[0]
	headers := newBlockHeadersStorage(blocks)

	t.Run("Index Single Chunk and Single Register", func(t *testing.T) {
		trie := trieUpdateFixture()
		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{TrieUpdate: trie},
			},
		}
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)

		newIndexBlockDataTest(t, headers, execData).
			// make sure update registers match in length and are same as block data ledger payloads
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				assert.Equal(t, height, block.Header.Height)
				assert.Len(t, trie.Payloads, entries.Len())

				for i, id := range entries.IDs() {
					k, _ := trie.Payloads[i].Key()
					regKey, _ := migrations.KeyToRegisterID(k)
					assert.Equal(t, id, regKey)
				}
				return nil
			}).
			run()
	})

	t.Run("Index Multiple Chunks and Merge Same Register Updates", func(t *testing.T) {
		tries := []*ledger.TrieUpdate{trieUpdateFixture(), trieUpdateFixture()}
		// make sure we have two register updates that are updating the same value, so we can check
		// if the value from the second update is being persisted instead of first
		tries[1].Paths[0] = tries[0].Paths[0]
		testValue := tries[1].Payloads[0]
		key, err := testValue.Key()
		require.NoError(t, err)
		testRegisterID, err := migrations.KeyToRegisterID(key)
		require.NoError(t, err)

		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{TrieUpdate: tries[0]},
				{TrieUpdate: tries[1]},
			},
		}
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)

		testRegisterFound := false
		newIndexBlockDataTest(t, headers, execData).
			// make sure update registers match in length and are same as block data ledger payloads
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				for _, entry := range entries {
					if entry.Key.String() == testRegisterID.String() {
						testRegisterFound = true
						assert.True(t, testValue.Value().Equals(entry.Value))
					}
				}
				// we should make sure the register updates are equal to both payloads' length -1 since we don't
				// duplicate the same register
				assert.Equal(t, len(tries[0].Payloads)+len(tries[1].Payloads)-1, len(entries))
				return nil
			}).
			run()

		assert.True(t, testRegisterFound)
	})

}

func newBlockHeadersStorage(blocks []*flow.Block) storage.Headers {
	blocksByID := make(map[flow.Identifier]*flow.Block, 0)
	for _, b := range blocks {
		blocksByID[b.ID()] = b
	}

	return synctest.MockBlockHeaderStorage(synctest.WithByID(blocksByID))
}

func blocksFixture(n int) []*flow.Block {
	blocks := make([]*flow.Block, n)

	genesis := unittest.BlockFixture()
	blocks[0] = &genesis
	for i := 1; i < n; i++ {
		blocks[i] = unittest.BlockWithParentFixture(blocks[i-1].Header)
	}

	return blocks
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

	payloads := make([]*ledger.Payload, 0)
	for regID, regVal := range executionSnapshot.WriteSet {
		key := ledger.Key{
			KeyParts: []ledger.KeyPart{
				{
					Type:  state.KeyPartOwner,
					Value: []byte(regID.Owner),
				},
				{
					Type:  state.KeyPartKey,
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

func trieUpdateFixture() *ledger.TrieUpdate {
	return trieUpdateWithPayloadsFixture(
		[]*ledger.Payload{
			ledgerPayloadFixture(),
			ledgerPayloadFixture(),
			ledgerPayloadFixture(),
			ledgerPayloadFixture(),
		})
}

func ledgerPayloadFixture() *ledger.Payload {
	owner := unittest.RandomAddressFixture()
	key := make([]byte, 8)
	rand.Read(key)
	val := make([]byte, 8)
	rand.Read(val)
	return ledgerPayloadWithValuesFixture(owner.String(), fmt.Sprintf("%x", key), val)
}

func ledgerPayloadWithValuesFixture(owner string, key string, value []byte) *ledger.Payload {
	k := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  state.KeyPartOwner,
				Value: []byte(owner),
			},
			{
				Type:  state.KeyPartKey,
				Value: []byte(key),
			},
		},
	}

	return ledger.NewPayload(k, value)
}
