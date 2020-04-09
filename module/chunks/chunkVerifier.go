package chunks

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/engine/verification"
	chmodels "github.com/dapperlabs/flow-go/model/chunks"
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
func (fcv *ChunkVerifier) Verify(ch *verification.VerifiableChunk) (chmodels.ChunkFault, error) {

	// TODO check collection hash to match
	// TODO check datapack hash to match
	// TODO check the number of transactions and computation used

	if ch.ChunkDataPack == nil {
		return nil, fmt.Errorf("missing chunk data pack")
	}
	if ch.Block == nil {
		return nil, fmt.Errorf("missing block")
	}

	chIndex := ch.ChunkIndex
	execResID := ch.Receipt.ExecutionResult.ID()
	blockCtx := fcv.vm.NewBlockContext(&ch.Block.Header)
	ptrie, err := trie.NewPSMT(ch.ChunkDataPack.StartState,
		fcv.trieDepth,
		ch.ChunkDataPack.Registers(),
		ch.ChunkDataPack.Values(),
		ch.ChunkDataPack.Proofs(),
	)
	if err != nil {
		// TODO more analysis on the error reason
		return chmodels.NewCFInvalidVerifiableChunk("error constructing partial trie", err, chIndex, execResID), nil
	}

	regMap := ch.ChunkDataPack.GetRegisterValues()
	unknownRegTouch := make(map[string]bool)
	getRegister := func(key flow.RegisterID) (flow.RegisterValue, error) {
		val, ok := regMap[string(key)]
		if !ok {
			unknownRegTouch[string(key)] = true
			return nil, fmt.Errorf("missing register")
		}
		return val, nil
	}

	chunkView := delta.NewView(getRegister)

	// executes all transactions in this chunk
	for i, tx := range ch.Collection.Transactions {
		txView := chunkView.NewChild()

		result, err := blockCtx.ExecuteTransaction(txView, tx)
		if err != nil {
			// TODO: at this point we don't care about the tx errors,
			// we might have to actually care about this later
			return nil, fmt.Errorf("failed to execute transaction: %d (%w)", i, err)
		}
		if result.Succeeded() {
			// Delta captures all register updates and only
			// applies them if TX is successful.
			chunkView.MergeView(txView)
		}
	}

	// check read access to register touches
	if len(unknownRegTouch) > 0 {
		var missingRegs []string
		for key := range unknownRegTouch {
			missingRegs = append(missingRegs, key)
		}
		return chmodels.NewCFMissingRegisterTouch(missingRegs, chIndex, execResID), nil
	}

	// applying chunk delta (register updates at chunk level) to the partial trie
	// this returns the expected end state commitment after updates.
	regs, values := chunkView.Delta().RegisterUpdates()
	expEndStateComm, failedKeys, err := ptrie.Update(regs, values)
	if err != nil {
		return chmodels.NewCFMissingRegisterTouch(failedKeys, chIndex, execResID), nil
	}

	// check if the end state commitment mentioned in the chunk matches
	// what the partial trie is providing.
	if !bytes.Equal(expEndStateComm, ch.EndState) {
		return chmodels.NewCFNonMatchingFinalState(expEndStateComm, ch.EndState, chIndex, execResID), nil
	}
	return nil, nil
}
