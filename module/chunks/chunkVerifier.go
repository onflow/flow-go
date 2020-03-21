package chunks

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
)

// ChunkVerifier is a verifier based on the current definitions of the flow network
type ChunkVerifier struct {
	vm        virtualmachine.VirtualMachine
	trieDepth int
}

// NewChunkVerifier creates a chunk verifier containing a flow virtual machine
func NewChunkVerifier(vm virtualmachine.VirtualMachine) *ChunkVerifier {
	return &ChunkVerifier{
		vm:        vm,
		trieDepth: 257,
	}
}

// Verify verifies the given VerifiableChunk by executing it and checking the final statecommitment
func (fcv *ChunkVerifier) Verify(ch *verification.VerifiableChunk) error {

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
		return fmt.Errorf("error constructing partial trie %w", err)
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
			// Delta captures all register updates and only
			// applies them if TX is successful.
			chunkView.ApplyDelta(txView.Delta())
		}
	}
	// TODO check the number of transactions and computation used

	// Apply delta (register updates (chunk level) to the partial trie
	// this returns the expected end state commitment after updates.

	regs, values := chunkView.Delta().RegisterUpdates()
	expEndStateComm, err := ptrie.Update(regs, values)
	if err != nil {
		return fmt.Errorf("error updating partial trie %v", err)

	}
	// Check if the end state commitment mentioned in the chunk matches
	// what the partial trie is providing.
	if !bytes.Equal(expEndStateComm, ch.EndState) {
		return fmt.Errorf("final state commitment doesn't match: [%x] != [%x]", expEndStateComm, ch.EndState)
	}
	return nil

}
