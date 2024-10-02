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

// ReplayBlockExecution re-executes transactions of a block using the
// events emitted when transactions where executed.
// it updates the state of the given ledger and uses the trace
func ReplayBlockExecution(
	chainID flow.ChainID,
	ledger atree.Ledger,
	blocks types.BlockHashProvider,
	tracer *gethTracer.Tracer,
	transactionEvents []events.TransactionEventPayload,
	blockEvent events.BlockEventPayload,
	validateResults bool,
) error {
	// reusing the same base context for all transactions
	ctx := createBlockContext(chainID, blocks, tracer, blockEvent)
	gasConsumedSoFar := uint64(0)
	txHashes := make(types.TransactionHashes, len(transactionEvents))
	var err error
	for idx, tx := range transactionEvents {
		err = replayTransactionExecution(
			chainID,
			ctx,
			uint(idx),
			gasConsumedSoFar,
			ledger,
			&tx,
			validateResults,
		)
		if err != nil {
			return err
		}
		gasConsumedSoFar += tx.GasConsumed
		txHashes[idx] = tx.Hash
	}

	// check transaction inclusion
	txHashRoot := gethTypes.DeriveSha(txHashes, gethTrie.NewStackTrie(nil))
	if txHashRoot != blockEvent.TransactionHashRoot {
		return fmt.Errorf("transaction root hash doesn't match [%x] != [%x]", txHashRoot, blockEvent.TransactionHashRoot)
	}

	// check total gas used
	if blockEvent.TotalGasUsed != gasConsumedSoFar {
		return fmt.Errorf("total gas used doesn't match [%x] != [%x]", txHashRoot, blockEvent.TransactionHashRoot)

	}
	// no need to check the receipt root hash given we have checked the logs and other
	// values during tx execution.
	return nil
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
		if err := validateResult(res, txEvent); err != nil {
			return fmt.Errorf("transaction replay failed (txHash %x): %w", txEvent.Hash, err)
		}
	}

	return nil
}

func validateResult(
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

	// TODO: check state update checksum when is deployed

	return nil
}

func createBlockContext(
	chainID flow.ChainID,
	blocks types.BlockHashProvider,
	tracer *gethTracer.Tracer,
	blkEvent events.BlockEventPayload,
) types.BlockContext {
	return types.BlockContext{
		ChainID:                types.EVMChainIDFromFlowChainID(chainID),
		BlockNumber:            blkEvent.Height,
		BlockTimestamp:         blkEvent.Timestamp,
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
		Random: blkEvent.PrevRandao,
		Tracer: tracer,
	}
}
