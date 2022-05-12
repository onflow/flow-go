package complete_test

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/ledger/partial/ptrie"
	"github.com/onflow/flow-go/module/metrics"
)

// GENERAL COMMENT:
// running this test with
//   go test -bench=.  -benchmem
// will track the heap allocations for the Benchmarks
func BenchmarkStorage(b *testing.B) { benchmarkStorage(100, b) }

// BenchmarkStorage benchmarks the performance of the storage layer
func benchmarkStorage(steps int, b *testing.B) {
	// assumption: 1000 key updates per collection
	numInsPerStep := 1000
	keyNumberOfParts := 10
	keyPartMinByteSize := 1
	keyPartMaxByteSize := 100
	valueMaxByteSize := 32
	rand.Seed(time.Now().UnixNano())

	dir, err := os.MkdirTemp("", "test-mtrie-")
	defer os.RemoveAll(dir)
	if err != nil {
		b.Fatal(err)
	}

	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, steps+1, pathfinder.PathByteSize, wal.SegmentSize)
	require.NoError(b, err)
	defer func() {
		<-diskWal.Done()
	}()

	led, err := complete.NewLedger(diskWal, steps+1, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	defer led.Done()
	if err != nil {
		b.Fatal("can't create a new complete ledger")
	}
	totalUpdateTimeMS := 0
	totalReadTimeMS := 0
	totalProofTimeMS := 0
	totalRegOperation := 0
	totalProofSize := 0
	totalPTrieConstTimeMS := 0

	state := led.InitialState()
	for i := 0; i < steps; i++ {

		keys := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
		values := utils.RandomValues(numInsPerStep, 1, valueMaxByteSize)

		totalRegOperation += len(keys)

		start := time.Now()
		update, err := ledger.NewUpdate(state, keys, values)
		if err != nil {
			b.Fatal(err)
		}

		newState, _, err := led.Set(update)
		if err != nil {
			b.Fatal(err)
		}

		elapsed := time.Since(start)
		totalUpdateTimeMS += int(elapsed / time.Millisecond)

		// read values and compare values
		start = time.Now()
		query, err := ledger.NewQuery(newState, keys)
		if err != nil {
			b.Fatal(err)
		}
		_, err = led.Get(query)
		if err != nil {
			b.Fatal(err)
		}
		elapsed = time.Since(start)
		totalReadTimeMS += int(elapsed / time.Millisecond)

		start = time.Now()
		// validate proofs (check individual proof and batch proof)
		proof, err := led.Prove(query)
		if err != nil {
			b.Fatal(err)
		}
		elapsed = time.Since(start)
		totalProofTimeMS += int(elapsed / time.Millisecond)

		totalProofSize += len(proof)

		start = time.Now()
		p, _ := encoding.DecodeTrieBatchProof(proof)

		// construct a partial trie using proofs
		_, err = ptrie.NewPSMT(ledger.RootHash(newState), p)
		if err != nil {
			b.Fatal("failed to create PSMT")
		}
		elapsed = time.Since(start)
		totalPTrieConstTimeMS += int(elapsed / time.Millisecond)

		state = newState
	}

	b.ReportMetric(float64(totalUpdateTimeMS/steps), "update_time_(ms)")
	b.ReportMetric(float64(totalUpdateTimeMS*1000000/totalRegOperation), "update_time_per_reg_(ns)")

	b.ReportMetric(float64(totalReadTimeMS/steps), "read_time_(ms)")
	b.ReportMetric(float64(totalReadTimeMS*1000000/totalRegOperation), "read_time_per_reg_(ns)")

	b.ReportMetric(float64(totalProofTimeMS/steps), "read_w_proof_time_(ms)")
	b.ReportMetric(float64(totalProofTimeMS*1000000/totalRegOperation), "read_w_proof_time_per_reg_(ns)")

	b.ReportMetric(float64(totalProofSize/steps), "proof_size_(MB)")
	b.ReportMetric(float64(totalPTrieConstTimeMS/steps), "ptrie_const_time_(ms)")

}

// BenchmarkTrieUpdate benchmarks the performance of a trie update
func BenchmarkTrieUpdate(b *testing.B) {
	// key updates per iteration
	numInsPerStep := 10000
	keyNumberOfParts := 10
	keyPartMinByteSize := 1
	keyPartMaxByteSize := 100
	valueMaxByteSize := 32
	rand.Seed(1)

	dir, err := os.MkdirTemp("", "test-mtrie-")
	defer os.RemoveAll(dir)
	if err != nil {
		b.Fatal(err)
	}

	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, 101, pathfinder.PathByteSize, wal.SegmentSize)
	require.NoError(b, err)
	defer func() {
		<-diskWal.Done()
	}()

	led, err := complete.NewLedger(diskWal, 101, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	defer led.Done()
	if err != nil {
		b.Fatal("can't create a new complete ledger")
	}

	state := led.InitialState()

	keys := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
	values := utils.RandomValues(numInsPerStep, 1, valueMaxByteSize)

	update, err := ledger.NewUpdate(state, keys, values)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := led.Set(update)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
}

// BenchmarkTrieUpdate benchmarks the performance of a trie read
func BenchmarkTrieRead(b *testing.B) {
	// key updates per iteration
	numInsPerStep := 10000
	keyNumberOfParts := 10
	keyPartMinByteSize := 1
	keyPartMaxByteSize := 100
	valueMaxByteSize := 32
	rand.Seed(1)

	dir, err := os.MkdirTemp("", "test-mtrie-")
	defer os.RemoveAll(dir)
	if err != nil {
		b.Fatal(err)
	}

	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, 101, pathfinder.PathByteSize, wal.SegmentSize)
	require.NoError(b, err)
	defer func() {
		<-diskWal.Done()
	}()

	led, err := complete.NewLedger(diskWal, 101, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	defer led.Done()
	if err != nil {
		b.Fatal("can't create a new complete ledger")
	}

	state := led.InitialState()

	keys := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
	values := utils.RandomValues(numInsPerStep, 1, valueMaxByteSize)

	update, err := ledger.NewUpdate(state, keys, values)
	if err != nil {
		b.Fatal(err)
	}

	newState, _, err := led.Set(update)
	if err != nil {
		b.Fatal(err)
	}

	query, err := ledger.NewQuery(newState, keys)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = led.Get(query)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
}

// BenchmarkTrieUpdate benchmarks the performance of a trie prove
func BenchmarkTrieProve(b *testing.B) {
	// key updates per iteration
	numInsPerStep := 10000
	keyNumberOfParts := 10
	keyPartMinByteSize := 1
	keyPartMaxByteSize := 100
	valueMaxByteSize := 32
	rand.Seed(1)

	dir, err := os.MkdirTemp("", "test-mtrie-")
	defer os.RemoveAll(dir)
	if err != nil {
		b.Fatal(err)
	}

	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metrics.NewNoopCollector(), dir, 101, pathfinder.PathByteSize, wal.SegmentSize)
	require.NoError(b, err)
	defer func() {
		<-diskWal.Done()
	}()

	led, err := complete.NewLedger(diskWal, 101, &metrics.NoopCollector{}, zerolog.Logger{}, complete.DefaultPathFinderVersion)
	defer led.Done()
	if err != nil {
		b.Fatal("can't create a new complete ledger")
	}

	state := led.InitialState()

	keys := utils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
	values := utils.RandomValues(numInsPerStep, 1, valueMaxByteSize)

	update, err := ledger.NewUpdate(state, keys, values)
	if err != nil {
		b.Fatal(err)
	}

	newState, _, err := led.Set(update)
	if err != nil {
		b.Fatal(err)
	}

	query, err := ledger.NewQuery(newState, keys)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := led.Prove(query)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
}
