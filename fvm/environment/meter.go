package environment

import (
	"context"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
)

const (
	// [2_000, 3_000) reserved for the FVM
	_ common.ComputationKind = iota + 2_000
	ComputationKindHash
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
	ComputationKindGetProgram
	ComputationKindGetStorageCapacity
	ComputationKindGetStorageUsed
	ComputationKindGetValue
	ComputationKindRemoveAccountContractCode
	ComputationKindResolveLocation
	ComputationKindRevokeAccountKey
	ComputationKindRevokeEncodedAccountKey
	ComputationKindSetProgram
	ComputationKindSetValue
	ComputationKindUpdateAccountContractCode
	ComputationKindValidatePublicKey
	ComputationKindValueExists
)

type Meter interface {
	MeterComputation(common.ComputationKind, uint) error
	ComputationUsed() uint64

	MeterMemory(usage common.MemoryUsage) error
	MemoryEstimate() uint64
}

type meterImpl struct {
	stTxn *state.StateHolder
}

func NewMeter(stTxn *state.StateHolder) Meter {
	return &meterImpl{
		stTxn: stTxn,
	}
}

func (meter *meterImpl) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
	if meter.stTxn.EnforceLimits() {
		return meter.stTxn.MeterComputation(kind, intensity)
	}
	return nil
}

func (meter *meterImpl) ComputationUsed() uint64 {
	return uint64(meter.stTxn.TotalComputationUsed())
}

func (meter *meterImpl) MeterMemory(usage common.MemoryUsage) error {
	if meter.stTxn.EnforceLimits() {
		return meter.stTxn.MeterMemory(usage.Kind, uint(usage.Amount))
	}
	return nil
}

func (meter *meterImpl) MemoryEstimate() uint64 {
	return meter.stTxn.TotalMemoryEstimate()
}

type cancellableMeter struct {
	meterImpl

	ctx context.Context
}

func NewCancellableMeter(ctx context.Context, stTxn *state.StateHolder) Meter {
	return &cancellableMeter{
		meterImpl: meterImpl{
			stTxn: stTxn,
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
