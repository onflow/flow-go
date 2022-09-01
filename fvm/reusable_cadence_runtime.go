package fvm

import (
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/fvm/errors"
)

var setAccountFrozenFunctionType = &sema.FunctionType{
	Parameters: []*sema.Parameter{
		{
			Label:          sema.ArgumentLabelNotRequired,
			Identifier:     "account",
			TypeAnnotation: sema.NewTypeAnnotation(&sema.AddressType{}),
		},
		{
			Label:          sema.ArgumentLabelNotRequired,
			Identifier:     "frozen",
			TypeAnnotation: sema.NewTypeAnnotation(sema.BoolType),
		},
	},
	ReturnTypeAnnotation: &sema.TypeAnnotation{
		Type: sema.VoidType,
	},
}

type AccountFreezer interface {
	SetAccountFrozen(common.Address, bool) error
}

type ReusableCadenceRuntime struct {
	runtime.Environment

	freezer AccountFreezer
}

func (reusable *ReusableCadenceRuntime) SetFreezer(freezer AccountFreezer) {
	reusable.freezer = freezer
}

func NewReusableCadenceRuntime() *ReusableCadenceRuntime {
	reusable := &ReusableCadenceRuntime{
		Environment: runtime.NewBaseInterpreterEnvironment(runtime.Config{}),
	}

	setAccountFrozen := stdlib.StandardLibraryValue{
		Name: "setAccountFrozen",
		Type: setAccountFrozenFunctionType,
		Kind: common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredHostFunctionValue(
			func(invocation interpreter.Invocation) interpreter.Value {
				address, ok := invocation.Arguments[0].(interpreter.AddressValue)
				if !ok {
					panic(errors.NewValueErrorf(invocation.Arguments[0].String(),
						"first argument of setAccountFrozen must be an address"))
				}

				frozen, ok := invocation.Arguments[1].(interpreter.BoolValue)
				if !ok {
					panic(errors.NewValueErrorf(invocation.Arguments[0].String(),
						"second argument of setAccountFrozen must be a boolean"))
				}

				var err error
				if reusable.freezer != nil {
					err = reusable.freezer.SetAccountFrozen(
						common.Address(address),
						bool(frozen))
				} else {
					err = errors.NewOperationNotSupportedError("SetAccountFrozen")
				}

				if err != nil {
					panic(err)
				}

				return interpreter.VoidValue{}
			},
			setAccountFrozenFunctionType,
		),
	}

	reusable.Declare(setAccountFrozen)
	return reusable
}

type ReusableCadenceRuntimePool struct {
	pool chan *ReusableCadenceRuntime
}

func NewReusableCadenceRuntimePool(poolSize int) ReusableCadenceRuntimePool {
	var pool chan *ReusableCadenceRuntime
	if poolSize > 0 {
		pool = make(chan *ReusableCadenceRuntime, poolSize)
	}

	return ReusableCadenceRuntimePool{
		pool: pool,
	}
}

func (pool ReusableCadenceRuntimePool) Borrow(
	freezer AccountFreezer,
) *ReusableCadenceRuntime {
	var reusable *ReusableCadenceRuntime
	select {
	case reusable = <-pool.pool:
		// Do nothing.
	default:
		reusable = NewReusableCadenceRuntime()
	}

	reusable.SetFreezer(freezer)
	return reusable
}

func (pool ReusableCadenceRuntimePool) Return(
	reusable *ReusableCadenceRuntime,
) {
	reusable.SetFreezer(nil)
	select {
	case pool.pool <- reusable:
		// Do nothing.
	default:
		// Do nothing.  Discard the overflow entry.
	}
}
