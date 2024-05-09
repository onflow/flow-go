package debug

import (
	"github.com/onflow/cadence"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
)

type RemoteDebugger struct {
	vm          fvm.VM
	ctx         fvm.Context
	grpcAddress string
}

// Warning : make sure you use the proper flow-go version, same version as the network you are collecting registers
// from, otherwise the execution might differ from the way runs on the network
func NewRemoteDebugger(grpcAddress string,
	chain flow.Chain,
	logger zerolog.Logger) *RemoteDebugger {
	vm := fvm.NewVirtualMachine()

	// no signature processor here
	// TODO Maybe we add fee-deduction step as well
	ctx := fvm.NewContext(
		fvm.WithLogger(logger),
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
	)

	return &RemoteDebugger{
		ctx:         ctx,
		vm:          vm,
		grpcAddress: grpcAddress,
	}
}

// RunTransaction runs the transaction given the latest sealed block data
func (d *RemoteDebugger) RunTransaction(
	txBody *flow.TransactionBody,
) (
	txErr error,
	processError error,
) {
	snapshot := NewRemoteStorageSnapshot(d.grpcAddress)
	defer snapshot.Close()

	blockCtx := fvm.NewContextFromParent(
		d.ctx,
		fvm.WithBlockHeader(d.ctx.BlockHeader))
	tx := fvm.Transaction(txBody, 0)
	_, output, err := d.vm.Run(blockCtx, tx, snapshot)
	if err != nil {
		return nil, err
	}
	return output.Err, nil
}

// RunTransaction runs the transaction and tries to collect the registers at
// the given blockID note that it would be very likely that block is far in the
// past and you can't find the trie to read the registers from.
// if regCachePath is empty, the register values won't be cached
func (d *RemoteDebugger) RunTransactionAtBlockID(
	txBody *flow.TransactionBody,
	blockID flow.Identifier,
	regCachePath string,
) (
	txErr error,
	processError error,
) {
	snapshot := NewRemoteStorageSnapshot(d.grpcAddress, WithBlockID(blockID))
	defer snapshot.Close()

	blockCtx := fvm.NewContextFromParent(
		d.ctx,
		fvm.WithBlockHeader(d.ctx.BlockHeader))
	if len(regCachePath) > 0 {
		snapshot.Cache = newFileRegisterCache(regCachePath)
	}
	tx := fvm.Transaction(txBody, 0)
	_, output, err := d.vm.Run(blockCtx, tx, snapshot)
	if err != nil {
		return nil, err
	}
	err = snapshot.Cache.Persist()
	if err != nil {
		return nil, err
	}
	return output.Err, nil
}

func (d *RemoteDebugger) RunScript(
	code []byte,
	arguments [][]byte,
) (
	value cadence.Value,
	scriptError error,
	processError error,
) {
	snapshot := NewRemoteStorageSnapshot(d.grpcAddress)
	defer snapshot.Close()

	scriptCtx := fvm.NewContextFromParent(
		d.ctx,
		fvm.WithBlockHeader(d.ctx.BlockHeader))
	script := fvm.Script(code).WithArguments(arguments...)
	_, output, err := d.vm.Run(scriptCtx, script, snapshot)
	if err != nil {
		return nil, nil, err
	}
	return output.Value, output.Err, nil
}

func (d *RemoteDebugger) RunScriptAtBlockID(
	code []byte,
	arguments [][]byte,
	blockID flow.Identifier,
) (
	value cadence.Value,
	scriptError error,
	processError error,
) {
	snapshot := NewRemoteStorageSnapshot(d.grpcAddress, WithBlockID(blockID))
	defer snapshot.Close()

	scriptCtx := fvm.NewContextFromParent(
		d.ctx,
		fvm.WithBlockHeader(d.ctx.BlockHeader))
	script := fvm.Script(code).WithArguments(arguments...)
	_, output, err := d.vm.Run(scriptCtx, script, snapshot)
	if err != nil {
		return nil, nil, err
	}
	return output.Value, output.Err, nil
}
