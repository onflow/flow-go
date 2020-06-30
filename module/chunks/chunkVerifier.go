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
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/ptrie"
)

type VirtualMachine interface {
	Invoke(fvm.Context, fvm.Invokable, fvm.Ledger) (*fvm.InvocationResult, error)
}

// ChunkVerifier is a verifier based on the current definitions of the flow network
type ChunkVerifier struct {
	vm      VirtualMachine
	execCtx fvm.Context
}

// NewChunkVerifier creates a chunk verifier containing a flow virtual machine
func NewChunkVerifier(vm VirtualMachine, blocks storage.Blocks) *ChunkVerifier {
	execCtx := fvm.NewContext(fvm.WithBlocks(blocks))

	return &ChunkVerifier{
		vm:      vm,
		execCtx: execCtx,
	}
}

// Verify verifies the given VerifiableChunk by executing it and checking the final statecommitment
func (fcv *ChunkVerifier) Verify(vc *verification.VerifiableChunkData) (chmodels.ChunkFault, error) {

	// TODO check collection hash to match
	// TODO check datapack hash to match
	// TODO check the number of transactions and computation used

	chIndex := vc.Chunk.Index
	execResID := vc.Result.ID()

	// build a block context
	blockCtx := fvm.NewContextFromParent(fcv.execCtx, fvm.WithBlockHeader(vc.Header))

	// constructing a partial trie given chunk data package
	if vc.ChunkDataPack == nil {
		return nil, fmt.Errorf("missing chunk data pack")
	}
	psmt, err := ptrie.NewPSMT(vc.ChunkDataPack.StartState,
		ledger.RegisterKeySize,
		vc.ChunkDataPack.Registers(),
		vc.ChunkDataPack.Values(),
		vc.ChunkDataPack.Proofs(),
	)
	if err != nil {
		// TODO provide more details based on the error type
		return chmodels.NewCFInvalidVerifiableChunk("error constructing partial trie", err, chIndex, execResID), nil
	}

	// chunk view construction
	// unknown register tracks access to parts of the partial trie which
	// are not expanded and values are unknown.
	unknownRegTouch := make(map[string]bool)
	regMap := vc.ChunkDataPack.GetRegisterValues()
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
	for i, tx := range vc.Collection.Transactions {
		txView := chunkView.NewChild()

		result, err := fcv.vm.Invoke(blockCtx, fvm.Transaction(tx), txView)
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
	if !bytes.Equal(expEndStateComm, vc.EndState) {
		return chmodels.NewCFNonMatchingFinalState(expEndStateComm, vc.EndState, chIndex, execResID), nil
	}
	return nil, nil
}
