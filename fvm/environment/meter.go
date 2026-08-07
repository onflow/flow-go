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
// from the FLIP 370 calibration (https://github.com/onflow/flips/blob/main/protocol/20260611-execution-effort-3.md),
// deployed to mainnet on 2026-07-27.
var MainnetExecutionEffortWeights = meter.ExecutionEffortWeights{
	ComputationKindBLSAggregatePublicKeys:                  63851230,
	ComputationKindBLSAggregateSignatures:                  45031044,
	ComputationKindCreateAccount:                           15751197,
	ComputationKindBLSVerifyPOP:                            9888886,
	ComputationKindUpdateAccountContractCode:               1923478,
	ComputationKindGetAccountBalance:                       1828131,
	ComputationKindGetStorageCapacity:                      1497945,
	ComputationKindGetAccountAvailableBalance:              1443208,
	ComputationKindVerifySignature:                         627265,
	ComputationKindGenerateAccountLocalID:                  249708,
	common.ComputationKindDestroyArrayValue:                155788,
	ComputationKindGetStorageUsed:                          147167,
	ComputationKindGetAccountContractNames:                 135863,
	ComputationKindAccountKeysCount:                        103486,
	ComputationKindEncodeEvent:                             46569,
	common.ComputationKindAtreeMapHas:                      46122,
	common.ComputationKindAtreeArrayGet:                    42997,
	ComputationKindAllocateSlabIndex:                       36924,
	common.ComputationKindAtreeMapGet:                      35646,
	ComputationKindGenerateUUID:                            31935,
	common.ComputationKindAtreeMapSet:                      26932,
	common.ComputationKindAtreeArrayInsert:                 22087,
	common.ComputationKindAtreeMapRemove:                   20860,
	ComputationKindHash:                                    20801,
	common.ComputationKindCreateArrayValue:                 20072,
	common.ComputationKindAtreeMapConstruction:             12342,
	common.ComputationKindAtreeArrayAppend:                 12291,
	common.ComputationKindAtreeMapReadIteration:            10864,
	common.ComputationKindFunctionInvocation:               10547,
	common.ComputationKindAtreeMapBatchConstruction:        8289,
	common.ComputationKindDestroyDictionaryValue:           7533,
	common.ComputationKindAtreeArraySet:                    7199,
	common.ComputationKindCreateCompositeValue:             6704,
	common.ComputationKindStatement:                        5610,
	common.ComputationKindLoop:                             4467,
	common.ComputationKindAtreeArrayPopIteration:           2052,
	common.ComputationKindUfixParse:                        1807,
	ComputationKindRLPDecoding:                             1791,
	common.ComputationKindFixParse:                         1508,
	common.ComputationKindGraphemesIteration:               1245,
	common.ComputationKindBigIntParse:                      1102,
	common.ComputationKindUintParse:                        833,
	common.ComputationKindAtreeArrayBatchConstruction:      830,
	common.ComputationKindIntParse:                         742,
	common.ComputationKindAtreeArraySingleSlabConstruction: 534,
	ComputationKindEVMDecodeABI:                            399,
	common.ComputationKindWordSliceOperation:               353,
	ComputationKindGetValue:                                247,
	ComputationKindSetValue:                                30,
	common.ComputationKindStringToLower:                    24,
	ComputationKindEVMGasUsage:                             7,
}

type Meter interface {
	// Gauge provides MeterComputation and MeterMemory. Both may return
	// [errors.LimitExceededError] when the corresponding metering limit is
	// exceeded. In script environments, MeterComputation may additionally
	// return [errors.ScriptExecutionTimedOutError] or
	// [errors.ScriptExecutionCancelledError].
	common.Gauge

	// MeteringResult returns the metering totals accumulated so far.
	MeteringResult() meter.MeteringResult

	ComputationRemaining(kind common.ComputationKind) uint64

	// MeterEmittedEvent captures the byte size of an emitted event.
	//
	// Expected error returns during normal operation:
	//   - [errors.LimitExceededError] with [errors.LimitKindEvent] if the
	//     event byte size limit is exceeded
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

func (m *meterImpl) MeteringResult() meter.MeteringResult {
	return meter.MeteringResult{
		ComputationUsed:        m.TotalComputationUsed(),
		MemoryEstimate:         m.TotalMemoryEstimate(),
		ComputationIntensities: m.ComputationIntensities(),
	}
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

// MeterComputation checks for script cancellation and timeout before
// delegating to the embedded meter.
//
// Expected error returns during normal operation:
//   - [errors.ScriptExecutionTimedOutError] if the script exceeded its
//     allotted execution time
//   - [errors.ScriptExecutionCancelledError] if the script's context was
//     cancelled
//   - [errors.LimitExceededError] if a metering limit is exceeded
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
