package main

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/sha3"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

const (
	keyByteSize = 32
)

func TestBenchmarkHash1(t *testing.T) {
	pairs := 100000
	keys := utils.GetRandomKeysFixedN(2*pairs, keyByteSize)

	var totalElapsed uint64 = 0
	var res []byte
	for i := 0; i < pairs; i++ {
		k1 := keys[2*i]
		k2 := keys[2*i+1]

		start := time.Now()
		res = mtrie.HashInterNode(k1, k2)
		elapsed := time.Since(start)

		totalElapsed += uint64(elapsed)
	}
	fmt.Printf(hex.EncodeToString(res) + "\n")
	fmt.Printf("Average time per run [ns]: %f\n", float64(totalElapsed)/float64(pairs))
}

func TestBenchmarkHash2(t *testing.T) {
	pairs := 100000
	keys := utils.GetRandomKeysFixedN(2*pairs, keyByteSize)

	hasher := sha3.New256()

	var totalElapsed uint64 = 0
	var res []byte
	for i := 0; i < pairs; i++ {
		k1 := keys[2*i]
		k2 := keys[2*i+1]

		start := time.Now()
		_, err := hasher.Write(k1)
		if err != nil {
			panic(err)
		}
		_, err = hasher.Write(k2)
		if err != nil {
			panic(err)
		}
		res := make([]byte, 0, keyByteSize)
		hasher.Sum(res)
		hasher.Reset()
		elapsed := time.Since(start)

		totalElapsed += uint64(elapsed)
	}
	fmt.Printf(hex.EncodeToString(res) + "\n")
	fmt.Printf("Average time per run [ns]: %f\n", float64(totalElapsed)/float64(pairs))
}

func BenchmarkHash3(t *testing.B) {
	keys := utils.GetRandomKeysFixedN(2, keyByteSize)
	k1 := keys[0]
	k2 := keys[1]

	hasher := sha3.New256()

	var totalElapsed uint64 = 0
	var res []byte
	for i := 0; i < t.N; i++ {

		start := time.Now()
		_, err := hasher.Write(k1)
		if err != nil {
			panic(err)
		}
		_, err = hasher.Write(k2)
		if err != nil {
			panic(err)
		}
		res := make([]byte, 0, keyByteSize)
		hasher.Sum(res)
		hasher.Reset()
		elapsed := time.Since(start)

		totalElapsed += uint64(elapsed)
	}
	fmt.Printf(hex.EncodeToString(res) + "\n")
}

func BenchmarkSinleCycle(t *testing.B) {
	var k uint64 = 0
	for i := 0; i < t.N; i++ {
		k += 1
	}
}
