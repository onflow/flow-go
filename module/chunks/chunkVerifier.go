package chunks

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/fvm"
	chmodels "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/ledger/ptrie"
)

// ChunkVerifier is a verifier based on the current definitions of the flow network
type ChunkVerifier struct {
	vm        fvm.VirtualMachine
	trieDepth int
	blocks    storage.Blocks
}

// NewChunkVerifier creates a chunk verifier containing a flow virtual machine
func NewChunkVerifier(vm fvm.VirtualMachine, blocks storage.Blocks) *ChunkVerifier {
	return &ChunkVerifier{
		vm:        vm,
		trieDepth: 257,
		blocks:    blocks,
	}
}

// Verify verifies the given VerifiableChunk by executing it and checking the final statecommitment
func (fcv *ChunkVerifier) Verify(ch *verification.VerifiableChunk) (chmodels.ChunkFault, error) {

	// TODO check collection hash to match
	// TODO check datapack hash to match
	// TODO check the number of transactions and computation used

	chIndex := ch.ChunkIndex

	if ch.Receipt == nil {
		return nil, fmt.Errorf("missing execution receipt")
	}
	execResID := ch.Receipt.ExecutionResult.ID()

	// build a block context
	if ch.Block == nil {
		return nil, fmt.Errorf("missing block")
	}
	blockCtx := fcv.vm.NewBlockContext(ch.Block.Header, fcv.blocks)

	// constructing a partial trie given chunk data package
	if ch.ChunkDataPack == nil {
		return nil, fmt.Errorf("missing chunk data pack")
	}
	psmt, err := ptrie.NewPSMT(ch.ChunkDataPack.StartState,
		fcv.trieDepth,
		ch.ChunkDataPack.Registers(),
		ch.ChunkDataPack.Values(),
		ch.ChunkDataPack.Proofs(),
	)
	if err != nil {
		// TODO provide more details based on the error type
		return chmodels.NewCFInvalidVerifiableChunk("error constructing partial trie", err, chIndex, execResID), nil
	}

	// chunk view construction
	// unknown register tracks access to parts of the partial trie which
	// are not expanded and values are unknown.
	unknownRegTouch := make(map[string]bool)
	regMap := ch.ChunkDataPack.GetRegisterValues()
	getRegister := func(key flow.RegisterID) (flow.RegisterValue, error) {
		// check if register has been provided in the chunk data pack
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
			// this covers unexpected and very rare cases (e.g. system memory issues...),
			// so we shouldn't be here even if transaction naturally fails (e.g. permission, runtime ... )
			return nil, fmt.Errorf("failed to execute transaction: %d (%w)", i, err)
		}
		if result.Succeeded() {
			// if tx is successful, we apply changes to the chunk view by merging the txView into chunk view
			chunkView.MergeView(txView)
		}
	}

	// check read access to unknown registers
	if len(unknownRegTouch) > 0 {
		var missingRegs []string
		for key := range unknownRegTouch {
			missingRegs = append(missingRegs, key)
		}
		return chmodels.NewCFMissingRegisterTouch(missingRegs, chIndex, execResID), nil
	}

	// applying chunk delta (register updates at chunk level) to the partial trie
	// this returns the expected end state commitment after updates and the list of
	// register keys that was not provided by the chunk data package (err).
	regs, values := chunkView.Delta().RegisterUpdates()
	expEndStateComm, failedKeys, err := psmt.Update(regs, values)
	if err != nil {
		return chmodels.NewCFMissingRegisterTouch(failedKeys, chIndex, execResID), nil
	}

	// TODO check if exec node provided register touches that was not used (no read and no update)

	// check if the end state commitment mentioned in the chunk matches
	// what the partial trie is providing.
	if !bytes.Equal(expEndStateComm, ch.EndState) {
		return chmodels.NewCFNonMatchingFinalState(expEndStateComm, ch.EndState, chIndex, execResID), nil
	}
	return nil, nil
}
