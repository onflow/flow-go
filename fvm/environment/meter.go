package environment

import (
	"context"

	"github.com/onflow/cadence/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/state"
)

const (
	// [2_000, 3_000) reserved for the FVM
	ComputationKindHash = 2001 + iota
	ComputationKindVerifySignature
	ComputationKindAddAccountKey
	ComputationKindAddEncodedAccountKey
	ComputationKindAllocateSlabIndex
	ComputationKindCreateAccount
	ComputationKindEmitEvent
	ComputationKindGenerateUUID
	ComputationKindGetAccountAvailableBalance
	ComputationKindGetAccountBalance
	ComputationKindGetAccountContractCode
	ComputationKindGetAccountContractNames
	ComputationKindGetAccountKey
	ComputationKindGetBlockAtHeight
	ComputationKindGetCode
	ComputationKindGetCurrentBlockHeight
	_
	ComputationKindGetStorageCapacity
	ComputationKindGetStorageUsed
	ComputationKindGetValue
	ComputationKindRemoveAccountContractCode
	ComputationKindResolveLocation
	ComputationKindRevokeAccountKey
	_ // removed, DO NOT REUSE
	_ // removed, DO NOT REUSE
	ComputationKindSetValue
	ComputationKindUpdateAccountContractCode
	ComputationKindValidatePublicKey
	ComputationKindValueExists
	ComputationKindAccountKeysCount
	ComputationKindBLSVerifyPOP
	ComputationKindBLSAggregateSignatures
	ComputationKindBLSAggregatePublicKeys
	ComputationKindGetOrLoadProgram
	ComputationKindGenerateAccountLocalID
	ComputationKindGetRandomSourceHistory
	ComputationKindEVMGasUsage
	ComputationKindRLPEncoding
	ComputationKindRLPDecoding
	ComputationKindEncodeEvent
	_
	ComputationKindEVMEncodeABI
	ComputationKindEVMDecodeABI
)

// MainnetExecutionEffortWeights are the execution effort weights as they are on mainnet
var MainnetExecutionEffortWeights = meter.ExecutionEffortWeights{
	ComputationKindCreateAccount:                      2143437,
	ComputationKindBLSVerifyPOP:                       1538600,
	ComputationKindGetAccountBalance:                  485476,
	ComputationKindBLSAggregatePublicKeys:             402728,
	ComputationKindGetStorageCapacity:                 397087,
	ComputationKindGetAccountAvailableBalance:         375235,
	ComputationKindUpdateAccountContractCode:          369407,
	ComputationKindBLSAggregateSignatures:             325309,
	ComputationKindGenerateAccountLocalID:             75507,
	ComputationKindGetAccountContractNames:            32771,
	ComputationKindGetStorageUsed:                     25416,
	ComputationKindAccountKeysCount:                   24709,
	ComputationKindAllocateSlabIndex:                  15372,
	common.ComputationKindAtreeMapGet:                 8837,
	common.ComputationKindAtreeMapRemove:              7373,
	common.ComputationKindCreateArrayValue:            4364,
	common.ComputationKindCreateDictionaryValue:       3818,
	common.ComputationKindAtreeMapSet:                 3656,
	common.ComputationKindAtreeArrayInsert:            3652,
	common.ComputationKindAtreeMapReadIteration:       3325,
	ComputationKindEncodeEvent:                        2911,
	common.ComputationKindTransferCompositeValue:      2358,
	common.ComputationKindAtreeArrayAppend:            1907,
	common.ComputationKindStatement:                   1770,
	common.ComputationKindAtreeArraySet:               1737,
	common.ComputationKindFunctionInvocation:          1399,
	common.ComputationKindAtreeMapPopIteration:        1210,
	common.ComputationKindAtreeArrayPopIteration:      736,
	ComputationKindRLPDecoding:                        516,
	common.ComputationKindGraphemesIteration:          278,
	common.ComputationKindUfixParse:                   257,
	common.ComputationKindFixParse:                    223,
	common.ComputationKindLoop:                        179,
	common.ComputationKindAtreeArrayBatchConstruction: 177,
	common.ComputationKindTransferDictionaryValue:     125,
	common.ComputationKindBigIntParse:                 69,
	common.ComputationKindTransferArrayValue:          48,
	ComputationKindSetValue:                           48,
	common.ComputationKindUintParse:                   31,
	common.ComputationKindIntParse:                    28,
	ComputationKindGetValue:                           23,
	common.ComputationKindStringToLower:               5,
	ComputationKindEVMGasUsage:                        3,
}

type Meter interface {
	common.Gauge

	// MeteringResult returns the metering totals accumulated so far.
	//
	// No error returns are expected during normal operation.
	MeteringResult() (meter.MeteringResult, error)

	ComputationRemaining(kind common.ComputationKind) uint64

	MeterEmittedEvent(byteSize uint64) error

	RunWithMeteringDisabled(f func())
}

type meterImpl struct {
	state.NestedTransactionPreparer
}

func NewMeter(txnState state.NestedTransactionPreparer) Meter {
	return &meterImpl{
		NestedTransactionPreparer: txnState,
	}
}

func (m *meterImpl) MeteringResult() (meter.MeteringResult, error) {
	return meter.MeteringResult{
		ComputationUsed:        m.TotalComputationUsed(),
		MemoryEstimate:         m.TotalMemoryEstimate(),
		ComputationIntensities: m.ComputationIntensities(),
	}, nil
}

type cancellableMeter struct {
	meterImpl

	ctx context.Context
}

func NewCancellableMeter(
	ctx context.Context,
	txnState state.NestedTransactionPreparer,
) Meter {
	return &cancellableMeter{
		meterImpl: meterImpl{
			NestedTransactionPreparer: txnState,
		},
		ctx: ctx,
	}
}

func (meter *cancellableMeter) MeterComputation(usage common.ComputationUsage) error {
	// this method is called on every unit of operation, so
	// checking the context here is the most likely would capture
	// timeouts or cancellation as soon as they happen, though
	// we might revisit this when optimizing script execution
	// by only checking on specific kind of Meter calls.
	//
	// in the future this context check should be done inside the cadence
	select {
	case <-meter.ctx.Done():
		err := meter.ctx.Err()
		if errors.Is(err, context.DeadlineExceeded) {
			return errors.NewScriptExecutionTimedOutError()
		}
		return errors.NewScriptExecutionCancelledError(err)
	default:
		// do nothing
	}

	return meter.meterImpl.MeterComputation(usage)
}
