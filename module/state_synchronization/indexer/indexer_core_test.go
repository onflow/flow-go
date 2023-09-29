package indexer

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

type indexCoreTest struct {
	t                *testing.T
	indexer          *IndexerCore
	registers        *storagemock.RegisterIndex
	events           *storagemock.Events
	headers          *storagemock.Headers
	ctx              context.Context
	blocks           []*flow.Block
	data             *execution_data.BlockExecutionDataEntity
	lastHeightStore  func(t *testing.T) uint64
	firstHeightStore func(t *testing.T) uint64
	registersStore   func(t *testing.T, entries flow.RegisterEntries, height uint64) error
	eventsStore      func(t *testing.T, ID flow.Identifier, events []flow.EventsList) error
	registersGet     func(t *testing.T, IDs flow.RegisterID, height uint64) (flow.RegisterValue, error)
}

func newIndexCoreTest(
	t *testing.T,
	blocks []*flow.Block,
	exeData *execution_data.BlockExecutionDataEntity,
) *indexCoreTest {
	return &indexCoreTest{
		t:         t,
		registers: storagemock.NewRegisterIndex(t),
		events:    storagemock.NewEvents(t),
		blocks:    blocks,
		ctx:       context.Background(),
		data:      exeData,
		headers:   newBlockHeadersStorage(blocks).(*storagemock.Headers), // convert it back to mock type for tests
	}
}

func (i *indexCoreTest) useDefaultBlockByHeight() *indexCoreTest {
	i.headers.
		On("BlockIDByHeight", mocks.AnythingOfType("uint64")).
		Return(func(height uint64) (flow.Identifier, error) {
			for _, b := range i.blocks {
				if b.Header.Height == height {
					return b.ID(), nil
				}
			}
			return flow.ZeroID, fmt.Errorf("not found")
		})

	return i
}

func (i *indexCoreTest) setLastHeight(f func(t *testing.T) uint64) *indexCoreTest {
	i.registers.
		On("LatestHeight").
		Return(func() uint64 {
			return f(i.t)
		})
	return i
}

func (i *indexCoreTest) useDefaultLastHeight() *indexCoreTest {
	i.registers.
		On("LatestHeight").
		Return(func() uint64 {
			return i.blocks[len(i.blocks)-1].Header.Height
		})
	return i
}

func (i *indexCoreTest) setStoreRegisters(f func(t *testing.T, entries flow.RegisterEntries, height uint64) error) *indexCoreTest {
	i.registers.
		On("Store", mock.AnythingOfType("flow.RegisterEntries"), mock.AnythingOfType("uint64")).
		Return(func(entries flow.RegisterEntries, height uint64) error {
			return f(i.t, entries, height)
		}).Once()
	return i
}

func (i *indexCoreTest) setStoreEvents(f func(t *testing.T, ID flow.Identifier, events []flow.EventsList) error) *indexCoreTest {
	i.events.
		On("Store", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList")).
		Return(func(ID flow.Identifier, events []flow.EventsList) error {
			return f(i.t, ID, events)
		})
	return i
}

func (i *indexCoreTest) setGetRegisters(f func(t *testing.T, ID flow.RegisterID, height uint64) (flow.RegisterValue, error)) *indexCoreTest {
	i.registers.
		On("Get", mock.AnythingOfType("flow.RegisterID"), mock.AnythingOfType("uint64")).
		Return(func(IDs flow.RegisterID, height uint64) (flow.RegisterValue, error) {
			return f(i.t, IDs, height)
		})
	return i
}

func (i *indexCoreTest) useDefaultEvents() *indexCoreTest {
	i.events.
		On("Store", mock.AnythingOfType("flow.Identifier"), mock.AnythingOfType("[]flow.EventsList")).
		Return(nil)
	return i
}

func (i *indexCoreTest) initIndexer() *indexCoreTest {
	indexer, err := New(zerolog.New(os.Stdout), i.registers, i.headers, i.events)
	require.NoError(i.t, err)
	i.indexer = indexer
	return i
}

func (i *indexCoreTest) runIndexBlockData() error {
	i.initIndexer()
	return i.indexer.IndexBlockData(i.data)
}

func (i *indexCoreTest) runGetRegisters(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error) {
	i.initIndexer()
	return i.indexer.RegisterValues(IDs, height)
}

func TestExecutionState_IndexBlockData(t *testing.T) {
	blocks := blocksFixture(5)
	block := blocks[len(blocks)-1]

	// this test makes sure the index block data is correctly calling store register with the
	// same entries we create as a block execution data test, and correctly converts the registers
	t.Run("Index Single Chunk and Single Register", func(t *testing.T) {
		trie := trieUpdateFixture(t)
		ed := &execution_data.BlockExecutionData{
			BlockID: block.ID(),
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
				{TrieUpdate: trie},
			},
		}
		execData := execution_data.NewBlockExecutionDataEntity(block.ID(), ed)

		err := newIndexCoreTest(t, blocks, execData).
			initIndexer().
			useDefaultLastHeight().
			useDefaultEvents().
			// make sure update registers match in length and are same as block data ledger payloads
			setStoreRegisters(func(t *testing.T, entries flow.RegisterEntries, height uint64) error {
				assert.Equal(t, height, block.Header.Height)
				assert.Len(t, trie.Payloads, entries.Len())

				// make sure all the registers from the execution data have been stored as well the value matches
				trieRegistersPayloadComparer(t, trie.Payloads, entries)
				return nil
			}).
			runIndexBlockData()

		assert.NoError(t, err)
	})

	// this test makes sure that if we have multiple trie updates in a single block data
	// and some of those trie updates are for same register but have different values,
	// we only update that register once with the latest value, so this makes sure merging of
	// registers is done correctly.
	t.Run("Index Multiple Chunks and Merge Same Register Updates", func(t *testing.T) {
		tries := []*ledger.TrieUpdate{trieUpdateFixture(t), trieUpdateFixture(t)}
		// make sure we have two register updates that are updating the same value, so we can check
		// if the value from the second update is being persisted instead of first
		tries[1].Paths[0] = tries[0].Paths[0]
		testValue := tries[1].Payloads[0]
		key, err := testValue.Key()
		require.NoError(t, err)
		testRegisterID, err := convert.LedgerKeyToRegisterID(key)
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
		err = newIndexCoreTest(t, blocks, execData).
			initIndexer().
			useDefaultEvents().
			useDefaultLastHeight().
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
			runIndexBlockData()

		assert.NoError(t, err)
		assert.True(t, testRegisterFound)
	})

	// this test makes sure we get correct error when we try to index block that is not
	// within the range of indexed heights.
	t.Run("Invalid Heights", func(t *testing.T) {
		last := blocks[len(blocks)-1]
		ed := &execution_data.BlockExecutionData{
			BlockID: last.Header.ID(),
		}
		execData := execution_data.NewBlockExecutionDataEntity(last.ID(), ed)
		latestHeight := blocks[len(blocks)-3].Header.Height

		err := newIndexCoreTest(t, blocks, execData).
			// return a height one smaller than the latest block in storage
			setLastHeight(func(t *testing.T) uint64 {
				return latestHeight
			}).
			runIndexBlockData()

		assert.EqualError(t, err, fmt.Sprintf("must store registers with the next height %d, but got %d", latestHeight+1, last.Header.Height))
	})

	// this test makes sure that if a block we try to index is not found in block storage
	// we get correct error.
	t.Run("Unknown block ID", func(t *testing.T) {
		unknownBlock := blocksFixture(1)[0]
		ed := &execution_data.BlockExecutionData{
			BlockID: unknownBlock.Header.ID(),
		}
		execData := execution_data.NewBlockExecutionDataEntity(unknownBlock.Header.ID(), ed)

		err := newIndexCoreTest(t, blocks, execData).runIndexBlockData()

		assert.True(t, errors.Is(err, storage.ErrNotFound))
	})

}

func TestExecutionState_RegisterValues(t *testing.T) {
	t.Run("Get value for single register", func(t *testing.T) {
		blocks := blocksFixture(5)
		height := blocks[1].Header.Height
		ids := []flow.RegisterID{{
			Owner: "1",
			Key:   "2",
		}}
		val := flow.RegisterValue("0x1")

		values, err := newIndexCoreTest(t, blocks, nil).
			initIndexer().
			setGetRegisters(func(t *testing.T, ID flow.RegisterID, height uint64) (flow.RegisterValue, error) {
				return val, nil
			}).
			runGetRegisters(ids, height)

		assert.NoError(t, err)
		assert.Equal(t, values, []flow.RegisterValue{val})
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

func trieUpdateFixture(t *testing.T) *ledger.TrieUpdate {
	return trieUpdateWithPayloadsFixture(
		[]*ledger.Payload{
			ledgerPayloadFixture(t),
			ledgerPayloadFixture(t),
			ledgerPayloadFixture(t),
			ledgerPayloadFixture(t),
		})
}

func ledgerPayloadFixture(t *testing.T) *ledger.Payload {
	owner := unittest.RandomAddressFixture()
	key := make([]byte, 8)
	_, err := rand.Read(key)
	require.NoError(t, err)
	val := make([]byte, 8)
	_, err = rand.Read(key)
	require.NoError(t, err)
	return ledgerPayloadWithValuesFixture(owner.String(), fmt.Sprintf("%x", key), val)
}

func ledgerPayloadWithValuesFixture(owner string, key string, value []byte) *ledger.Payload {
	k := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  convert.KeyPartOwner,
				Value: []byte(owner),
			},
			{
				Type:  convert.KeyPartKey,
				Value: []byte(key),
			},
		},
	}

	return ledger.NewPayload(k, value)
}

// trieRegistersPayloadComparer checks that trie payloads and register payloads are same, used for testing.
func trieRegistersPayloadComparer(t *testing.T, triePayloads []*ledger.Payload, registerPayloads flow.RegisterEntries) {
	assert.Equal(t, len(triePayloads), len(registerPayloads.Values()), "registers length should equal")

	// crate a lookup map that matches flow register ID to index in the payloads slice
	payloadRegID := make(map[flow.RegisterID]int)
	for i, p := range triePayloads {
		k, _ := p.Key()
		regKey, _ := convert.LedgerKeyToRegisterID(k)
		payloadRegID[regKey] = i
	}

	for _, entry := range registerPayloads {
		index, ok := payloadRegID[entry.Key]
		assert.True(t, ok, fmt.Sprintf("register entry not found for key %s", entry.Key.String()))
		val := triePayloads[index].Value()
		assert.True(t, val.Equals(entry.Value), fmt.Sprintf("payload values not same %s - %s", val, entry.Value))
	}
}

func TestIndexerIntegration_StoreAndGet(t *testing.T) {
	regOwner := "f8d6e0586b0a20c7"
	regKey := "code"
	registerID := flow.NewRegisterID(regOwner, regKey)

	// this test makes sure index values for a single register are correctly updated and always last value is returned
	t.Run("Single Index Value Changes", func(t *testing.T) {
		pebbleStorage.RunWithRegistersStorageAtInitialHeights(t, 0, 0, func(registers *pebbleStorage.Registers) {
			index, err := New(zerolog.Nop(), registers, nil, nil)
			require.NoError(t, err)

			values := [][]byte{[]byte("1"), []byte("1"), []byte("2"), []byte("3") /*nil,*/, []byte("4")}

			value, err := index.RegisterValues(flow.RegisterIDs{registerID}, 0)
			require.Nil(t, value)
			assert.ErrorIs(t, err, storage.ErrNotFound)

			for i, val := range values {
				testDesc := fmt.Sprintf("test itteration number %d failed with test value %s", i, val)
				height := uint64(i + 1)
				err := storeRegisterWithValue(index, height, regOwner, regKey, val)
				assert.NoError(t, err)

				results, err := index.RegisterValues(flow.RegisterIDs{registerID}, height)
				require.Nil(t, err, testDesc)
				assert.Equal(t, val, results[0])
			}
		})
	})

	// this test makes sure that even if indexed values for a specific register are requested with higher height
	// the correct highest height indexed value is returned.
	// e.g. we index A{h(1) -> X}, A{h(2) -> Y}, when we request h(4) we get value Y
	t.Run("Single Index Value At Later Heights", func(t *testing.T) {
		pebbleStorage.RunWithRegistersStorageAtInitialHeights(t, 0, 0, func(registers *pebbleStorage.Registers) {
			index, err := New(zerolog.Nop(), registers, nil, nil)
			require.NoError(t, err)

			storeValues := [][]byte{[]byte("1"), []byte("2")}

			require.NoError(t, storeRegisterWithValue(index, 1, regOwner, regKey, storeValues[0]))

			require.NoError(t, index.indexRegisters(nil, 2))

			value, err := index.RegisterValues(flow.RegisterIDs{registerID}, uint64(2))
			require.Nil(t, err)
			assert.Equal(t, storeValues[0], value[0])

			require.NoError(t, index.indexRegisters(nil, 3))

			err = storeRegisterWithValue(index, 4, regOwner, regKey, storeValues[1])
			require.NoError(t, err)

			value, err = index.RegisterValues(flow.RegisterIDs{registerID}, uint64(4))
			require.Nil(t, err)
			assert.Equal(t, storeValues[1], value[0])

			value, err = index.RegisterValues(flow.RegisterIDs{registerID}, uint64(3))
			require.Nil(t, err)
			assert.Equal(t, storeValues[0], value[0])
		})
	})

	// this test makes sure we correctly handle weird payloads
	t.Run("Empty and Nil Payloads", func(t *testing.T) {
		pebbleStorage.RunWithRegistersStorageAtInitialHeights(t, 0, 0, func(registers *pebbleStorage.Registers) {
			index, err := New(zerolog.Nop(), registers, nil, nil)
			require.NoError(t, err)

			require.NoError(t, index.indexRegisters(map[ledger.Path]*ledger.Payload{}, 1))
			require.NoError(t, index.indexRegisters(map[ledger.Path]*ledger.Payload{}, 1))
			require.NoError(t, index.indexRegisters(nil, 2))
		})
	})
}

// helper to store register at height and increment index range
func storeRegisterWithValue(indexer *IndexerCore, height uint64, owner string, key string, value []byte) error {
	payload := ledgerPayloadWithValuesFixture(owner, key, value)
	return indexer.indexRegisters(map[ledger.Path]*ledger.Payload{ledger.DummyPath: payload}, height)
}
