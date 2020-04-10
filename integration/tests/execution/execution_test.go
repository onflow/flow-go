package execution

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	badgerDB "github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	executionState "github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
	protocolBadger "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const defaultTimeout = 10 * time.Second

// default set of non-execution nodes
func defaultOtherNodes() []testnet.NodeConfig {
	var (
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))
		verNode  = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel("info"))
	)

	return []testnet.NodeConfig{conNode1, conNode2, conNode3, verNode}
}

func TestExecutionNodes(t *testing.T) {

	var (
		colNode  = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel("info"))
		exeNode1 = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithIDInt(1))
		exeNode2 = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithIDInt(2))
	)

	nodes := append([]testnet.NodeConfig{colNode, exeNode1, exeNode2}, defaultOtherNodes()...)
	conf := testnet.NetworkConfig{Nodes: nodes}

	net, err := testnet.PrepareFlowNetwork(t, "exe_tests", conf)
	require.Nil(t, err)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Cleanup()

	// we will send transaction to COL1
	colContainer, ok := net.ContainerByID(colNode.Identifier)
	assert.True(t, ok)

	port, ok := colContainer.Ports[testnet.ColNodeAPIPort]
	assert.True(t, ok)

	client, err := testnet.NewClient(fmt.Sprintf(":%s", port))

	// we will test against EXE1
	exeContainer1, ok := net.ContainerByID(exeNode1.Identifier)
	assert.True(t, ok)

	t.Run("TODO", func(t *testing.T) {
		tx := unittest.TransactionBodyFixture()
		tx, err := client.SignTransaction(tx)
		assert.Nil(t, err)
		t.Log("sending transaction: ", tx.ID())

		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
		err = client.SendTransaction(ctx, tx)
		assert.Nil(t, err)

		// wait for consensus to complete
		//TODO we should listen for collection guarantees instead, but this is blocked
		// ref: https://github.com/dapperlabs/flow-go/issues/3021
		time.Sleep(10 * time.Second)

		// TODO stop then start containers
		err = net.StopContainers()
		assert.Nil(t, err)

		// get database for EXE1
		db, err := exeContainer1.DB()
		require.Nil(t, err)

		protocolState, err := protocolBadger.NewState(db)
		require.Nil(t, err)

		head, err := protocolState.Final().Head()
		require.Nil(t, err)

		ledgerStorage, err := exeContainer1.ExecutionLedgerStorage()
		require.Nil(t, err)

		executionState := getExecutionState(ledgerStorage, db)

		stateCommitment, err := executionState.StateCommitmentByBlockID(head.ID())
		require.Nil(t, err)

		stateView := executionState.NewView(stateCommitment)

		res, err := stateView.Get(fullKeyHash(common.CounterOwner, common.CounterController, common.CounterKey))
		require.Nil(t, err)

		require.Equal(t, "TODO", res)
	})
}

func getExecutionState(ledgerStorage *ledger.TrieStorage, db *badgerDB.DB) executionState.ExecutionState {
	chunkDataPacks := badger.NewChunkDataPacks(db)
	executionResults := badger.NewExecutionResults(db)
	stateCommitments := badger.NewCommits(db)
	executionState := executionState.NewExecutionState(ledgerStorage, stateCommitments, chunkDataPacks,
		executionResults)
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
