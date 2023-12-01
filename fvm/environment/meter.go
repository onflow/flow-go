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
	ComputationKindHash                       = 2001
	ComputationKindVerifySignature            = 2002
	ComputationKindAddAccountKey              = 2003
	ComputationKindAddEncodedAccountKey       = 2004
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
	_                                         = 2017
	ComputationKindGetStorageCapacity         = 2018
	ComputationKindGetStorageUsed             = 2019
	ComputationKindGetValue                   = 2020
	ComputationKindRemoveAccountContractCode  = 2021
	ComputationKindResolveLocation            = 2022
	ComputationKindRevokeAccountKey           = 2023
	_                                         = 2024 // removed, DO NOT REUSE
	_                                         = 2025 // removed, DO NOT REUSE
	ComputationKindSetValue                   = 2026
	ComputationKindUpdateAccountContractCode  = 2027
	ComputationKindValidatePublicKey          = 2028
	ComputationKindValueExists                = 2029
	ComputationKindAccountKeysCount           = 2030
	ComputationKindBLSVerifyPOP               = 2031
	ComputationKindBLSAggregateSignatures     = 2032
	ComputationKindBLSAggregatePublicKeys     = 2033
	ComputationKindGetOrLoadProgram           = 2034
	ComputationKindGenerateAccountLocalID     = 2035
	ComputationKindGetRandomSourceHistory     = 2036
	ComputationKindEVMGasUsage                = 2037
	ComputationKindRLPEncoding                = 2038
	ComputationKindRLPDecoding                = 2039
	ComputationKindEncodeEvent                = 2040
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
