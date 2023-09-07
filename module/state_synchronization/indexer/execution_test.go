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

func TestExecutionState_HeightByBlockID(t *testing.T) {
	blocks := blocksFixture(5)
	indexer := ExecutionState{headers: newBlockHeadersStorage(blocks)}

	for _, b := range blocks {
		ret, err := indexer.HeightByBlockID(b.ID())
		require.NoError(t, err)
		require.Equal(t, b.Header.Height, ret)
	}
}

func TestExecutionState_Commitment(t *testing.T) {
	const start, end = 10, 20

	t.Run("success", func(t *testing.T) {
		indexer := ExecutionState{
			commitments: make(map[uint64]flow.StateCommitment),
		}

		commitments := indexCommitments(&indexer, start, end, t)
		for i := start; i <= end; i++ {
			ret, err := indexer.Commitment(uint64(i))
			require.NoError(t, err)
			assert.Equal(t, commitments[uint64(i)], ret)
		}
	})

	t.Run("invalid heights", func(t *testing.T) {
		indexer := ExecutionState{
			commitments: make(map[uint64]flow.StateCommitment),
		}

		_ = indexCommitments(&indexer, start, end, t)
		tests := []struct {
			height uint64
			err    string
		}{
			{height: end + 1, err: "state commitment out of indexed height bounds, current height range: [10, 20], requested height: 21"},
			{height: start - 1, err: "state commitment out of indexed height bounds, current height range: [10, 20], requested height: 9"},
		}

		for i, test := range tests {
			c, err := indexer.Commitment(test.height)
			assert.Equal(t, flow.DummyStateCommitment, c)
			assert.EqualError(t, err, test.err, fmt.Sprintf("invalid height test number %d failed", i))
		}
	})
}

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
		registers:   registers,
		headers:     headers,
		events:      events,
		commitments: make(map[uint64]flow.StateCommitment),
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

	if i.latestHeight != nil {
		i.registers.
			On("SetLatestHeight", mock.AnythingOfType("uint64")).
			Return(func(height uint64) error {
				return i.latestHeight(i.t, height)
			})
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
			setLatestHeight(func(t *testing.T, height uint64) error {
				assert.Equal(t, height, block.Header.Height)
				return nil
			}).
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

	t.Run("Index Multiple Chunks", func(t *testing.T) {
		tries := []*ledger.TrieUpdate{trieUpdateFixture(), trieUpdateFixture()}

		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{TrieUpdate: tries[0]},
				{TrieUpdate: tries[1]},
			},
		}
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)

		storeCalled := 0

		newIndexBlockDataTest(t, headers, execData).
			setLatestHeight(func(t *testing.T, height uint64) error {
				assert.Equal(t, height, block.Header.Height)
				return nil
			}).
			// make sure update registers match in length and are same as block data ledger payloads
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				fmt.Println("Set store called: ", storeCalled, height, entries)
				assert.Equal(t, height, block.Header.Height)
				assert.Len(t, tries[storeCalled].Payloads, entries.Len())

				for i, id := range entries.IDs() {
					k, _ := tries[storeCalled].Payloads[i].Key()
					regKey, _ := migrations.KeyToRegisterID(k)
					assert.Equal(t, id, regKey, fmt.Sprintf("register key doens't match the trie updates at index %d, at %d time calling store", i, storeCalled))
				}

				storeCalled++

				return nil
			}).
			run()

		assert.Len(t, ed.ChunkExecutionDatas, storeCalled)
	})

}

func indexCommitments(indexer *ExecutionState, start uint64, end uint64, t *testing.T) map[uint64]flow.StateCommitment {
	commits := commitmentsFixture(int(end - start))
	commitsHeight := make(map[uint64]flow.StateCommitment)

	for j, i := 0, start; i <= end; i++ {
		commitsHeight[i] = commits[j]
		err := indexer.indexCommitment(commits[j], i)
		require.NoError(t, err)
		j++
	}

	return commitsHeight
}

//func buildEvents() storage.Events {
//	events := storagemock.Events{}
//	events.
//		On("Store", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList")).
//		Return(func(id flow.Identifier, events []flow.EventsList) {
//
//	})
//}

func newBlockHeadersStorage(blocks []*flow.Block) storage.Headers {
	blocksByID := make(map[flow.Identifier]*flow.Block, 0)
	for _, b := range blocks {
		blocksByID[b.ID()] = b
	}

	return synctest.MockBlockHeaderStorage(synctest.WithByID(blocksByID))
}

func commitmentsFixture(n int) []flow.StateCommitment {
	commits := make([]flow.StateCommitment, n)
	for i := 0; i < n; i++ {
		commits[i] = flow.StateCommitment(unittest.IdentifierFixture())
	}

	return commits
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
