package utils_test

import (
	"bufio"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/offchain/blocks"
	"github.com/onflow/flow-go/fvm/evm/offchain/sync"
	"github.com/onflow/flow-go/fvm/evm/offchain/utils"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

const resume_height = 6559268

func TestTestnetFrom(t *testing.T) {
	values, err := deserialize("./values.gob")
	require.NoError(t, err)
	allocators, err := deserializeAllocator("./allocators.gob")
	require.NoError(t, err)
	store := GetSimpleValueStorePopulated(values, allocators)
	ReplayingFromSratchFromHeight(t, flow.Testnet, store,
		fmt.Sprintf("/Users/leozhang/Downloads/devnet51_evm_events_%v.jsonl", resume_height), resume_height)
}

func TestTestnetTo(t *testing.T) {
	ReplayingFromSratchToHeight(t, flow.Testnet, GetSimpleValueStore(),
		"/Users/leozhang/Downloads/devnet51_evm_events.jsonl", resume_height-1)
}

func ReplayingFromSratchFromHeight(
	t *testing.T,
	chainID flow.ChainID,
	storage *TestValueStore,
	filePath string,
	startExecutingFromHeight uint64,
) {
	rootAddr := evm.StorageAccountAddress(chainID)
	bp, err := blocks.NewBasicProvider(chainID, storage, rootAddr)

	ReplayingBlocksFromScratch(t, storage, filePath,
		func(blockEventPayload *events.BlockEventPayload, txEvents []events.TransactionEventPayload) error {
			if blockEventPayload.Height < startExecutingFromHeight {
				return nil
			}
			fmt.Println("height: ", blockEventPayload.Height)

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

			return nil
		})
}

func ReplayingFromSratchToHeight(
	t *testing.T,
	chainID flow.ChainID,
	storage *TestValueStore,
	filePath string,
	stopAfterExecuted uint64,
) {
	rootAddr := evm.StorageAccountAddress(chainID)
	// setup the rootAddress account
	as := environment.NewAccountStatus()
	err := storage.SetValue(rootAddr[:], []byte(flow.AccountStatusKey), as.ToBytes())
	require.NoError(t, err)

	bp, err := blocks.NewBasicProvider(chainID, storage, rootAddr)
	require.NoError(t, err)

	ReplayingBlocksFromScratch(t, storage, filePath,
		func(blockEventPayload *events.BlockEventPayload, txEvents []events.TransactionEventPayload) error {
			fmt.Println("height: ", blockEventPayload.Height)
			err = bp.OnBlockReceived(blockEventPayload)
			require.NoError(t, err)

			sp := NewTestStorageProvider(storage, blockEventPayload.Height)
			cr := sync.NewReplayer(chainID, rootAddr, sp, bp, zerolog.Logger{}, nil, true)
			res, err := cr.ReplayBlock(txEvents, blockEventPayload)
			require.NoError(t, err, fmt.Sprintf("failed to replay block at height %v: %v", blockEventPayload.Height, err))
			// commit all changes
			for k, v := range res.StorageRegisterUpdates() {
				err = storage.SetValue([]byte(k.Owner), []byte(k.Key), v)
				require.NoError(t, err)
			}

			err = bp.OnBlockExecuted(blockEventPayload.Height, res)
			require.NoError(t, err)

			if blockEventPayload.Height == stopAfterExecuted {
				values, allocators := storage.Dump()
				require.NoError(t, serialize("./values.gob", values))
				require.NoError(t, serializeAllocator("./allocators.gob", allocators))
				fmt.Println("finished writing for height ", blockEventPayload.Height)

				newValues, err := deserialize("./values.gob")
				require.NoError(t, err)
				newAllocators, err := deserializeAllocator("./allocators.gob")
				require.NoError(t, err)

				checkMapConsistent(t, values, newValues)
				checkAllocatorConsistent(t, allocators, newAllocators)
				fmt.Println("finished checking for height ", blockEventPayload.Height)
				return fmt.Errorf("exeucted height reached target height: %v", stopAfterExecuted)
			}
			return nil
		})
}

func ReplayingFromSratch(
	t *testing.T,
	chainID flow.ChainID,
	storage *TestValueStore,
	filePath string,
) {
	rootAddr := evm.StorageAccountAddress(chainID)

	// setup the rootAddress account
	as := environment.NewAccountStatus()
	err := storage.SetValue(rootAddr[:], []byte(flow.AccountStatusKey), as.ToBytes())
	require.NoError(t, err)

	bp, err := blocks.NewBasicProvider(chainID, storage, rootAddr)
	require.NoError(t, err)

	ReplayingBlocksFromScratch(t, storage, filePath,
		func(blockEventPayload *events.BlockEventPayload, txEvents []events.TransactionEventPayload) error {
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

			return nil
		})
}

func ReplayingBlocksFromScratch(
	t *testing.T,
	storage *TestValueStore,
	filePath string,
	handler func(*events.BlockEventPayload, []events.TransactionEventPayload) error,
) {
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

			require.NoError(t, handler(blockEventPayload, txEvents), fmt.Sprintf("fail to handle block at height %d",
				blockEventPayload.Height))

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

func TestRemoveEventsUpToHeight(t *testing.T) {
	RemoveEventsUpToHeight(t, "/Users/leozhang/Downloads/devnet51_evm_events.jsonl", resume_height-1,
		fmt.Sprintf("/Users/leozhang/Downloads/devnet51_evm_events_%v.jsonl", resume_height))
}

func RemoveEventsUpToHeight(t *testing.T, filePath string, height uint64, newFilePath string) {
	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)

	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	newFile, err := os.Create(newFilePath)
	require.NoError(t, err)
	defer newFile.Close()

	writing := false

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

			if !writing {
				writing = blockEventPayload.Height == height
				continue
			}
		}

		if writing {
			_, err = newFile.Write(append(data, '\n'))
			require.NoError(t, err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

// Serialize function: saves map data to a file
func serialize(filename string, data map[string][]byte) error {
	// Create a file to save data
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use gob to encode data
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		return err
	}

	return nil
}

// Deserialize function: reads map data from a file
func deserialize(filename string) (map[string][]byte, error) {
	// Open the file for reading
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Prepare the map to store decoded data
	var data map[string][]byte

	// Use gob to decode data
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Serialize function: saves map data to a file
func serializeAllocator(filename string, data map[string]uint64) error {
	// Create a file to save data
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use gob to encode data
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		return err
	}

	return nil
}

// Deserialize function: reads map data from a file
func deserializeAllocator(filename string) (map[string]uint64, error) {
	// Open the file for reading
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Prepare the map to store decoded data
	var data map[string]uint64

	// Use gob to decode data
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func checkMapConsistent(t *testing.T, oldMap map[string][]byte, newMap map[string][]byte) {
	require.Equal(t, len(oldMap), len(newMap))
	for k, v := range oldMap {
		require.Equal(t, v, newMap[k])
	}
}

func checkAllocatorConsistent(t *testing.T, oldMap map[string]uint64, newMap map[string]uint64) {
	require.Equal(t, len(oldMap), len(newMap))
	for k, v := range oldMap {
		require.Equal(t, v, newMap[k])
	}
}
