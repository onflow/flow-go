package procedures

import (
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/context"
	"github.com/onflow/flow-go/fvm/env"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/processors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionProcedure struct {
	ID            flow.Identifier
	Transaction   *flow.TransactionBody
	TxIndex       uint32
	Logs          []string
	Events        []flow.Event
	ServiceEvents []flow.Event
	GasUsed       uint64
	Err           errors.Error
	Retried       int
	TraceSpan     opentracing.Span
}

var (
	accountFrozenChecker processors.AccountFrozenChecker
	signatureVerifier    processors.TransactionSignatureVerifier
	seqNumChecker        processors.SequenceNumberChecker
	feeDeductor          processors.FeeDeductor
	accountFrozenEnabler processors.AccountFrozenEnabler
	txInvocator          processors.TransactionInvocator
)

func (proc *TransactionProcedure) SetTraceSpan(traceSpan opentracing.Span) {
	proc.TraceSpan = traceSpan
}

func (proc *TransactionProcedure) Run(vm context.VirtualMachine, ctx context.Context, sth *state.StateHolder, programs *programs.Programs) error {

	accounts := state.NewAccounts(sth)
	txEnv := env.NewTransactionEnvironment(ctx, vm, sth, accounts, programs, proc.Transaction, proc.TxIndex)

	var err error
	// check accounts not be frozen
	err = accountFrozenChecker.Check(proc.Transaction, accounts)
	if err != nil {
		proc.handleError(err, ctx.Logger)
		// TODO we should not break here we should continue for fee deductions
		return nil
	}

	// TODO rest of the steps
	return nil
}

func (proc *TransactionProcedure) handleError(err error, logger zerolog.Logger) {
	txErr, failure := errors.SplitErrorTypes(err)
	if failure != nil {
		logger.Err(err).Msg("fatal error when execution a transaction")
		panic(failure)
	}
	if txErr != nil {
		proc.Err = txErr
	}
}
