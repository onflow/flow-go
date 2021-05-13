package computation

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/cadence"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

var scriptLogThreshold = 1 * time.Second

func TestComputeBlockWithStorage(t *testing.T) {
	rt := fvm.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine(rt)
	execCtx := fvm.NewContext(zerolog.Nop(), fvm.WithChain(chain))

	privateKeys, err := testutil.GenerateAccountPrivateKeys(2)
	require.NoError(t, err)

	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
	require.NoError(t, err)

	tx1 := testutil.DeployCounterContractTransaction(accounts[0], chain)
	tx1.SetProposalKey(chain.ServiceAddress(), 0, 0).
		SetGasLimit(1000).
		SetPayer(chain.ServiceAddress())

	err = testutil.SignPayload(tx1, accounts[0], privateKeys[0])
	require.NoError(t, err)

	err = testutil.SignEnvelope(tx1, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	tx2 := testutil.CreateCounterTransaction(accounts[0], accounts[1])
	tx2.SetProposalKey(chain.ServiceAddress(), 0, 0).
		SetGasLimit(1000).
		SetPayer(chain.ServiceAddress())

	err = testutil.SignPayload(tx2, accounts[1], privateKeys[1])
	require.NoError(t, err)

	err = testutil.SignEnvelope(tx2, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	transactions := []*flow.TransactionBody{tx1, tx2}

	col := flow.Collection{Transactions: transactions}

	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signature:    nil,
	}

	block := flow.Block{
		Header: &flow.Header{
			View: 42,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee},
		},
	}

	executableBlock := &entity.ExecutableBlock{
		Block: &block,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee.ID(): {
				Guarantee:    &guarantee,
				Transactions: transactions,
			},
		},
		StartState: unittest.StateCommitmentPointerFixture(),
	}

	me := new(module.Local)
	me.On("NodeID").Return(flow.ZeroID)

	blockComputer, err := computer.NewBlockComputer(vm, execCtx, nil, trace.NewNoopTracer(), zerolog.Nop(), committer.NewNoopViewCommitter())
	require.NoError(t, err)

	programsCache, err := NewProgramsCache(10)
	require.NoError(t, err)

	engine := &Manager{
		blockComputer: blockComputer,
		me:            me,
		programsCache: programsCache,
	}

	view := delta.NewView(ledger.Get)
	blockView := view.NewChild()

	returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, blockView)
	require.NoError(t, err)

	require.NotEmpty(t, blockView.(*delta.View).Delta())
	require.Len(t, returnedComputationResult.StateSnapshots, 1+1) // 1 coll + 1 system chunk
	assert.NotEmpty(t, returnedComputationResult.StateSnapshots[0].Delta)
	assert.True(t, returnedComputationResult.ComputationUsed > 0)
}

func TestExecuteScript(t *testing.T) {

	logger := zerolog.Nop()

	execCtx := fvm.NewContext(logger)

	me := new(module.Local)
	me.On("NodeID").Return(flow.ZeroID)

	rt := fvm.NewInterpreterRuntime()

	vm := fvm.NewVirtualMachine(rt)

	ledger := testutil.RootBootstrappedLedger(vm, execCtx)

	view := delta.NewView(ledger.Get)

	scriptView := view.NewChild()

	script := []byte(fmt.Sprintf(
		`
			import FungibleToken from %s

			pub fun main() {}
		`,
		fvm.FungibleTokenAddress(execCtx.Chain).HexWithPrefix(),
	))

	engine, err := New(logger, nil, nil, me, nil, vm, execCtx, DefaultProgramsCacheSize, committer.NewNoopViewCommitter(), scriptLogThreshold)
	require.NoError(t, err)

	header := unittest.BlockHeaderFixture()
	_, err = engine.ExecuteScript(script, nil, &header, scriptView)
	require.NoError(t, err)
}

func TestExecuteScripPanicsAreHandled(t *testing.T) {

	ctx := fvm.NewContext(zerolog.Nop())

	vm := &PanickingVM{}

	buffer := &bytes.Buffer{}
	log := zerolog.New(buffer)

	view := delta.NewView(func(_, _, _ string) (flow.RegisterValue, error) {
		return nil, nil
	})
	header := unittest.BlockHeaderFixture()

	manager, err := New(log, nil, nil, nil, nil, vm, ctx, DefaultProgramsCacheSize, committer.NewNoopViewCommitter(), scriptLogThreshold)
	require.NoError(t, err)

	_, err = manager.ExecuteScript([]byte("whatever"), nil, &header, view)

	require.Error(t, err)

	require.Contains(t, buffer.String(), "Verunsicherung")
}

func TestExecuteScript_LongScriptsAreLogged(t *testing.T) {

	ctx := fvm.NewContext(zerolog.Nop())

	vm := &LongRunningVM{duration: 2 * time.Millisecond}

	buffer := &bytes.Buffer{}
	log := zerolog.New(buffer)

	view := delta.NewView(func(_, _, _ string) (flow.RegisterValue, error) {
		return nil, nil
	})
	header := unittest.BlockHeaderFixture()

	manager, err := New(log, nil, nil, nil, nil, vm, ctx, DefaultProgramsCacheSize, committer.NewNoopViewCommitter(), 1*time.Millisecond)
	require.NoError(t, err)

	_, err = manager.ExecuteScript([]byte("whatever"), nil, &header, view)

	require.NoError(t, err)

	require.Contains(t, buffer.String(), "exceeded threshold")
}

func TestExecuteScript_ShortScriptsAreNotLogged(t *testing.T) {

	ctx := fvm.NewContext(zerolog.Nop())

	vm := &LongRunningVM{duration: 0}

	buffer := &bytes.Buffer{}
	log := zerolog.New(buffer)

	view := delta.NewView(func(_, _, _ string) (flow.RegisterValue, error) {
		return nil, nil
	})
	header := unittest.BlockHeaderFixture()

	manager, err := New(log, nil, nil, nil, nil, vm, ctx, DefaultProgramsCacheSize, committer.NewNoopViewCommitter(), 1*time.Second)
	require.NoError(t, err)

	_, err = manager.ExecuteScript([]byte("whatever"), nil, &header, view)

	require.NoError(t, err)

	require.NotContains(t, buffer.String(), "exceeded threshold")
}

type PanickingVM struct{}

func (p *PanickingVM) Run(f fvm.Context, procedure fvm.Procedure, view state.View, p2 *programs.Programs) error {
	panic("panic, but expected with sentinel for test: Verunsicherung ")
}

func (p *PanickingVM) GetAccount(f fvm.Context, address flow.Address, view state.View, p2 *programs.Programs) (*flow.Account, error) {
	panic("not expected")
}

type LongRunningVM struct {
	duration time.Duration
}

func (l *LongRunningVM) Run(f fvm.Context, procedure fvm.Procedure, view state.View, p2 *programs.Programs) error {
	time.Sleep(l.duration)
	// satisfy value marshaller
	if scriptProcedure, is := procedure.(*fvm.ScriptProcedure); is {
		scriptProcedure.Value = cadence.NewVoid()
	}

	return nil
}

func (l *LongRunningVM) GetAccount(f fvm.Context, address flow.Address, view state.View, p2 *programs.Programs) (*flow.Account, error) {
	panic("not expected")
}
