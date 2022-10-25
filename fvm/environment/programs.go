package environment

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// TODO(patrick): remove and switch to *programs.TransactionPrograms once
// emulator is updated.
type TransactionPrograms interface {
	Get(loc common.Location) (*interpreter.Program, *state.State, bool)
	Set(loc common.Location, prog *interpreter.Program, state *state.State)
}

// Programs manages operations around cadence program parsing.
//
// Note that cadence guarantees that Get/Set methods are called in a LIFO
// manner. Hence, we create new nested transactions on Get calls and commit
// these nested transactions on Set calls in order to capture the states
// needed for parsing the programs.
type Programs struct {
	tracer *Tracer
	meter  Meter

	txnState *state.TransactionState
	accounts Accounts

	transactionPrograms TransactionPrograms
}

// NewPrograms construts a new ProgramHandler
func NewPrograms(
	tracer *Tracer,
	meter Meter,
	txnState *state.TransactionState,
	accounts Accounts,
	transactionPrograms TransactionPrograms,
) *Programs {
	return &Programs{
		tracer:              tracer,
		meter:               meter,
		txnState:            txnState,
		accounts:            accounts,
		transactionPrograms: transactionPrograms,
	}
}

func (programs *Programs) set(
	location common.Location,
	program *interpreter.Program,
) error {
	// ignore empty locations
	if location == nil {
		return nil
	}

	// transactionPrograms only cache state for AddressLocation, so for non
	// address location, simply set the state to nil.
	address, ok := location.(common.AddressLocation)
	if !ok {
		programs.transactionPrograms.Set(address, program, nil)
		return nil
	}

	state, err := programs.txnState.CommitParseRestricted(address)
	if err != nil {
		return err
	}

	programs.transactionPrograms.Set(address, program, state)
	return nil
}

func (programs *Programs) get(
	location common.Location,
) (
	*interpreter.Program,
	bool,
) {
	// ignore empty locations
	if location == nil {
		return nil, false
	}

	program, state, has := programs.transactionPrograms.Get(location)
	if has {
		if state != nil {
			err := programs.txnState.AttachAndCommit(state)
			if err != nil {
				panic(fmt.Sprintf(
					"merge error while getting program, panic: %s",
					err))
			}
		}
		return program, true
	}

	address, ok := location.(common.AddressLocation)
	if ok {
		// Address location program is reusable across transactions.  Create
		// a nested transaction here in order to capture the states read to
		// parse the program.
		_, err := programs.txnState.BeginParseRestrictedNestedTransaction(
			address)
		if err != nil {
			panic(err)
		}
	}
	return nil, false
}

func (programs *Programs) GetProgram(
	location common.Location,
) (
	*interpreter.Program,
	error,
) {
	defer programs.tracer.StartSpanFromRoot(trace.FVMEnvGetProgram).End()

	err := programs.meter.MeterComputation(ComputationKindGetProgram, 1)
	if err != nil {
		return nil, fmt.Errorf("get program failed: %w", err)
	}

	if addressLocation, ok := location.(common.AddressLocation); ok {
		address := flow.Address(addressLocation.Address)

		freezeError := programs.accounts.CheckAccountNotFrozen(address)
		if freezeError != nil {
			return nil, fmt.Errorf("get program failed: %w", freezeError)
		}
	}

	program, has := programs.get(location)
	if has {
		return program, nil
	}

	return nil, nil
}

func (programs *Programs) SetProgram(
	location common.Location,
	program *interpreter.Program,
) error {
	defer programs.tracer.StartSpanFromRoot(trace.FVMEnvSetProgram).End()

	err := programs.meter.MeterComputation(ComputationKindSetProgram, 1)
	if err != nil {
		return fmt.Errorf("set program failed: %w", err)
	}

	err = programs.set(location, program)
	if err != nil {
		return fmt.Errorf("set program failed: %w", err)
	}
	return nil
}

func (programs *Programs) DecodeArgument(
	bytes []byte,
	_ cadence.Type,
) (
	cadence.Value,
	error,
) {
	defer programs.tracer.StartExtensiveTracingSpanFromRoot(
		trace.FVMEnvDecodeArgument).End()

	v, err := jsoncdc.Decode(programs.meter, bytes)
	if err != nil {
		return nil, fmt.Errorf(
			"decodeing argument failed: %w",
			errors.NewInvalidArgumentErrorf(
				"argument is not json decodable: %w",
				err))
	}

	return v, err
}
