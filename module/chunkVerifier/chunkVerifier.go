package chunkVerifier

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
)

// ChunkVerifier provides functionality to verify chunks
type ChunkVerifier interface {
	// Verify verifies the given VerifiableChunk by executing it and checking the final statecommitment
	// TODO return challenges beside errors
	Verify(ch *verification.VerifiableChunk) error
}

// FlowChunkVerifier is a verifier based on the current definitions of the flow network
type FlowChunkVerifier struct {
	vm        virtualmachine.VirtualMachine
	trieDepth int
}

// NewFlowChunkVerifier creates a chunk verifier containing a flow virtual machine
func NewFlowChunkVerifier() *FlowChunkVerifier {
	rt := runtime.NewInterpreterRuntime()
	return &FlowChunkVerifier{
		vm:        virtualmachine.New(rt),
		trieDepth: 257,
	}
}

// Verify verifies the given VerifiableChunk by executing it and checking the final statecommitment
func (fcv *FlowChunkVerifier) Verify(ch *verification.VerifiableChunk) error {

	// TODO check collection hash to match
	if ch.ChunkDataPack == nil {
		return fmt.Errorf("chunk data pack is empty")
	}
	blockCtx := fcv.vm.NewBlockContext(&ch.Block.Header)

	ptrie, err := trie.NewPSMT(ch.ChunkDataPack.StartState,
		fcv.trieDepth,
		ch.ChunkDataPack.Registers(),
		ch.ChunkDataPack.Values(),
		ch.ChunkDataPack.Proofs(),
	)
	if err != nil {
		return fmt.Errorf("error constructing partial trie %x", err)
	}

	regMap := ch.ChunkDataPack.GetRegisterValues()
	getRegister := func(key flow.RegisterID) (flow.RegisterValue, error) {
		val, ok := regMap[string(key)]
		if !ok {
			return nil, fmt.Errorf("missing register")
		}
		return val, nil
	}

	chunkView := state.NewView(getRegister)

	// executes all transactions in this chunk
	for i, tx := range ch.Collection.Transactions {
		txView := chunkView.NewChild()

		result, err := blockCtx.ExecuteTransaction(txView, tx)
		if err != nil {
			return fmt.Errorf("failed to execute transaction: %d (%w)", i, err)
		}

		if result.Succeeded() {
			chunkView.ApplyDelta(txView.Delta())
		}
	}
	// TODO check the number of transactions and computation used

	if err != nil {
		return fmt.Errorf("failed to execute transactions: %w", err)
	}

	// Apply delta to ptrie
	regs, values := chunkView.Delta().RegisterUpdates()
	expectedEndState, err := ptrie.Update(regs, values)
	if err != nil {
		return fmt.Errorf("error updating partial trie %v", err)

	}
	// Check state commitment
	if !bytes.Equal(expectedEndState, ch.EndState) {
		return fmt.Errorf("final state commitment doesn't match: [%x] != [%x]", ptrie.GetRootHash(), ch.EndState)
	}
	return nil

}
