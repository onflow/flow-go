package debug

import (
	"strings"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
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

type DebugInfo struct {
	RegistersRead []RegisterInfo
}

func (v *DebugInfo) RegistersReadCollapseSlabReads() []RegisterInfo {
	compact := make([]RegisterInfo, 0)
	lastIndex := -1

	for _, info := range v.RegistersRead {
		if strings.HasPrefix(info.Key, "24") && lastIndex >= 0 {
			compact[lastIndex].Size += info.Size
			continue
		}
		compact = append(compact, info)
		lastIndex += 1
	}

	return compact
}

// Warning : make sure you use the proper flow-go version, same version as the network you are collecting registers
// from, otherwise the execution might differ from the way runs on the network
func NewRemoteDebugger(
	grpcAddress string,
	chain flow.Chain,
	logger zerolog.Logger) *RemoteDebugger {
	vm := fvm.NewVirtualMachine(fvm.NewInterpreterRuntime(runtime.Config{}))

	// no signature processor here
	ctx := fvm.NewContext(
		logger,
		fvm.WithChain(chain),
		fvm.WithTransactionProcessors(
			fvm.NewTransactionInvoker(logger),
		),
	)

	return &RemoteDebugger{
		ctx:         ctx,
		vm:          vm,
		grpcAddress: grpcAddress,
	}
}

// RunTransaction runs the transaction given the latest sealed block data
func (d *RemoteDebugger) RunTransaction(txBody *flow.TransactionBody) (txErr, processError error, info DebugInfo) {
	view := NewRemoteView(d.grpcAddress)
	blockCtx := fvm.NewContextFromParent(d.ctx, fvm.WithBlockHeader(d.ctx.BlockHeader))
	tx := fvm.Transaction(txBody, 0)

	processError = d.vm.Run(blockCtx, tx, view, programs.NewEmptyPrograms())
	txErr = tx.Err
	info.RegistersRead = view.RegistersRead()

	return
}

// RunTransaction runs the transaction and tries to collect the registers at the given blockID
// note that it would be very likely that block is far in the past and you can't find the trie to
// read the registers from
// if regCachePath is empty, the register values won't be cached
func (d *RemoteDebugger) RunTransactionAtBlockID(txBody *flow.TransactionBody, blockID flow.Identifier, regCachePath string) (txErr, processError error, info DebugInfo) {
	view := NewRemoteView(d.grpcAddress, WithBlockID(blockID))
	defer view.Done()

	blockCtx := fvm.NewContextFromParent(d.ctx, fvm.WithBlockHeader(d.ctx.BlockHeader))
	if len(regCachePath) > 0 {
		view.Cache = newFileRegisterCache(regCachePath)
	}
	tx := fvm.Transaction(txBody, 0)

	processError = d.vm.Run(blockCtx, tx, view, programs.NewEmptyPrograms())
	txErr = tx.Err
	info.RegistersRead = view.RegistersRead()

	if processError != nil {
		processError = view.Cache.Persist()
	}
	return
}

func (d *RemoteDebugger) RunScript(code []byte, arguments [][]byte) (value cadence.Value, scriptError, processError error, info DebugInfo) {
	view := NewRemoteView(d.grpcAddress)
	scriptCtx := fvm.NewContextFromParent(d.ctx, fvm.WithBlockHeader(d.ctx.BlockHeader))
	script := fvm.Script(code).WithArguments(arguments...)

	processError = d.vm.Run(scriptCtx, script, view, programs.NewEmptyPrograms())
	if processError != nil {
		return
	}
	scriptError = script.Err
	value = script.Value
	info.RegistersRead = view.RegistersRead()

	return
}

func (d *RemoteDebugger) RunScriptAtBlockID(code []byte, arguments [][]byte, blockID flow.Identifier) (value cadence.Value, scriptError, processError error, info DebugInfo) {
	view := NewRemoteView(d.grpcAddress, WithBlockID(blockID))
	scriptCtx := fvm.NewContextFromParent(d.ctx, fvm.WithBlockHeader(d.ctx.BlockHeader))
	script := fvm.Script(code).WithArguments(arguments...)
	processError = d.vm.Run(scriptCtx, script, view, programs.NewEmptyPrograms())
	if processError != nil {
		return
	}
	scriptError = script.Err
	value = script.Value
	info.RegistersRead = view.RegistersRead()

	return
}
