package emulator_test

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/onflow/flow-go/utils/io"

	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/ethereum/go-ethereum/common"
	gethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	gethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

const (
	storageBytesMetric = "storage_size_bytes"
	storageItemsMetric = "storage_items"
	bytesReadMetric    = "bytes_read"
	bytesWrittenMetric = "bytes_written"
)

// storage test is designed to evaluate the impact of state modifications on storage size.
// It measures the bytes used in the underlying storage, aiming to understand how storage size scales with changes in state.
// While the specific operation details are not crucial for this benchmark, the primary goal is to analyze how the storage
// size evolves in response to state modifications.

type storageTest struct {
	store        *testutils.TestValueStore
	db           *database.MeteredDatabase
	ethDB        ethdb.Database
	stateDB      gethState.Database
	addressIndex uint64
	hash         common.Hash
	metrics      *metrics
}

func newStorageTest() (*storageTest, error) {
	simpleStore := testutils.GetSimpleValueStore()

	db, err := database.NewMeteredDatabase(simpleStore, flow.Address{0x01})
	if err != nil {
		return nil, err
	}

	hash, err := db.GetRootHash()
	if err != nil {
		return nil, err
	}

	rawDB := gethRawDB.NewDatabase(db)
	stateDB := gethState.NewDatabase(rawDB)

	return &storageTest{
		store:        simpleStore,
		db:           db,
		ethDB:        rawDB,
		stateDB:      stateDB,
		addressIndex: 100,
		hash:         hash,
		metrics:      newMetrics(),
	}, nil
}

func (s *storageTest) newAddress() common.Address {
	s.addressIndex++
	var addr common.Address
	binary.BigEndian.PutUint64(addr[12:], s.addressIndex)
	return addr
}

// run the provided runner with a newly created state which gets comitted after the runner
// is finished. Storage metrics are being recorded with each run.
func (s *storageTest) run(runner func(state *gethState.StateDB)) error {
	state, err := gethState.New(s.hash, s.stateDB, nil)
	if err != nil {
		return err
	}

	runner(state)

	s.hash, err = state.Commit(true)
	if err != nil {
		return err
	}

	err = state.Database().TrieDB().Commit(s.hash, true)
	if err != nil {
		return err
	}

	err = s.db.Commit(s.hash)
	if err != nil {
		return err
	}

	s.db.DropCache()

	s.metrics.add(bytesWrittenMetric, s.db.BytesStored())
	s.metrics.add(bytesReadMetric, s.db.BytesRetrieved())
	s.metrics.add(storageItemsMetric, s.store.TotalStorageItems())
	s.metrics.add(storageBytesMetric, s.store.TotalStorageSize())

	return nil
}

// metrics offers adding custom metrics as well as plotting the metrics on the provided x-axis
// as well as generating csv export for visualisation.
type metrics struct {
	data   map[string]int
	charts map[string][][2]int
}

func newMetrics() *metrics {
	return &metrics{
		data:   make(map[string]int),
		charts: make(map[string][][2]int),
	}
}

func (m *metrics) add(name string, value int) {
	m.data[name] = value
}

func (m *metrics) get(name string) int {
	return m.data[name]
}

func (m *metrics) plot(chartName string, x int, y int) {
	if _, ok := m.charts[chartName]; !ok {
		m.charts[chartName] = make([][2]int, 0)
	}
	m.charts[chartName] = append(m.charts[chartName], [2]int{x, y})
}

func (m *metrics) chartCSV(name string) string {
	c, ok := m.charts[name]
	if !ok {
		return ""
	}

	s := strings.Builder{}
	s.WriteString(name + "\n") // header
	for _, line := range c {
		s.WriteString(fmt.Sprintf("%d,%d\n", line[0], line[1]))
	}

	return s.String()
}

func Test_AccountCreations(t *testing.T) {
	if os.Getenv("benchmark") == "" {
		t.Skip("Skipping benchmarking")
	}

	tester, err := newStorageTest()
	require.NoError(t, err)

	accountChart := "accounts,storage_size"
	maxAccounts := 50_000
	for i := 0; i < maxAccounts; i++ {
		err = tester.run(func(state *gethState.StateDB) {
			state.AddBalance(tester.newAddress(), big.NewInt(100))
		})
		require.NoError(t, err)

		if i%50 == 0 { // plot with resolution
			tester.metrics.plot(accountChart, i, tester.metrics.get(storageBytesMetric))
		}
	}

	csv := tester.metrics.chartCSV(accountChart)
	err = io.WriteFile("./account_storage_size.csv", []byte(csv))
	require.NoError(t, err)
}

func Test_AccountContractInteraction(t *testing.T) {
	if os.Getenv("benchmark") == "" {
		t.Skip("Skipping benchmarking")
	}

	tester, err := newStorageTest()
	require.NoError(t, err)
	interactionChart := "interactions,storage_size_bytes"

	// build test contract storage state
	contractState := make(map[common.Hash]common.Hash)
	for i := 0; i < 10; i++ {
		h := common.HexToHash(fmt.Sprintf("%d", i))
		v := common.HexToHash(fmt.Sprintf("%d %s", i, make([]byte, 32)))
		contractState[h] = v
	}

	// build test contract code, aprox kitty contract size
	code := make([]byte, 50000)

	interactions := 50000
	for i := 0; i < interactions; i++ {
		err = tester.run(func(state *gethState.StateDB) {
			// create a new account
			accAddr := tester.newAddress()
			state.AddBalance(accAddr, big.NewInt(100))

			// create a contract
			contractAddr := tester.newAddress()
			state.SetBalance(contractAddr, big.NewInt(int64(i)))
			state.SetCode(contractAddr, code)
			state.SetStorage(contractAddr, contractState)

			// simulate interaction with contract state and account balance for fees
			state.SetState(contractAddr, common.HexToHash("0x03"), common.HexToHash("0x40"))
			state.AddBalance(accAddr, big.NewInt(1))
		})
		require.NoError(t, err)

		if i%50 == 0 { // plot with resolution
			tester.metrics.plot(interactionChart, i, tester.metrics.get(storageBytesMetric))
		}
	}

	csv := tester.metrics.chartCSV(interactionChart)
	err = io.WriteFile("./interactions_storage_size.csv", []byte(csv))
	require.NoError(t, err)
}
