package environment

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type AccountLocalIDGenerator interface {
	GenerateAccountID(address common.Address) (uint64, error)
}

type ParseRestrictedAccountLocalIDGenerator struct {
	txnState state.NestedTransactionPreparer
	impl     AccountLocalIDGenerator
}

func NewParseRestrictedAccountLocalIDGenerator(
	txnState state.NestedTransactionPreparer,
	impl AccountLocalIDGenerator,
) AccountLocalIDGenerator {
	return ParseRestrictedAccountLocalIDGenerator{
		txnState: txnState,
		impl:     impl,
	}
}

func (generator ParseRestrictedAccountLocalIDGenerator) GenerateAccountID(
	address common.Address,
) (uint64, error) {
	return parseRestrict1Arg1Ret(
		generator.txnState,
		trace.FVMEnvGenerateAccountLocalID,
		generator.impl.GenerateAccountID,
		address)
}

type accountLocalIDGenerator struct {
	tracer   tracing.TracerSpan
	meter    Meter
	accounts Accounts
}

func NewAccountLocalIDGenerator(
	tracer tracing.TracerSpan,
	meter Meter,
	accounts Accounts,
) AccountLocalIDGenerator {
	return &accountLocalIDGenerator{
		tracer:   tracer,
		meter:    meter,
		accounts: accounts,
	}
}

func (generator *accountLocalIDGenerator) GenerateAccountID(
	runtimeAddress common.Address,
) (
	uint64,
	error,
) {
	defer generator.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvGenerateAccountLocalID,
	).End()

	err := generator.meter.MeterComputation(ComputationKindGenerateAccountLocalID, 1)
	if err != nil {
		return 0, err
	}

	return generator.accounts.GenerateAccountLocalID(
		flow.ConvertAddress(runtimeAddress),
	)
}
