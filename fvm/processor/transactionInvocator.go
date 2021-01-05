package processor

import (
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/flow-go/fvm"
	"github.com/rs/zerolog"
)

type TransactionInvocator struct {
	logger zerolog.Logger
}

func NewTransactionInvocator(logger zerolog.Logger) *TransactionInvocator {
	return &TransactionInvocator{
		logger: logger,
	}
}

func (i *TransactionInvocator) Process(
	vm fvm.VirtualMachine,
	proc fvm.Procedure,
	env fvm.Environment,
) error {

	var txProc fvm.TransactionProcedure
	var ok bool
	if accountProc, ok := proc.(fvm.TransactionProcedure); !ok {
		return errors.New("transaction invocator can only process transaction procedures")
	}

	if txProc.Transaction == nil {
		return errors.New("transaction invocator cannot process a procedure with empty transaction")
	}

	location := runtime.TransactionLocation(txProc.ID()[:])
	err = vm.Runtime.ExecuteTransaction(txProc.Transaction().Script, txProc.Transaction().Arguments, env, location)

	if err != nil {
		i.topshotSafetyErrorCheck(err)
		return err
	}

	i.logger.Info().Str("txHash", proc.ID().String()).Msgf("(%d) ledger interactions used by transaction", st.InteractionUsed())

	// TODO only commit the changes if no error
	// commit changes
	err = st.Commit()
	if err != nil {
		return err
	}

	// Collect the parts we need from env and add it to proc
	// TODO change this to return 
	proc.Events = env.getEvents()
	proc.Logs = env.getLogs()
	proc.GasUsed = evn.ComputationUsed()

	return nil
}

// topshotSafetyErrorCheck is additional check introduced to help chase erroneous execution results
// which caused unexpected network fork. TopShot is first full-fledged game running on Flow, and
// checking failures in this contract indicate the unexpected computation happening.
// This is a temporary measure.
func (i *TransactionInvocator) topshotSafetyErrorCheck(err error) {
	e := err.Error()
	i.logger.Info().Str("error", e).Msg("TEMP LOGGING: Cadence Execution ERROR")
	if strings.Contains(e, "checking") {
		re, isRuntime := err.(runtime.Error)
		if !isRuntime {
			i.logger.Err(err).Msg("found checking error for a contract but exception is not RuntimeError")
			return
		}
		ee, is := re.Err.(*runtime.ParsingCheckingError)
		if !is {
			i.logger.Err(err).Msg("found checking error for a contract but exception is not ExtendedParsingCheckingError")
			return
		}

		// serializing such large and complex objects to JSON
		// causes stack overflow, spew works fine
		spew.Config.DisableMethods = true
		dump := spew.Sdump(ee)

		i.logger.Error().Str("extended_error", dump).Msg("contract checking failed")
	}
}
