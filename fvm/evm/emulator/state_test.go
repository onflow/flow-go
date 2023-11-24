package emulator_test

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/triedb/pathdb"

	"github.com/onflow/flow-go/fvm/evm/testutils"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/common"
	gethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	gethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/model/flow"
)

var rootAddr = flow.Address{0x01}

// BenchmarkStateAccountsBalanceChange is designed to evaluate the impact of state modifications on storage size.
// It measures the bytes used in the underlying storage, aiming to understand how storage size scales with changes in state.
// During the test, each account balance is updated, with a focus on measuring any consequential changes to the state.
// While the specific operation details are not crucial for this benchmark, the primary goal is to analyze how the storage
// size evolves in response to state modifications. Users can specify the number of accounts on which the balance is updated.
// Accounts will be automatically generated, and the benchmark allows users to determine the frequency of balance
// updates across all accounts, including in-between state committing.
func benchmarkStateAccountsBalanceChange(
	t *testing.T,
	numberOfAccounts int,
	numberOfUpdatesPerAccount int,
	debug bool,
	pathDB bool,
) {
	if debug {
		log.Root().SetHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()))
	}

	backend := testutils.GetSimpleValueStore()

	db, err := database.NewMeteredDatabase(backend, rootAddr)
	require.NoError(t, err)

	rootHash, err := db.GetRootHash()
	require.NoError(t, err)

	rawDB := gethRawDB.NewDatabase(db)

	var config *trie.Config
	if pathDB {
		config = &trie.Config{
			PathDB: &pathdb.Config{},
		}
	}

	stateDB := gethState.NewDatabaseWithConfig(rawDB, config)

	accounts := make([]common.Address, numberOfAccounts)
	for i := range accounts {
		accounts[i] = common.Address{byte(i + 100)}
	}

	updateAccounts := func(blockHeight uint64, root common.Hash, balance *big.Int) common.Hash {
		state, err := gethState.New(root, stateDB, nil)
		require.NoError(t, err)

		var hash common.Hash
		for _, addr := range accounts {
			// create account and set code
			state.SetBalance(addr, balance)
			state.SetCode(addr, []byte(fmt.Sprintf("test code %d", blockHeight)))
		}

		hash, err = state.Commit(blockHeight, true)
		require.NoError(t, err)

		err = state.Database().TrieDB().Commit(hash, true)
		require.NoError(t, err)

		err = db.Commit(hash)
		require.NoError(t, err)

		db.DropCache()

		return hash
	}

	for i := 0; i < numberOfUpdatesPerAccount; i++ {
		rootHash = updateAccounts(uint64(i), rootHash, big.NewInt(int64(i)))
	}

	fmt.Println("bytes_written", db.BytesStored())
	fmt.Println("bytes_read", db.BytesRetrieved())
	fmt.Println("storage_items", backend.TotalStorageItems())
	fmt.Println("storage_size_bytes", backend.TotalStorageSize())
}

func TestStateBalanceSingleAccountSingle(t *testing.T) {
	benchmarkStateAccountsBalanceChange(t, 10, 10, false, true)
}

func TestStateBalanceSingleAccount(t *testing.T) {
	itterations := 100000
	t.Run("Hash Database", func(t *testing.T) {
		benchmarkStateAccountsBalanceChange(t, 100, itterations, false, false)
	})
	t.Run("Path Database", func(t *testing.T) {
		benchmarkStateAccountsBalanceChange(t, 100, itterations, false, true)
	})
}

func testComplexStateUpdates(t *testing.T, numberOfAccounts int, numberOfContracts int, debug bool, pathDB bool) {
	//t.Skip() // don't run in a normal test suite

	if debug {
		log.Root().SetHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()))
	}

	backend := testutils.GetSimpleValueStore()

	db, err := database.NewMeteredDatabase(backend, rootAddr)
	require.NoError(t, err)

	hash, err := db.GetRootHash()
	require.NoError(t, err)

	rawDB := gethRawDB.NewDatabase(db)

	var config *trie.Config
	if pathDB {
		config = &trie.Config{
			PathDB: &pathdb.Config{},
		}
	}

	stateDB := gethState.NewDatabaseWithConfig(rawDB, config)

	newAddress := addressGenerator()
	block := uint64(0)

	// create accounts
	accounts := make([]common.Address, numberOfAccounts)
	hash, err = withNewState(block, hash, stateDB, db, func(state *gethState.StateDB) {
		for i := range accounts {
			addr := newAddress()
			state.SetBalance(addr, big.NewInt(int64(i*10)))
			accounts[i] = addr
		}
	})

	// create test contracts
	contracts := make([]common.Address, numberOfContracts)
	contractsCode := [][]byte{
		make([]byte, 500),   // small contract
		make([]byte, 5000),  // aprox erc20 size
		make([]byte, 50000), // aprox kitty contract size
	}

	// build test storage
	contractState := make(map[common.Hash]common.Hash)
	for i := 0; i < 10; i++ {
		h := common.HexToHash(fmt.Sprintf("%d", i))
		v := common.HexToHash(fmt.Sprintf("%d %s", i, make([]byte, 32)))
		contractState[h] = v
	}

	block++
	hash, err = withNewState(block, hash, stateDB, db, func(state *gethState.StateDB) {
		for i := range contracts {
			addr := newAddress()
			state.SetBalance(addr, big.NewInt(int64(i)))
			code := contractsCode[i%len(contractsCode)]
			state.SetCode(addr, code)
			state.SetStorage(addr, contractState)
			contracts[i] = addr
		}
	})

	for _, addr := range contracts {
		block++
		hash, err = withNewState(block, hash, stateDB, db, func(state *gethState.StateDB) {
			state.SetState(addr, common.HexToHash("0x03"), common.HexToHash("0x40"))
		})
	}

	for _, addr := range accounts {
		block++
		hash, err = withNewState(block, hash, stateDB, db, func(state *gethState.StateDB) {
			state.AddBalance(addr, big.NewInt(int64(2)))
		})
	}

	t.Logf("bytes_written: %d", db.BytesStored())
	t.Logf("bytes_read: %d", db.BytesRetrieved())
	t.Logf("storage_items: %d", backend.TotalStorageItems())
	t.Logf("storage_size_bytes: %d", backend.TotalStorageSize())
}

func addressGenerator() func() common.Address {
	i := uint64(0)

	return func() common.Address {
		i++
		var addr common.Address
		binary.BigEndian.PutUint64(addr[12:], i)
		return addr
	}
}

func withNewState(
	block uint64,
	hash common.Hash,
	stateDB gethState.Database,
	db *database.MeteredDatabase,
	f func(state *gethState.StateDB),
) (common.Hash, error) {
	state, err := gethState.New(hash, stateDB, nil)
	if err != nil {
		return types.EmptyRootHash, err
	}

	f(state)

	hash, err = state.Commit(block, true)
	if err != nil {
		return types.EmptyRootHash, err
	}

	err = state.Database().TrieDB().Commit(hash, true)
	if err != nil {
		return types.EmptyRootHash, err
	}

	err = db.Commit(hash)
	if err != nil {
		return types.EmptyRootHash, err
	}

	db.DropCache()

	return hash, nil
}

func TestComplexState(t *testing.T) {
	for i := 0; i < 10; i++ {
		numberTests := 10000 * i
		t.Run(fmt.Sprintf("PathDB - %d Accounts and %d Contracts"), func(t *testing.T) {
			testComplexStateUpdates(t, 100000, 100000, false, true)
		})

		t.Run("HashDB - 1,000 Accounts and 1,000 Contracts", func(t *testing.T) {
			testComplexStateUpdates(t, 100000, 100000, false, false)
		})
	}
}
