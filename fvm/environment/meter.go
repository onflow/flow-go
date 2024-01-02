package environment

import (
	"context"

	"github.com/onflow/cadence/runtime/common"

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
	ComputationKindAllocateStorageIndex
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

type Meter interface {
	MeterComputation(common.ComputationKind, uint) error
	ComputationUsed() (uint64, error)
	ComputationIntensities() meter.MeteredComputationIntensities
	ComputationAvailable(common.ComputationKind, uint) bool

	MeterMemory(usage common.MemoryUsage) error
	MemoryUsed() (uint64, error)

	MeterEmittedEvent(byteSize uint64) error
	TotalEmittedEventBytes() uint64

	InteractionUsed() (uint64, error)
}

type meterImpl struct {
	txnState state.NestedTransactionPreparer
}

func NewMeter(txnState state.NestedTransactionPreparer) Meter {
	return &meterImpl{
		txnState: txnState,
	}
}

func (meter *meterImpl) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
	return meter.txnState.MeterComputation(kind, intensity)
}

func (meter *meterImpl) ComputationIntensities() meter.MeteredComputationIntensities {
	return meter.txnState.ComputationIntensities()
}

func (meter *meterImpl) ComputationAvailable(
	kind common.ComputationKind,
	intensity uint,
) bool {
	return meter.txnState.ComputationAvailable(kind, intensity)
}

func (meter *meterImpl) ComputationUsed() (uint64, error) {
	return meter.txnState.TotalComputationUsed(), nil
}

func (meter *meterImpl) MeterMemory(usage common.MemoryUsage) error {
	return meter.txnState.MeterMemory(usage.Kind, uint(usage.Amount))
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

func (meter *cancellableMeter) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
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

	return meter.meterImpl.MeterComputation(kind, intensity)
}
