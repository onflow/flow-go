package environment

import (
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
)

func parseRestricted(txnState *state.TransactionState, opCode string) error {
	if txnState.IsParseRestricted() {
		return errors.NewParseRestrictedModeInvalidAccessFailure(opCode)
	}

	return nil
}

// Utility functions used for checking unexpected operation access while
// cadence is parsing programs.
//
// The generic functions are of the form
//      parseRestrict<x>Arg<y>Ret(txnState, opCode, callback, arg1, ..., argX)
// where the callback expects <x> number of arguments, and <y> number of
// return values (not counting error). If the callback expects no argument,
// `<x>Arg` is omitted, and similarly for return value.

func parseRestrict2Arg[Arg1T any, Arg2T any](
	txnState *state.TransactionState,
	opCode string,
	callback func(Arg1T, Arg2T) error,
	arg1 Arg1T,
	arg2 Arg2T,
) error {
	err := parseRestricted(txnState, opCode)
	if err != nil {
		return err
	}

	return callback(arg1, arg2)
}

func parseRestrict3Arg[Arg1T any, Arg2T any, Arg3T any](
	txnState *state.TransactionState,
	opCode string,
	callback func(Arg1T, Arg2T, Arg3T) error,
	arg1 Arg1T,
	arg2 Arg2T,
	arg3 Arg3T,
) error {
	err := parseRestricted(txnState, opCode)
	if err != nil {
		return err
	}

	return callback(arg1, arg2, arg3)
}

func parseRestrict1Ret[RetT any](
	txnState *state.TransactionState,
	opCode string,
	callback func() (RetT, error),
) (
	RetT,
	error,
) {
	err := parseRestricted(txnState, opCode)
	if err != nil {
		var value RetT
		return value, err
	}

	return callback()
}

func parseRestrict1Arg1Ret[ArgT any, RetT any](
	txnState *state.TransactionState,
	opCode string,
	callback func(ArgT) (RetT, error),
	arg ArgT,
) (
	RetT,
	error,
) {
	err := parseRestricted(txnState, opCode)
	if err != nil {
		var value RetT
		return value, err
	}

	return callback(arg)
}

func parseRestrict2Arg1Ret[Arg1T any, Arg2T any, RetT any](
	txnState *state.TransactionState,
	opCode string,
	callback func(Arg1T, Arg2T) (RetT, error),
	arg1 Arg1T,
	arg2 Arg2T,
) (
	RetT,
	error,
) {
	err := parseRestricted(txnState, opCode)
	if err != nil {
		var value RetT
		return value, err
	}

	return callback(arg1, arg2)
}

func parseRestrict4Arg1Ret[
	Arg1T any,
	Arg2T any,
	Arg3T any,
	Arg4T any,
	RetT any,
](
	txnState *state.TransactionState,
	opCode string,
	callback func(Arg1T, Arg2T, Arg3T, Arg4T) (RetT, error),
	arg1 Arg1T,
	arg2 Arg2T,
	arg3 Arg3T,
	arg4 Arg4T,
) (
	RetT,
	error,
) {
	err := parseRestricted(txnState, opCode)
	if err != nil {
		var value RetT
		return value, err
	}

	return callback(arg1, arg2, arg3, arg4)
}
