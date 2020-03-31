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
	// TODO check the number of transactions and computation used

	if ch.ChunkDataPack == nil {
		return ErrIncompleteVerifiableChunk{[]string{"chunk data pack is empty"}}
	}
	if ch.Block == nil {
		return ErrIncompleteVerifiableChunk{[]string{"block is empty"}}
	}

	blockCtx := fcv.vm.NewBlockContext(&ch.Block.Header)

	ptrie, err := trie.NewPSMT(ch.ChunkDataPack.StartState,
		fcv.trieDepth,
		ch.ChunkDataPack.Registers(),
		ch.ChunkDataPack.Values(),
		ch.ChunkDataPack.Proofs(),
	)
	if err != nil {
		return ErrInvalidVerifiableChunk{"error constructing partial trie", err}
	}

	regMap := ch.ChunkDataPack.GetRegisterValues()
	unAuthRegTouch := make(map[string]bool)
	getRegister := func(key flow.RegisterID) (flow.RegisterValue, error) {
		val, ok := regMap[string(key)]
		if !ok {
			unAuthRegTouch[string(key)] = true
			return nil, fmt.Errorf("missing register")
		}
		return val, nil
	}

	chunkView := state.NewView(getRegister)

	// executes all transactions in this chunk
	for _, tx := range ch.Collection.Transactions {
		txView := chunkView.NewChild()

		result, err := blockCtx.ExecuteTransaction(txView, tx)
		if err != nil {
			// TODO: at this point we don't care about the tx errors,
			// we might have to actually care about this later
			// return fmt.Errorf("failed to execute transaction: %d (%w)", i, err)
			continue
		}
		if result.Succeeded() {
			// Delta captures all register updates and only
			// applies them if TX is successful.
			chunkView.ApplyDelta(txView.Delta())
		}
	}

	// check read access to register touches
	if len(unAuthRegTouch) > 0 {
		var missingRegs []string
		for key := range unAuthRegTouch {
			missingRegs = append(missingRegs, key)
		}
		return ErrMissingRegisterTouch{missingRegs}
	}

	// applying chunk delta (register updates at chunk level) to the partial trie
	// this returns the expected end state commitment after updates.
	regs, values := chunkView.Delta().RegisterUpdates()
	expEndStateComm, failedKeys, err := ptrie.Update(regs, values)
	if err != nil {
		return ErrMissingRegisterTouch{failedKeys}
	}

	// check if the end state commitment mentioned in the chunk matches
	// what the partial trie is providing.
	if !bytes.Equal(expEndStateComm, ch.EndState) {
		return ErrNonMatchingFinalState{expEndStateComm, ch.EndState}
	}
	return nil

}
