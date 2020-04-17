package execution

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	badgerDB "github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	executionState "github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
	protocolBadger "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	// the timeout for individual actions (eg. send a transaction)
	defaultTimeout = 10 * time.Second
	// the period we wait to give consensus/routing time to complete
	waitTime = 20 * time.Second
)

// default set of non-execution nodes
func defaultOtherNodes() []testnet.NodeConfig {
	var (
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel))
		verNode  = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel))
	)

	return []testnet.NodeConfig{conNode1, conNode2, conNode3, verNode}
}

// TestExecutionStateSync tests that state that lives in another execution node is queried from that node.
// Approach: Stop or disconnect an execution node 2, send transactions to execution node 1, start execution node 2,
// verify state is synced
func TestExecutionStateSync(t *testing.T) {

	var (
		colNodeConfig  = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.InfoLevel))
		exeNodeConfigs = testnet.NewNodeConfigSet(2, flow.RoleExecution)
	)
	exeNodeConfig1 := exeNodeConfigs[0]
	// exeNodeConfig2 := exeNodeConfigs[1]

	nodes := append(append(exeNodeConfigs, colNodeConfig), defaultOtherNodes()...)
	conf := testnet.NewNetworkConfig("exe_state_sync_test", nodes)

	net, err := testnet.PrepareFlowNetwork(t, conf)
	require.Nil(t, err)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Cleanup()

	// we will send transaction to COL1
	colNode, ok := net.ContainerByID(colNodeConfig.Identifier)
	assert.True(t, ok)

	colClient, err := testnet.NewClient(colNode.Addr(testnet.ColNodeAPIPort))

	// we will test against EXE1
	exeNode1, ok := net.ContainerByID(exeNodeConfig1.Identifier)
	assert.True(t, ok)

	// exeNode2, ok := net.ContainerByID(exeNodeConfig2.Identifier)
	// assert.True(t, ok)

	tx := unittest.TransactionBodyFixture()
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	err = colClient.SignAndSendTransaction(ctx, tx)
	assert.Nil(t, err)

	// wait for consensus to complete
	//TODO we should listen for collection guarantees instead, but this is blocked
	// ref: https://github.com/dapperlabs/flow-go/issues/3021
	time.Sleep(waitTime)

	err = net.Stop()
	assert.Nil(t, err)

	stateView := getLatestExecutionStateView(t, exeNode1)
	res, err := stateView.Get(fullKeyHash(common.CounterOwner, common.CounterController, common.CounterKey))
	require.Nil(t, err)

	require.Equal(t, "TODO", res)
}

func getLatestExecutionStateView(t *testing.T, node *testnet.Container) *delta.View {
	// get database for EXE1
	db, err := node.DB()
	require.Nil(t, err)

	protocolState, err := protocolBadger.NewState(db)
	require.Nil(t, err)

	head, err := protocolState.Final().Head()
	require.Nil(t, err)

	ledgerStorage, err := node.ExecutionLedgerStorage()
	require.Nil(t, err)

	executionState := getExecutionState(ledgerStorage, db)

	stateCommitment, err := executionState.StateCommitmentByBlockID(head.ID())
	require.Nil(t, err)

	return executionState.NewView(stateCommitment)
}

func getExecutionState(ledgerStorage *ledger.TrieStorage, db *badgerDB.DB) executionState.ExecutionState {
	chunkDataPacks := badger.NewChunkDataPacks(db)
	executionResults := badger.NewExecutionResults(db)
	stateCommitments := badger.NewCommits(db)
	executionState := executionState.NewExecutionState(ledgerStorage, stateCommitments, chunkDataPacks,
		executionResults, db)
	return executionState
}

func fullKey(owner, controller, key string) string {
	return strings.Join([]string{owner, controller, key}, "__")
}

func fullKeyHash(owner, controller, key string) flow.RegisterID {
	h := sha256.New()
	_, _ = h.Write([]byte(fullKey(owner, controller, key)))
	return h.Sum(nil)
}
