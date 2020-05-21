package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
	ptriep "github.com/dapperlabs/flow-go/storage/ledger/ptrie"
	utils "github.com/dapperlabs/flow-go/storage/ledger/utils"
)

var dir = "./db/"

// StorageBenchmark benchmarks the performance of the storage layer
func StorageBenchmark() {
	// number of collections
	steps := 250 // 1000
	// assumption: 1000 key updates per collection
	numInsPerStep := 1000
	keyByteSize := 32
	valueMaxByteSize := 32
	trieHeight := keyByteSize*8 + 1 // 257

	absPath, err := filepath.Abs("./logs.txt")
	if err != nil {
		panic("can't creat log file")
	}
	fmt.Printf("Writing log file to '%s'\n", absPath)
	f, err := os.Create(absPath)
	if err != nil {
		panic("can't creat log file")
	}
	logger := zerolog.New(f).With().Time("time", time.Now()).Logger()

	rand.Seed(time.Now().UnixNano())

	metricsCollector := &metrics.NoopCollector{}
	led, err := ledger.NewMTrieStorage(dir, steps+1, metricsCollector, nil)
	defer func() {
		led.Done()
		os.RemoveAll(dir)
	}()
	if err != nil {
		panic("can't creat storage")
	}

	stateCommitment := led.EmptyStateCommitment()
	for i := 0; i < steps; i++ {
		fmt.Println("Step: ", i)

		keys := utils.GetRandomKeysFixedN(numInsPerStep, keyByteSize)
		values := utils.GetRandomValues(len(keys), valueMaxByteSize)

		start := time.Now()
		newState, err := led.UpdateRegisters(keys, values, stateCommitment)
		if err != nil {
			panic(err)
		}
		elapsed := time.Since(start)

		storageSize, _ := led.DiskSize()
		logger.Info().
			Int64("update_time_per_reg_ms", int64(elapsed/time.Millisecond)/int64(len(keys))).
			Int64("storage_size_mb", storageSize/int64(1000000)).
			Dur("update_time_ms", elapsed).
			Msg("update register")

		// read values and compare values
		start = time.Now()
		_, err = led.GetRegisters(keys, newState)
		if err != nil {
			panic("failed to update register")
		}
		elapsed = time.Since(start)

		logger.Info().
			Int64("read_time_per_reg_ms", int64(elapsed/time.Millisecond)/int64(len(keys))).
			Int("reg_count", len(keys)).
			Dur("read_time_ms", elapsed).
			Msg("read register")

		start = time.Now()
		// validate proofs (check individual proof and batch proof)
		retValues, proofs, err := led.GetRegistersWithProof(keys, newState)
		if err != nil {
			panic("failed to update register")
		}
		elapsed = time.Since(start)
		start = time.Now()
		// validate proofs as a batch
		_, err = ptriep.NewPSMT(newState, trieHeight, keys, retValues, proofs)
		if err != nil {
			panic("failed to create PSMT")
		}
		elapsed2 := time.Since(start)
		logger.Info().
			Int64("read_time_per_reg_ms", int64(elapsed/time.Millisecond)/int64(len(keys))).
			Int("reg_count", len(keys)).
			Dur("read_time_ms", elapsed).
			Dur("time_to_const_psmt_ms", elapsed2).
			Int64("time_to_const_psmt_per_reg_ms", int64(elapsed2/time.Millisecond)/int64(len(keys))).
			Msg("read register with proof")
		stateCommitment = newState

	}
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

// go run main.go -cpuprofile cpu.prof -memprofile mem.prof
func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}

	// goroutine
	StorageBenchmark()

}
