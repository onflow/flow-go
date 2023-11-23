package emulator_test

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/common"
	gethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	gethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

var rootAddr = flow.Address{0x01}

// testSimpleAccountStateUpdate is designed to evaluate the impact of state modifications on storage size.
// It measures the bytes used in the underlying storage, aiming to understand how storage size scales with changes in state.
// During the test, each account balance is updated, with a focus on measuring any consequential changes to the state.
// While the specific operation details are not crucial for this benchmark, the primary goal is to analyze how the storage
// size evolves in response to state modifications. Users can specify the number of accounts on which the balance is updated.
// Accounts will be automatically generated, and the benchmark allows users to determine the frequency of balance
// updates across all accounts, including in-between state committing.
func testSimpleAccountStateUpdate(
	t *testing.T,
	numberOfAccounts int,
	numberOfUpdatesPerAccount int,
	debug bool,
) {
	t.Skip() // don't run in a normal test suite

	if debug {
		log.Root().SetHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()))
	}

	backend := testutils.GetSimpleValueStore()

	db, err := database.NewMeteredDatabase(backend, rootAddr)
	require.NoError(t, err)

	hash, err := db.GetRootHash()
	require.NoError(t, err)

	rawDB := gethRawDB.NewDatabase(db)
	stateDB := gethState.NewDatabase(rawDB)

	newAddress := addressGenerator()

	accounts := make([]common.Address, numberOfAccounts)
	for i := range accounts {
		accounts[i] = newAddress()
	}

	updateAccounts := func(hash common.Hash, balance *big.Int) common.Hash {
		for _, addr := range accounts {
			hash, err = withNewState(hash, stateDB, db, func(state *gethState.StateDB) {
				// create account and set code
				state.SetBalance(addr, balance)
				hash, err = state.Commit(true)
				require.NoError(t, err)
			})
		}

		return hash
	}

	for i := 0; i < numberOfUpdatesPerAccount; i++ {
		hash = updateAccounts(hash, big.NewInt(int64(i)))
	}

	t.Logf("bytes_written: %d", db.BytesStored())
	t.Logf("bytes_read: %d", db.BytesRetrieved())
	t.Logf("storage_items: %d", backend.TotalStorageItems())
	t.Logf("storage_size_bytes: %d", backend.TotalStorageSize())
}

func testComplexStateUpdates(t *testing.T, numberOfAccounts int, numberOfContracts int, debug bool) {
	t.Skip() // don't run in a normal test suite

	if debug {
		log.Root().SetHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()))
	}

	backend := testutils.GetSimpleValueStore()

	db, err := database.NewMeteredDatabase(backend, rootAddr)
	require.NoError(t, err)

	hash, err := db.GetRootHash()
	require.NoError(t, err)

	rawDB := gethRawDB.NewDatabase(db)
	stateDB := gethState.NewDatabase(rawDB)

	newAddress := addressGenerator()

	// create accounts
	accounts := make([]common.Address, numberOfAccounts)
	hash, err = withNewState(hash, stateDB, db, func(state *gethState.StateDB) {
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

	hash, err = withNewState(hash, stateDB, db, func(state *gethState.StateDB) {
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
		hash, err = withNewState(hash, stateDB, db, func(state *gethState.StateDB) {
			state.SetState(addr, common.HexToHash("0x03"), common.HexToHash("0x40"))
		})
	}

	for _, addr := range accounts {
		hash, err = withNewState(hash, stateDB, db, func(state *gethState.StateDB) {
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

	hash, err = state.Commit(true)
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

/*
TestComplexState/Single_Account_and_Single_Contract

	bytes_written: 2197
	bytes_read: 0
	storage_items: 5
	storage_size_bytes: 1560

TestComplexState/1,000_Accounts_and_Single_Contract

	bytes_written: 10658873
	bytes_read: 16731757
	storage_items: 2801
	storage_size_bytes: 1,891349

TestComplexState/1,000_Accounts_and_1,000_Contracts

	bytes_written: 25825483
	bytes_read: 39469351
	storage_items: 7726
	storage_size_bytes: 4,207,023

TestComplexState/500,000_Account_and_50,000_Contract	bytes_written: 15770922168

	bytes_read: 27327810772
	storage_items: 3088579
	storage_size_bytes: 1821,284,857
*/
func TestComplexState(t *testing.T) {
	t.Run("Single Account and Single Contract", func(t *testing.T) {
		testComplexStateUpdates(t, 1, 1, false)
	})
	t.Run("1,000 Accounts and Single Contract", func(t *testing.T) {
		testComplexStateUpdates(t, 1000, 1, false)
	})
	t.Run("1,000 Accounts and 1,000 Contracts", func(t *testing.T) {
		testComplexStateUpdates(t, 1000, 1000, false)
	})
	t.Run("500,000 Account and 50,000 Contract", func(t *testing.T) {
		testComplexStateUpdates(t, 500000, 50000, false)
	})
}

/*
bytes_written: 2271944
bytes_read: 3195554
storage_items: 160
storage_size_bytes: 167570
*/
func TestStateBalanceSingleAccount(t *testing.T) {
	testSimpleAccountStateUpdate(t, 1, 1000, false)
}

/*
bytes_written: 1474641586
bytes_read: 2532348564
storage_items: 253386
storage_size_bytes: 16,3091,602
*/
func TestStateBalanceMultipleAccounts(t *testing.T) {
	testSimpleAccountStateUpdate(t, 1000, 100, false)
}
