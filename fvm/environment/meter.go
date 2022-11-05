package environment

import (
	"context"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
)

const (
	// [2_000, 3_000) reserved for the FVM
	ComputationKindHash                       = 2001
	ComputationKindVerifySignature            = 2002
	ComputationKindAddAccountKey              = 2003
	_                                         // removed, DO NOT REUSE
	ComputationKindAllocateStorageIndex       = 2005
	ComputationKindCreateAccount              = 2006
	ComputationKindEmitEvent                  = 2007
	ComputationKindGenerateUUID               = 2008
	ComputationKindGetAccountAvailableBalance = 2009
	ComputationKindGetAccountBalance          = 2010
	ComputationKindGetAccountContractCode     = 2011
	ComputationKindGetAccountContractNames    = 2012
	ComputationKindGetAccountKey              = 2013
	ComputationKindGetBlockAtHeight           = 2014
	ComputationKindGetCode                    = 2015
	ComputationKindGetCurrentBlockHeight      = 2016
	ComputationKindGetProgram                 = 2017
	ComputationKindGetStorageCapacity         = 2018
	ComputationKindGetStorageUsed             = 2019
	ComputationKindGetValue                   = 2020
	ComputationKindRemoveAccountContractCode  = 2021
	ComputationKindResolveLocation            = 2022
	ComputationKindRevokeAccountKey           = 2023
	_                                         // removed, DO NOT REUSE
	ComputationKindSetProgram                 = 2025
	ComputationKindSetValue                   = 2026
	ComputationKindUpdateAccountContractCode  = 2027
	ComputationKindValidatePublicKey          = 2028
	ComputationKindValueExists                = 2029
	ComputationKindAccountKeysCount           = 2030
	ComputationKindBLSVerifyPOP               = 2031
	ComputationKindBLSAggregateSignatures     = 2032
	ComputationKindBLSAggregatePublicKeys     = 2033
)

type Meter interface {
	MeterComputation(common.ComputationKind, uint) error
	ComputationUsed() uint64
	ComputationIntensities() meter.MeteredComputationIntensities

	MeterMemory(usage common.MemoryUsage) error
	MemoryEstimate() uint64

	MeterEmittedEvent(byteSize uint64) error
	TotalEmittedEventBytes() uint64
}

type meterImpl struct {
	txnState *state.TransactionState
}

func NewMeter(txnState *state.TransactionState) Meter {
	return &meterImpl{
		txnState: txnState,
	}
}

func (meter *meterImpl) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
	if meter.txnState.EnforceLimits() {
		return meter.txnState.MeterComputation(kind, intensity)
	}
	return nil
}

func (meter *meterImpl) ComputationIntensities() meter.MeteredComputationIntensities {
	return meter.txnState.ComputationIntensities()
}

func (meter *meterImpl) ComputationUsed() uint64 {
	return meter.txnState.TotalComputationUsed()
}

func (meter *meterImpl) MeterMemory(usage common.MemoryUsage) error {
	if meter.txnState.EnforceLimits() {
		return meter.txnState.MeterMemory(usage.Kind, uint(usage.Amount))
	}
	return nil
}

func (meter *meterImpl) MemoryEstimate() uint64 {
	return meter.txnState.TotalMemoryEstimate()
}

func (meter *meterImpl) MeterEmittedEvent(byteSize uint64) error {
	if meter.txnState.EnforceLimits() {
		return meter.txnState.MeterEmittedEvent(byteSize)
	}
	return nil
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
	txnState *state.TransactionState,
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
