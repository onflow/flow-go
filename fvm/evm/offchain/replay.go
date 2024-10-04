package sync

import (
	"bytes"
	"fmt"

	"github.com/onflow/atree"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethTracer "github.com/onflow/go-ethereum/eth/tracers"
	gethTrie "github.com/onflow/go-ethereum/trie"

	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

var emptyChecksum = [types.ChecksumLength]byte{0, 0, 0, 0}

type ReplayResults interface {
	StorageRegisterUpdates() map[flow.RegisterID]flow.RegisterValue
}

// ReplayBlockExecution re-executes transactions of a block using the
// events emitted when transactions where executed.
// it updates the state of the given ledger and uses the trace
func ReplayBlockExecution(
	chainID flow.ChainID,
	snapshot BackendStorageSnapshot,
	tracer *gethTracer.Tracer,
	transactionEvents []events.TransactionEventPayload,
	blockEvent *events.BlockEventPayload,
	validateResults bool,
) (ReplayResults, error) {

	// create storage
	storage := NewEphemeralStorage(NewReadOnlyStorage(snapshot))

	// prepare blocks
	blocks, err := NewBlocks(chainID, storage)
	if err != nil {
		return nil, err
	}
	// push the new block meta
	// it should be done before execution so block context creation
	// can be done properly
	err = blocks.PushBlockMeta(
		NewBlockMeta(
			blockEvent.Height,
			blockEvent.Timestamp,
			blockEvent.PrevRandao,
		),
	)
	if err != nil {
		return nil, err
	}

	// create a base block context for all transactions
	// tx related context values will be replaced during execution
	ctx, err := CreateBlockContext(chainID, blocks, tracer)
	if err != nil {
		return nil, err
	}

	gasConsumedSoFar := uint64(0)
	txHashes := make(types.TransactionHashes, len(transactionEvents))
	for idx, tx := range transactionEvents {
		err = replayTransactionExecution(
			chainID,
			ctx,
			uint(idx),
			gasConsumedSoFar,
			storage,
			&tx,
			validateResults,
		)
		if err != nil {
			return nil, err
		}
		gasConsumedSoFar += tx.GasConsumed
		txHashes[idx] = tx.Hash
	}

	if validateResults {
		// check transaction inclusion
		txHashRoot := gethTypes.DeriveSha(txHashes, gethTrie.NewStackTrie(nil))
		if txHashRoot != blockEvent.TransactionHashRoot {
			return nil, fmt.Errorf("transaction root hash doesn't match [%x] != [%x]", txHashRoot, blockEvent.TransactionHashRoot)
		}

		// check total gas used
		if blockEvent.TotalGasUsed != gasConsumedSoFar {
			return nil, fmt.Errorf("total gas used doesn't match [%x] != [%x]", txHashRoot, blockEvent.TransactionHashRoot)
		}
		// no need to check the receipt root hash given we have checked the logs and other
		// values during tx execution.
	}
	// push block hash
	// we push the block hash after execution, so the behaviour of the blockhash is
	// identical to the evm.handler.
	err = blocks.PushBlockHash(
		blockEvent.Height,
		blockEvent.Hash,
	)
	if err != nil {
		return nil, err
	}

	return storage, nil
}

func replayTransactionExecution(
	chainID flow.ChainID,
	ctx types.BlockContext,
	txIndex uint,
	gasUsedSoFar uint64,
	ledger atree.Ledger,
	txEvent *events.TransactionEventPayload,
	validate bool,
) error {

	// create emulator
	em := emulator.NewEmulator(ledger, evm.StorageAccountAddress(chainID))

	// update block context with tx level info
	ctx.TotalGasUsedSoFar = gasUsedSoFar
	ctx.TxCountSoFar = txIndex
	// populate precompiled calls
	if len(txEvent.PrecompiledCalls) > 0 {
		pcs, err := types.AggregatedPrecompileCallsFromEncoded(txEvent.PrecompiledCalls)
		if err != nil {
			return fmt.Errorf("error decoding precompiled calls [%x]: %w", txEvent.Payload, err)
		}
		ctx.ExtraPrecompiledContracts = precompiles.AggregatedPrecompiledCallsToPrecompiledContracts(pcs)
	}

	// create a new block view
	bv, err := em.NewBlockView(ctx)
	if err != nil {
		return err
	}

	var res *types.Result
	// check if the transaction payload is actually from a direct call,
	// which is a special state transition in Flow EVM.
	if txEvent.TransactionType == types.DirectCallTxType {
		call, err := types.DirectCallFromEncoded(txEvent.Payload)
		if err != nil {
			return fmt.Errorf("failed to RLP-decode direct call [%x]: %w", txEvent.Payload, err)
		}

		res, err = bv.DirectCall(call)
		if err != nil {
			return fmt.Errorf("failed to execute direct call [%x]: %w", txEvent.Hash, err)
		}
	} else {
		gethTx := &gethTypes.Transaction{}
		if err := gethTx.UnmarshalBinary(txEvent.Payload); err != nil {
			return fmt.Errorf("failed to RLP-decode transaction [%x]: %w", txEvent.Payload, err)
		}
		res, err = bv.RunTransaction(gethTx)
		if err != nil {
			return fmt.Errorf("failed to run transaction [%x]: %w", txEvent.Hash, err)
		}
	}

	// validate results
	if validate {
		if err := ValidateResult(res, txEvent); err != nil {
			return fmt.Errorf("transaction replay failed (txHash %x): %w", txEvent.Hash, err)
		}
	}

	return nil
}

func ValidateResult(
	res *types.Result,
	txEvent *events.TransactionEventPayload,
) error {

	// we should never produce invalid transaction, since if the transaction was emitted from the evm core
	// it must have either been successful or failed, invalid transactions are not emitted
	if res.Invalid() {
		return fmt.Errorf("invalid transaction: %w", res.ValidationError)
	}

	// check gas consumed
	if res.GasConsumed != txEvent.GasConsumed {
		return fmt.Errorf("gas consumption mismatch %d != %d", res.GasConsumed, txEvent.GasConsumed)
	}

	// check error msg
	if errMsg := res.ErrorMsg(); errMsg != txEvent.ErrorMessage {
		return fmt.Errorf("error msg mismatch %s != %s", errMsg, txEvent.ErrorMessage)
	}

	// check encoded logs
	encodedLogs, err := res.RLPEncodedLogs()
	if err != nil {
		return fmt.Errorf("failed to RLP-encode logs: %w", err)
	}
	if !bytes.Equal(encodedLogs, txEvent.Logs) {
		return fmt.Errorf("encoded logs mismatch %s != %s", encodedLogs, txEvent.ErrorMessage)
	}

	// check deployed address
	if deployedAddress := res.DeployedContractAddressString(); deployedAddress != txEvent.ContractAddress {
		return fmt.Errorf("deployed address mismatch %s != %s", deployedAddress, txEvent.ContractAddress)
	}

	// check the state change checksum
	// if empty checksum skip (supporting blocks before checksum integration)
	if checksum := res.StateChangeChecksum(); txEvent.StateUpdateChecksum != emptyChecksum &&
		checksum != txEvent.StateUpdateChecksum {
		return fmt.Errorf("state change checksum mismatch %x != %x", checksum, txEvent.StateUpdateChecksum)
	}

	return nil
}

func CreateBlockContext(
	chainID flow.ChainID,
	blocks *Blocks,
	tracer *gethTracer.Tracer,
) (types.BlockContext, error) {
	bm, err := blocks.LatestBlock()
	if err != nil {
		return types.BlockContext{}, err
	}
	return types.BlockContext{
		ChainID:                types.EVMChainIDFromFlowChainID(chainID),
		BlockNumber:            bm.Height,
		BlockTimestamp:         bm.Timestamp,
		DirectCallBaseGasUsage: types.DefaultDirectCallBaseGasUsage,
		DirectCallGasPrice:     types.DefaultDirectCallGasPrice,
		GasFeeCollector:        types.CoinbaseAddress,
		GetHashFunc: func(n uint64) gethCommon.Hash {
			hash, err := blocks.BlockHash(n)
			if err != nil {
				panic(err)
			}
			return hash
		},
		Random: bm.Random,
		Tracer: tracer,
	}, nil
}
