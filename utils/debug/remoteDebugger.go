package debug

import (
	"github.com/onflow/cadence"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
)

type RemoteDebugger struct {
	vm  fvm.VM
	ctx fvm.Context
}

// NewRemoteDebugger creates a new remote debugger.
// NOTE: Make sure to use the same version of flow-go as the network
// you are collecting registers from, otherwise the execution might differ
// from the way it runs on the network
func NewRemoteDebugger(
	chain flow.Chain,
	logger zerolog.Logger,
) *RemoteDebugger {
	vm := fvm.NewVirtualMachine()

	// no signature processor here
	// TODO Maybe we add fee-deduction step as well
	ctx := fvm.NewContext(
		fvm.WithLogger(logger),
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
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
	txErr error,
	processError error,
) {
	blockCtx := fvm.NewContextFromParent(
		d.ctx,
		fvm.WithBlockHeader(blockHeader))

	tx := fvm.Transaction(txBody, 0)

	_, output, err := d.vm.Run(blockCtx, tx, snapshot)
	if err != nil {
		return nil, err
	}
	return output.Err, nil
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
