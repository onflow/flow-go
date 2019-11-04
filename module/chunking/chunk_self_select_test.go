// +build relic

package chunking

import (
	"fmt"
	"math"
	"testing"

	"github.com/dapperlabs/flow-go/crypto"
	exec "github.com/dapperlabs/flow-go/model/execution"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestSeedPrep(t *testing.T) {
	seed, _ := SeedPrep([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	if seed[0] != uint64(72623859790382856) || seed[1] != uint64(651345242494996240) {
		t.Error(fmt.Sprintf("Error with SeedPrep"))
	}
}
func TestChunkSelectAndVerify(t *testing.T) {
	var ChunkTotalGasSpent = []uint64{100, 200, 100, 200, 100, 200}
	var chunks []exec.Chunk
	for i, totalGas := range ChunkTotalGasSpent {
		chunks = append(chunks, exec.Chunk{Transactions: []flow.Transaction{flow.Transaction{Script: []byte("script"), ReferenceBlockHash: []byte("blockhash"), Nonce: uint64(i), ComputeLimit: uint64(1000)}}, TotalGasSpent: totalGas})
	}
	ratio := 0.5
	sk, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, []byte{1, 2, 3, 4})
	if err != nil {
		t.Error(fmt.Sprintf("%v Error creating keys", err))
	}
	er := exec.ExecutionResult{Chunks: chunks}
	selectedChunks, proof, err := ChunkSelfSelect(er, ratio, sk)
	if err != nil {
		t.Error("Error ChunkSelfSelect.")
	}
	// check the size of selected chunks
	if len(selectedChunks) != int(math.Ceil(float64(len(chunks))*ratio)) {
		t.Error(fmt.Sprintf("number selected chunks %v doesn't match ratio %v", len(selectedChunks), ratio))
	}
	// check for repetition
	has := make(map[*exec.Chunk]bool)
	for i := range selectedChunks {
		if _, ok := has[&selectedChunks[i]]; ok {
			t.Error(fmt.Sprintf("dupplicated chunk in selected chunks"))
		}
		has[&selectedChunks[i]] = true
	}

	// check the proofs
	verified, _ := VerifyChunkSelfSelect(er, ratio, sk.PublicKey(), selectedChunks, proof)
	if !verified {
		t.Error(fmt.Sprintf("Failed to verify the output of ChunkSelfSelect"))
	}
}
