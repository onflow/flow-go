package benchmark

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/sha3"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/common"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

// GENERAL COMMENT:
// running this test with
//   go test -bench=.  -benchmem
// will track the heap allocations for the Benchmarks

const (
	keyByteSize = 32
)

// Test_BenchmarkHashingWithAllocation benchmarks hashing where the
// Hasher structure is allocated and garbage collected for each hash
func Test_BenchmarkHashingWithAllocation(t *testing.T) {
	pairs := 100000
	keys := utils.GetRandomKeysFixedN(2*pairs, keyByteSize)

	var totalElapsed uint64 = 0
	var res []byte
	for i := 0; i < pairs; i++ {
		k1 := keys[2*i]
		k2 := keys[2*i+1]

		start := time.Now()
		res = common.HashInterNode(k1, k2)
		elapsed := time.Since(start)

		totalElapsed += uint64(elapsed)
	}
	fmt.Printf(hex.EncodeToString(res) + "\n")
	fmt.Printf("Average time per run [ns]: %f\n\n", float64(totalElapsed)/float64(pairs))
}

// Test_BenchmarkHashingWithConstantHasher benchmarks hashing where the
// Hasher structure is only allocated once and reused for all hashing operations.
// Note: the Hasher clears its internal state after each hash computation
// which still leads to heap allocations.
func Test_BenchmarkHashingWithConstantHasher(t *testing.T) {
	pairs := 100000
	keys := utils.GetRandomKeysFixedN(2*pairs, keyByteSize)

	hasher := sha3.New256()

	var totalElapsed uint64 = 0
	var res []byte
	for i := 0; i < pairs; i++ {
		k1 := keys[2*i]
		k2 := keys[2*i+1]
		res = make([]byte, 0, keyByteSize)

		start := time.Now()
		_, err := hasher.Write(k1)
		if err != nil {
			panic(err)
		}
		_, err = hasher.Write(k2)
		if err != nil {
			panic(err)
		}
		res = hasher.Sum(res)
		hasher.Reset()
		elapsed := time.Since(start)

		totalElapsed += uint64(elapsed)
	}
	fmt.Printf(hex.EncodeToString(res) + "\n")
	fmt.Printf("Average time per run [ns]: %f\n\n", float64(totalElapsed)/float64(pairs))
}

func Test_DefaultHash(t *testing.T) {
	res := GetDefaultHashForHeight(0)
	fmt.Printf(hex.EncodeToString(res) + "\n")

	res = GetDefaultHashForHeight(1)
	fmt.Printf(hex.EncodeToString(res) + "\n")

	var v uint32 = 1030549473

	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	fmt.Printf(hex.EncodeToString(b) + "\n")
}

// Benchmark_Hash benchmarks how many heap allocations result from
// computing a hash. Here we use a constant Hasher, i.e. the heap allocations
// are purely from the Hasher clearing its internal state.
func Benchmark_Hash(b *testing.B) {
	keys := utils.GetRandomKeysFixedN(2, keyByteSize)
	k1 := keys[0]
	k2 := keys[1]

	hasher := sha3.New256()

	var totalElapsed uint64 = 0
	var res []byte
	for i := 0; i < b.N; i++ {
		res = make([]byte, 0, keyByteSize)

		start := time.Now()
		_, err := hasher.Write(k1)
		if err != nil {
			panic(err)
		}
		_, err = hasher.Write(k2)
		if err != nil {
			panic(err)
		}
		res = hasher.Sum(res)
		hasher.Reset()
		elapsed := time.Since(start)

		totalElapsed += uint64(elapsed)
	}
	b.Log(">" + hex.EncodeToString(res) + "\n\n")
}

// Benchmark_SinleCycle benchmarks the time for a few CPU operations.
// Thereby, we can get an idea how expensive hashing is compared to conventional cheap checks etc.
func Benchmark_SinleCycle(t *testing.B) {
	var k uint64 = 0
	for i := 0; i < t.N; i++ {
		k += 1
	}
	fmt.Printf("%d\n\n", k)
}
