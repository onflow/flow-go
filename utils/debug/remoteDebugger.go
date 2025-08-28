package debug

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

type RemoteDebugger struct {
	vm  fvm.VM
	ctx fvm.Context
}

// ReusableCadenceRuntimePoolSize is the size of the reusable cadence runtime pool.
// Copied from engine/execution/computation/manager.go to avoid circular dependency.
const reusableCadenceRuntimePoolSize = 1000

// NewRemoteDebugger creates a new remote debugger.
// NOTE: Make sure to use the same version of flow-go as the network
// you are collecting registers from, otherwise the execution might differ
// from the way it runs on the network
func NewRemoteDebugger(
	chain flow.Chain,
	logger zerolog.Logger,
	traceCadence bool,
	options ...fvm.Option,
) *RemoteDebugger {
	vm := fvm.NewVirtualMachine()

	// no signature processor here
	// TODO Maybe we add fee-deduction step as well

	ctx := fvm.NewContext(
		append(
			[]fvm.Option{
				fvm.WithLogger(logger),
				fvm.WithChain(chain),
				fvm.WithAuthorizationChecksEnabled(false),
				fvm.WithReusableCadenceRuntimePool(
					reusableRuntime.NewReusableCadenceRuntimePool(
						reusableCadenceRuntimePoolSize,
						runtime.Config{
							TracingEnabled: traceCadence,
						},
					)),
				fvm.WithEVMEnabled(true),
			},
			options...,
		)...,
	)

	return &RemoteDebugger{
		ctx: ctx,
		vm:  vm,
	}
}

// RunTransaction runs the transaction using the given storage snapshot.
func (d *RemoteDebugger) RunTransaction(
	txBody *flow.TransactionBody,
	snapshot StorageSnapshot,
	blockHeader *flow.Header,
) (
	resultSnapshot *snapshot.ExecutionSnapshot,
	txErr error,
	processError error,
) {
	blockCtx := fvm.NewContextFromParent(
		d.ctx,
		fvm.WithBlockHeader(blockHeader))

	tx := fvm.Transaction(txBody, 0)

	var (
		output fvm.ProcedureOutput
		err    error
	)
	resultSnapshot, output, err = d.vm.Run(blockCtx, tx, snapshot)
	if err != nil {
		return resultSnapshot, nil, err
	}
	return resultSnapshot, output.Err, nil
}

// RunScript runs the script using the given storage snapshot.
func (d *RemoteDebugger) RunScript(
	code []byte,
	arguments [][]byte,
	snapshot StorageSnapshot,
	blockHeader *flow.Header,
) (
	value cadence.Value,
	scriptError error,
	processError error,
) {
	scriptCtx := fvm.NewContextFromParent(
		d.ctx,
		fvm.WithBlockHeader(blockHeader),
	)

	script := fvm.Script(code).WithArguments(arguments...)

	_, output, err := d.vm.Run(scriptCtx, script, snapshot)
	if err != nil {
		return nil, nil, err
	}

	return output.Value, output.Err, nil
}
