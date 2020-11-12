package chunks

import (
	"bytes"
	"errors"
	"fmt"

	executionState "github.com/onflow/flow-go/engine/execution/state"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/verification"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/partial"
	chmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.Ledger) error
}

// ChunkVerifier is a verifier based on the current definitions of the flow network
type ChunkVerifier struct {
	vm    VirtualMachine
	vmCtx fvm.Context
}

// NewChunkVerifier creates a chunk verifier containing a flow virtual machine
func NewChunkVerifier(vm VirtualMachine, vmCtx fvm.Context) *ChunkVerifier {
	return &ChunkVerifier{
		vm:    vm,
		vmCtx: vmCtx,
	}
}

// Verify verifies a given VerifiableChunk corresponding to a non-system chunk.
// by executing it and checking the final state commitment
// It returns a Spock Secret as a byte array, verification fault of the chunk, and an error.
// Note: Verify should only be executed on non-system chunks. It returns an error if it is invoked on
// system chunks.
func (fcv *ChunkVerifier) Verify(vc *verification.VerifiableChunkData) ([]byte, chmodels.ChunkFault, error) {
	if vc.IsSystemChunk {
		return nil, nil, fmt.Errorf("wrong method invoked for verifying system chunk")
	}

	transactions := make([]*fvm.TransactionProcedure, 0)
	for _, txBody := range vc.Collection.Transactions {
		tx := fvm.Transaction(txBody)
		transactions = append(transactions, tx)
	}

	return fcv.verifyTransactions(vc.Chunk, vc.ChunkDataPack, vc.Collection, vc.Result, vc.Header, transactions)
}

// SystemChunkVerify verifies a given VerifiableChunk corresponding to a system chunk.
// by executing it and checking the final state commitment
// It returns a Spock Secret as a byte array, verification fault of the chunk, and an error.
// Note: SystemChunkVerify should only be executed on system chunks. It returns an error if it is invoked on
// non-system chunks.
func (fcv *ChunkVerifier) SystemChunkVerify(vc *verification.VerifiableChunkData) ([]byte, chmodels.ChunkFault, error) {
	if !vc.IsSystemChunk {
		return nil, nil, fmt.Errorf("wrong method invoked for verifying non-system chunk")
	}

	// transaction body of system chunk
	txBody := fvm.SystemChunkTransaction(fcv.vmCtx.Chain.ServiceAddress())
	tx := fvm.Transaction(txBody)
	transactions := []*fvm.TransactionProcedure{tx}

	return fcv.verifyTransactions(vc.Chunk, vc.ChunkDataPack, vc.Collection, vc.Result, vc.Header, transactions)
}

func (fcv *ChunkVerifier) verifyTransactions(chunk *flow.Chunk,
	chunkDataPack *flow.ChunkDataPack,
	collection *flow.Collection,
	result *flow.ExecutionResult,
	header *flow.Header,
	transactions []*fvm.TransactionProcedure) ([]byte, chmodels.ChunkFault, error) {

	if chunkDataPack == nil {
		return nil, nil, fmt.Errorf("missing chunk data pack")
	}

	chIndex := chunk.Index
	execResID := result.ID()

	// check that the ChunkDataPack refers to the correct Chunk
	if chunkDataPack.ChunkID != chunk.ID() {
		return nil,
			chmodels.NewCFInvalidVerifiableChunk(
				"invalid Chunk ID",
				fmt.Errorf("ChunkDataPack.ChunkID (%s) does not match the provided Chunk (%s)", chunkDataPack.ChunkID, chunk.ID()),
				chIndex,
				execResID),
			nil
	}

	// check that the ChunkDataPack refers to the correct Collection
	if collection.ID() != chunkDataPack.CollectionID {
		// NOTE: the execution nodes set the CollectionID of the system chunk's
		// DataPack to Zero (cf. engin/execution/ingestion/ingestion.go saveExecutionResult).
		// But Zero is not the natural ID for an empty collection, as computed by
		// the ID function (cf. model/flow/collection.go). So we have to account for
		// this special case.
		if !(chunkDataPack.CollectionID == flow.ZeroID && collection.Len() == 0) {
			return nil,
				chmodels.NewCFInvalidVerifiableChunk(
					"invalid Collection ID",
					fmt.Errorf("ChunkDataPack.CollectionID (%s) does not match the provided Collection (%s)", chunkDataPack.CollectionID, collection.ID()),
					chIndex,
					execResID),
				nil
		}
	}

	// TODO:
	// * Check number of transactions
	// * Check gas used
	// This is dependant on updating the execution nodes to set these values

	// build a block context
	blockCtx := fvm.NewContextFromParent(fcv.vmCtx, fvm.WithBlockHeader(header))

	// constructing a partial trie given chunk data package
	psmt, err := partial.NewLedger(chunkDataPack.Proof, chunk.StartState, partial.DefaultPathFinderVersion)

	if err != nil {
		// TODO provide more details based on the error type
		return nil, chmodels.NewCFInvalidVerifiableChunk("error constructing partial trie: ", err, chIndex, execResID),
			nil
	}

	// chunk view construction
	// unknown register tracks access to parts of the partial trie which
	// are not expanded and values are unknown.
	unknownRegTouch := make(map[string]*ledger.Key)
	getRegister := func(owner, controller, key string) (flow.RegisterValue, error) {
		// check if register has been provided in the chunk data pack
		registerID := flow.NewRegisterID(owner, controller, key)

		registerKey := executionState.RegisterIDToKey(registerID)

		query, err := ledger.NewQuery(chunk.StartState, []ledger.Key{registerKey})

		if err != nil {
			return nil, fmt.Errorf("cannot create query: %w", err)
		}

		values, err := psmt.Get(query)
		if err != nil {
			if errors.Is(err, ledger.ErrMissingKeys{}) {

				unknownRegTouch[registerID.String()] = &registerKey
				return nil, fmt.Errorf("missing register")
			}
			// append to missing keys if error is ErrMissingKeys

			return nil, fmt.Errorf("cannot query register: %w", err)
		}

		return values[0], nil
	}

	chunkView := delta.NewView(getRegister)

	// executes all transactions in this chunk
	for i, tx := range transactions {
		txView := chunkView.NewChild()

		err := fcv.vm.Run(blockCtx, tx, txView)
		if err != nil {
			// this covers unexpected and very rare cases (e.g. system memory issues...),
			// so we shouldn't be here even if transaction naturally fails (e.g. permission, runtime ... )
			return nil, nil, fmt.Errorf("failed to execute transaction: %d (%w)", i, err)
		}

		if tx.Err == nil {
			// if tx is successful, we apply changes to the chunk view by merging the txView into chunk view
			chunkView.MergeView(txView)
		}
	}

	// check read access to unknown registers
	if len(unknownRegTouch) > 0 {
		var missingRegs []string
		for _, key := range unknownRegTouch {
			missingRegs = append(missingRegs, key.String())
		}
		return nil, chmodels.NewCFMissingRegisterTouch(missingRegs, chIndex, execResID), nil
	}

	// applying chunk delta (register updates at chunk level) to the partial trie
	// this returns the expected end state commitment after updates and the list of
	// register keys that was not provided by the chunk data package (err).
	regs, values := chunkView.Delta().RegisterUpdates()

	update, err := ledger.NewUpdate(
		chunk.StartState,
		executionState.RegisterIDSToKeys(regs),
		executionState.RegisterValuesToValues(values),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create ledger update: %w", err)
	}

	expEndStateComm, err := psmt.Set(update)

	if err != nil {
		if errors.Is(err, ledger.ErrMissingKeys{}) {
			keys := err.(*ledger.ErrMissingKeys).Keys
			stringKeys := make([]string, len(keys))
			for i, key := range keys {
				stringKeys[i] = key.String()
			}
			return nil, chmodels.NewCFMissingRegisterTouch(stringKeys, chIndex, execResID), nil
		}
		return nil, chmodels.NewCFMissingRegisterTouch(nil, chIndex, execResID), nil
	}

	// TODO check if exec node provided register touches that was not used (no read and no update)
	// check if the end state commitment mentioned in the chunk matches
	// what the partial trie is providing.
	if !bytes.Equal(expEndStateComm, chunk.EndState) {
		return nil, chmodels.NewCFNonMatchingFinalState(expEndStateComm, chunk.EndState, chIndex, execResID), nil
	}
	return chunkView.SpockSecret(), nil, nil
}
