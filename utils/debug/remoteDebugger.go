package debug

import (
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

type Result struct {
	Snapshot *snapshot.ExecutionSnapshot
	Output   fvm.ProcedureOutput
}

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
	options ...fvm.Option,
) *RemoteDebugger {
	vm := fvm.NewVirtualMachine()

	// no signature processor here
	// TODO Maybe we add fee-deduction step as well

	ctx := fvm.NewContext(chain, append(
		[]fvm.Option{
			fvm.WithLogger(logger),
			fvm.WithAuthorizationChecksEnabled(false),
		},
		options...,
	)...)

	return &RemoteDebugger{
		ctx: ctx,
		vm:  vm,
	}
}

// RunTransaction runs the transaction using the given storage snapshot.
func (d *RemoteDebugger) RunTransaction(
	txBody *flow.TransactionBody,
	isSystemTransaction bool,
	snapshot StorageSnapshot,
	blockHeader *flow.Header,
) (
	Result,
	error,
) {
	opts := []fvm.Option{
		fvm.WithBlockHeader(blockHeader),
	}

	if isSystemTransaction {
		opts = append(
			opts,
			fvm.WithAuthorizationChecksEnabled(false),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithRandomSourceHistoryCallAllowed(true),
			fvm.WithAccountStorageLimit(false),
		)
	}

	blockCtx := fvm.NewContextFromParent(
		d.ctx,
		opts...,
	)

	tx := fvm.Transaction(txBody, 0)

	resultSnapshot, output, err := d.vm.Run(blockCtx, tx, snapshot)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Snapshot: resultSnapshot,
		Output:   output,
	}, nil
}

func (d *RemoteDebugger) RunSDKTransaction(
	tx *sdk.Transaction,
	snapshot StorageSnapshot,
	header *flow.Header,
) (
	Result,
	error,
) {
	txBodyBuilder := flow.NewTransactionBodyBuilder().
		SetScript(tx.Script).
		SetComputeLimit(tx.GasLimit).
		SetPayer(flow.Address(tx.Payer))

	for _, argument := range tx.Arguments {
		txBodyBuilder.AddArgument(argument)
	}

	for _, authorizer := range tx.Authorizers {
		txBodyBuilder.AddAuthorizer(flow.Address(authorizer))
	}

	txBodyBuilder.SetProposalKey(
		flow.Address(tx.ProposalKey.Address),
		tx.ProposalKey.KeyIndex,
		tx.ProposalKey.SequenceNumber,
	)

	txBody, err := txBodyBuilder.Build()
	if err != nil {
		return Result{}, err
	}

	return d.RunTransaction(
		txBody,
		isSystemTransaction(tx),
		snapshot,
		header,
	)
}

func isSystemTransaction(tx *sdk.Transaction) bool {
	return tx.Payer == sdk.EmptyAddress
}

// RunScript runs the script using the given storage snapshot.
func (d *RemoteDebugger) RunScript(
	code []byte,
	arguments [][]byte,
	snapshot StorageSnapshot,
	blockHeader *flow.Header,
) (
	Result,
	error,
) {
	scriptCtx := fvm.NewContextFromParent(
		d.ctx,
		fvm.WithBlockHeader(blockHeader),
	)

	script := fvm.Script(code).
		WithArguments(arguments...)

	resultSnapshot, output, err := d.vm.Run(scriptCtx, script, snapshot)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Snapshot: resultSnapshot,
		Output:   output,
	}, nil
}
