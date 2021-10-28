package debug

import (
	"github.com/onflow/cadence"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/model/flow"
)

type RemoteDebugger struct {
	vm          *fvm.VirtualMachine
	ctx         fvm.Context
	grpcAddress string
}

// Warning : make sure you use the proper flow-go version, same version as the network you are collecting registers
// from, otherwise the execution might differ from the way runs on the network
// TODO wire to blockHeader from access node (*flow.Header) to the context
func NewRemoteDebugger(grpcAddress string,
	chain flow.Chain,
	logger zerolog.Logger) *RemoteDebugger {
	vm := fvm.NewVirtualMachine(fvm.NewInterpreterRuntime())

	// no signature processor here
	// TODO Maybe we add fee-deduction step as well
	ctx := fvm.NewContext(
		logger,
		fvm.WithChain(chain),
		fvm.WithTransactionProcessors(
			fvm.NewTransactionAccountFrozenChecker(),
			fvm.NewTransactionSequenceNumberChecker(),
			fvm.NewTransactionAccountFrozenEnabler(),
			fvm.NewTransactionInvocator(logger),
		),
	)

	return &RemoteDebugger{
		ctx:         ctx,
		vm:          vm,
		grpcAddress: grpcAddress,
	}
}

// TODO add options, if no blockID provided just collect the latest one
// TODO change the rpc endpoint to access node one maybe

// RunTransaction runs the transaction given the latest sealed block data
func (d *RemoteDebugger) RunTransaction(txBody *flow.TransactionBody) (txErr, processError error) {
	view := NewRemoteView(d.grpcAddress)
	tx := fvm.Transaction(txBody, 0)
	err := d.vm.Run(d.ctx, tx, view, programs.NewEmptyPrograms())
	if err != nil {
		return nil, err
	}
	return tx.Err, nil
}

// RunTransaction runs the transaction and tries to collect the registers at the given blockID
// note that it would be very likely that block is far in the past and you can't find the trie to
// read the registers from
// if regCachePath is empty, the register values won't be cached
func (d *RemoteDebugger) RunTransactionAtBlockID(txBody *flow.TransactionBody, blockID flow.Identifier, regCachePath string) (txErr, processError error) {
	view := NewRemoteView(d.grpcAddress, WithBlockID(blockID))
	if len(regCachePath) > 0 {
		view.Cache = newFileRegisterCache(regCachePath)
	}
	tx := fvm.Transaction(txBody, 0)
	err := d.vm.Run(d.ctx, tx, view, programs.NewEmptyPrograms())
	if err != nil {
		return nil, err
	}
	err = view.Cache.Persist()
	if err != nil {
		return nil, err
	}
	return tx.Err, nil
}

func (d *RemoteDebugger) RunScript(code []byte, arguments [][]byte) (value cadence.Value, scriptError, processError error) {
	view := NewRemoteView(d.grpcAddress)
	script := fvm.Script(code).WithArguments(arguments...)
	err := d.vm.Run(d.ctx, script, view, programs.NewEmptyPrograms())
	if err != nil {
		return nil, nil, err
	}
	return script.Value, script.Err, nil
}

func (d *RemoteDebugger) RunScriptAtBlockID(code []byte, arguments [][]byte, blockID flow.Identifier) (value cadence.Value, scriptError, processError error) {
	view := NewRemoteView(d.grpcAddress, WithBlockID(blockID))
	script := fvm.Script(code).WithArguments(arguments...)
	err := d.vm.Run(d.ctx, script, view, programs.NewEmptyPrograms())
	if err != nil {
		return nil, nil, err
	}
	return script.Value, script.Err, nil
}
