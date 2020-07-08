package benchmark

// func TestFullBenchamark(t *testing.T) {
// 	t.Skipf("manual debug only")
// 	StorageBenchmark()
// }

import (
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/ptrie"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

func BenchmarkStorage(b *testing.B) { benchmarkStorage(100, b) }

// BenchmarkStorage benchmarks the performance of the storage layer
func benchmarkStorage(steps int, b *testing.B) {
	// assumption: 1000 key updates per collection
	numInsPerStep := 1000
	keyByteSize := 32
	valueMaxByteSize := 32
	rand.Seed(time.Now().UnixNano())

	dir, err := ioutil.TempDir("", "test-mtrie-")
	defer os.RemoveAll(dir)
	if err != nil {
		b.Fatal(err)
	}

	led, err := ledger.NewMTrieStorage(dir, steps+1, &metrics.NoopCollector{}, nil)
	defer led.Done()
	if err != nil {
		b.Fatal("can't create MTrieStorage")
	}
	totalUpdateTimeMS := 0
	totalReadTimeMS := 0
	totalProofTimeMS := 0
	totalRegOperation := 0
	totalProofSize := 0
	totalPTrieConstTimeMS := 0

	stateCommitment := led.EmptyStateCommitment()
	for i := 0; i < steps; i++ {
		keys := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
		values := utils.GetRandomValues(len(keys), 0, valueMaxByteSize)
		totalRegOperation += len(keys)

		start := time.Now()
		newState, err := led.UpdateRegisters(keys, values, stateCommitment)
		if err != nil {
			panic(err)
		}
		elapsed := time.Since(start)
		totalUpdateTimeMS += int(elapsed / time.Millisecond)

		// read values and compare values
		start = time.Now()
		_, err = led.GetRegisters(keys, newState)
		if err != nil {
			panic("failed to update register")
		}
		elapsed = time.Since(start)
		totalReadTimeMS += int(elapsed / time.Millisecond)

		start = time.Now()
		// validate proofs (check individual proof and batch proof)
		retValues, proofs, err := led.GetRegistersWithProof(keys, newState)
		if err != nil {
			panic("failed to update register")
		}
		elapsed = time.Since(start)
		totalProofTimeMS += int(elapsed / time.Millisecond)

		byteSize := 0
		for _, p := range proofs {
			byteSize += len(p)
		}
		totalProofSize += byteSize

		start = time.Now()
		// construct a partial trie using proofs
		_, err = ptrie.NewPSMT(newState, keyByteSize, keys, retValues, proofs)
		if err != nil {
			b.Fatal("failed to create PSMT")
		}
		elapsed = time.Since(start)
		totalPTrieConstTimeMS += int(elapsed / time.Millisecond)

		stateCommitment = newState
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
