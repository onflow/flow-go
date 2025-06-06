package environment

import (
	"context"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"

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

// MainnetExecutionEffortWeights are the execution effort weights as they are
// on mainnet from crescendo spork
var MainnetExecutionEffortWeights = meter.ExecutionEffortWeights{
	common.ComputationKindStatement:          314,
	common.ComputationKindLoop:               314,
	common.ComputationKindFunctionInvocation: 314,
	ComputationKindGetValue:                  162,
	ComputationKindCreateAccount:             567534,
	ComputationKindSetValue:                  153,
	ComputationKindEVMGasUsage:               13,
}

type Meter interface {
	runtime.MeterInterface

	ComputationIntensities() meter.MeteredComputationIntensities
	ComputationAvailable(common.ComputationUsage) bool

	MeterEmittedEvent(byteSize uint64) error
	TotalEmittedEventBytes() uint64
}

type meterImpl struct {
	txnState state.NestedTransactionPreparer
}

func NewMeter(txnState state.NestedTransactionPreparer) Meter {
	return &meterImpl{
		txnState: txnState,
	}
}

func (meter *meterImpl) MeterComputation(usage common.ComputationUsage) error {
	return meter.txnState.MeterComputation(usage)
}

func (meter *meterImpl) ComputationIntensities() meter.MeteredComputationIntensities {
	return meter.txnState.ComputationIntensities()
}

func (meter *meterImpl) ComputationAvailable(usage common.ComputationUsage) bool {
	return meter.txnState.ComputationAvailable(usage)
}

func (meter *meterImpl) ComputationRemaining(kind common.ComputationKind) uint64 {
	return meter.txnState.ComputationRemaining(kind)
}

func (meter *meterImpl) ComputationUsed() (uint64, error) {
	return meter.txnState.TotalComputationUsed(), nil
}

func (meter *meterImpl) MeterMemory(usage common.MemoryUsage) error {
	return meter.txnState.MeterMemory(usage)
}

func (meter *meterImpl) MemoryUsed() (uint64, error) {
	return meter.txnState.TotalMemoryEstimate(), nil
}

func (meter *meterImpl) InteractionUsed() (uint64, error) {
	return meter.txnState.InteractionUsed(), nil
}

func (meter *meterImpl) MeterEmittedEvent(byteSize uint64) error {
	return meter.txnState.MeterEmittedEvent(byteSize)
}

func (meter *meterImpl) TotalEmittedEventBytes() uint64 {
	return meter.txnState.TotalEmittedEventBytes()
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
			txnState: txnState,
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
