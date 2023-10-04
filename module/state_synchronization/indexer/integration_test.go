package indexer

import (
	"context"
	"os"
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

		badgerDB, dbDir := unittest.TempBadgerDB(t)
		t.Cleanup(func() {
			require.NoError(t, badgerDB.Close())
			require.NoError(t, os.RemoveAll(dbDir))
		})

		txScript := []byte(`transaction { prepare(auth: AuthAccount) { auth.save<String>("test", to: /storage/testPath) } }`)
		err = chain.execute(txScript)
		require.NoError(t, err)

		indexer := newIndexerTestWithBlocks(t, chain.blocks, 0)
		indexer.setBlockDataByID(chain.getExecutionDataByID)

		// todo we need to create indexer here instead use the method above because it needs to have a real indexer core
		// instance with real dbs instead of mocks, otherwise we don't cover that area
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
	exeData      map[flow.Identifier]*execution_data.BlockExecutionData
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
	data := make(map[flow.Identifier]*execution_data.BlockExecutionData)
	data[genesis.ID()] = nil

	return &testChain{
		ldg:          ldg,
		chain:        chain,
		fvm:          fvm.NewVirtualMachine(),
		fvmOpts:      opts,
		latestCommit: flow.StateCommitment(ldg.InitialState()),
		blocks:       []*flow.Block{&genesis},
		exeData:      data,
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

func (t *testChain) execute(script []byte) error {
	previousBlock := t.blocks[len(t.blocks)-1]
	newBlock := unittest.BlockWithParentFixture(previousBlock.Header)
	t.blocks = append(t.blocks, newBlock)

	txBody := flow.NewTransactionBody().
		SetScript(script).
		AddAuthorizer(t.chain.ServiceAddress())

	txProc := fvm.NewTransaction(txBody.ID(), 0, txBody)

	trieUpdate, err := t.fvmRun(txProc)
	if err != nil {
		return err
	}

	chunk := []*execution_data.ChunkExecutionData{{
		Events:             nil, // todo extract
		TrieUpdate:         trieUpdate,
		TransactionResults: nil, // todo extract
	}}

	t.exeData[newBlock.ID()] = &execution_data.BlockExecutionData{
		BlockID:             newBlock.Header.ID(),
		ChunkExecutionDatas: chunk,
	}

	return nil
}

func (t *testChain) getExecutionDataByID(ID flow.Identifier) (*execution_data.BlockExecutionDataEntity, bool) {
	data, ok := t.exeData[ID]
	if !ok {
		return nil, false
	}
	return execution_data.NewBlockExecutionDataEntity(ID, data), true
}
