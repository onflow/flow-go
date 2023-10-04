package indexer

import (
	"context"
	"path"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/metrics"
	pStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

func Test_Integration(t *testing.T) {
	const (
		fileName   = "integration-checkpoint"
		rootHeight = uint64(1)
	)

	log := zerolog.Nop()
	chain, err := newTestChain()
	require.NoError(t, err)

	tries, err := chain.bootstrap()
	require.NoError(t, err)

	unittest.RunWithTempDir(t, func(dir string) {
		err = wal.StoreCheckpointV6Concurrently([]*trie.MTrie{tries}, dir, fileName, log)
		require.NoErrorf(t, err, "fail to store checkpoint")

		checkpointFile := path.Join(dir, fileName)
		pb, dbDir := createPebbleForTest(t)

		bootstrap, err := pStorage.NewRegisterBootstrap(pb, checkpointFile, rootHeight, log)
		require.NoError(t, err)

		err = bootstrap.IndexCheckpointFile(context.Background())
		require.NoError(t, err)

		registers, err := pStorage.NewRegisters(pb)
		require.NoError(t, err)

		assert.Equal(t, registers.LatestHeight(), rootHeight)
		assert.Equal(t, registers.FirstHeight(), rootHeight)

		txScript := []byte(`transaction { prepare(auth: AuthAccount) { auth.save<String>("test", to: /storage/testPath) } }`)
		exeData, err := chain.execute(txScript)
		require.NoError(err)

	})

}

func createPebbleForTest(t *testing.T) (*pebble.DB, string) {
	dbDir := unittest.TempPebblePath(t)
	pb, err := pStorage.OpenRegisterPebbleDB(dbDir)
	require.NoError(t, err)
	return pb, dbDir
}

type testChain struct {
	ldg          *completeLedger.Ledger
	chain        flow.Chain
	fvm          *fvm.VirtualMachine
	fvmOpts      []fvm.Option
	latestCommit flow.StateCommitment
	blocks       []*flow.Block
}

func newTestChain() (*testChain, error) {
	ldg, err := completeLedger.NewLedger(
		&fixtures.NoopWAL{},
		100,
		&metrics.NoopCollector{},
		zerolog.Logger{},
		completeLedger.DefaultPathFinderVersion,
	)
	if err != nil {
		return nil, err
	}

	chain := flow.Emulator.Chain()
	opts := []fvm.Option{
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithTransactionFeesEnabled(false),
		fvm.WithMemoryAndInteractionLimitsDisabled(),
	}

	genesis := unittest.BlockFixture()

	return &testChain{
		ldg:          ldg,
		chain:        chain,
		fvm:          fvm.NewVirtualMachine(),
		fvmOpts:      opts,
		latestCommit: flow.StateCommitment(ldg.InitialState()),
		blocks:       []*flow.Block{&genesis},
	}, nil
}

func (t *testChain) newSnapshot() snapshot.StorageSnapshot {
	return state.NewLedgerStorageSnapshot(t.ldg, t.latestCommit)
}

func (t *testChain) fvmRun(proc fvm.Procedure) (*ledger.TrieUpdate, error) {
	ctx := fvm.NewContext(t.fvmOpts...)
	executionSnapshot, _, _ := t.fvm.Run(ctx, proc, t.newSnapshot())

	newCommit, updates, err := state.CommitDelta(
		t.ldg,
		executionSnapshot,
		t.latestCommit,
	)
	if err != nil {
		return nil, err
	}

	t.latestCommit = newCommit
	return updates, nil
}

func (t *testChain) bootstrap() (*trie.MTrie, error) {
	bootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	_, err := t.fvmRun(fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...))
	if err != nil {
		return nil, err
	}

	return t.ldg.FindTrieByStateCommit(t.latestCommit)
}

func (t *testChain) execute(script []byte) (*execution_data.BlockExecutionData, error) {
	previousBlock := t.blocks[len(t.blocks)-1]
	newBlock := unittest.BlockWithParentFixture(previousBlock.Header)
	t.blocks = append(t.blocks, newBlock)

	txBody := flow.NewTransactionBody().
		SetScript(script).
		AddAuthorizer(t.chain.ServiceAddress())

	txProc := fvm.NewTransaction(txBody.ID(), 0, txBody)

	trieUpdate, err := t.fvmRun(txProc)
	if err != nil {
		return nil, err
	}

	exeData := []*execution_data.ChunkExecutionData{{
		Collection:         nil,
		Events:             nil,
		TrieUpdate:         trieUpdate,
		TransactionResults: nil,
	}}

	return &execution_data.BlockExecutionData{
		BlockID:             newBlock.Header.ID(),
		ChunkExecutionDatas: exeData,
	}, nil
}
