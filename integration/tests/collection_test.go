package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dapperlabs/testingdock"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clusterstate "github.com/dapperlabs/flow-go/cluster/badger"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/protocol"
)

func TestCollection(t *testing.T) {

	var (
		colNode1 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000001")
		})
		colNode2 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000002")
		})
		conNode = testnet.NewNodeConfig(flow.RoleConsensus)
		exeNode = testnet.NewNodeConfig(flow.RoleExecution)
		verNode = testnet.NewNodeConfig(flow.RoleVerification)
	)

	nodes := []*testnet.NodeConfig{colNode1, colNode2, conNode, exeNode, verNode}

	testingdock.Verbose = true

	ctx := context.Background()

	net, err := testnet.PrepareFlowNetwork(ctx, t, "col", nodes)
	require.Nil(t, err)

	net.Start(ctx)
	defer net.Cleanup()

	// get the collection node container
	colContainer, ok := net.ContainerByID(colNode1.Identifier)
	assert.True(t, ok)

	// get the node's ingress port and create an RPC client
	ingressPort, ok := colContainer.Ports[testnet.ColNodeAPIPort]
	assert.True(t, ok)

	key, err := generateRandomKey()
	assert.Nil(t, err)
	client, err := testnet.NewClient(fmt.Sprintf(":%s", ingressPort), key)
	assert.Nil(t, err)

	err = client.SendTransaction(ctx, noopTransaction())
	assert.Nil(t, err)

	// give the cluster a chance to do some consensus
	time.Sleep(10 * time.Second)
	err = net.StopContainers()
	assert.Nil(t, err)

	identities := net.Identities()

	// TODO check+set permissions
	{
		fmt.Println(colContainer.DataDir)
		stat, err := os.Stat(colContainer.DataDir)
		assert.Nil(t, err)
		fmt.Println(stat.Mode().String())

		err = os.Chown(colContainer.DataDir, os.Getuid(), os.Getgid())
		if err != nil {
			fmt.Println("chown err: ", err.Error())
		}

		stat, err = os.Stat(colContainer.DataDir)
		assert.Nil(t, err)
		fmt.Println(stat.Mode().String())

		err = filepath.Walk(colContainer.DataDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Println("walk err: ", err.Error())
			}
			fmt.Println("walk pre-chown: ", path, info.Mode().String(), info.Sys())
			err = os.Chown(path, os.Getuid(), os.Getgid())
			if err != nil {
				fmt.Println("walk chown err: ", err)
			}
			err = os.Chmod(path, 0777)
			if err != nil {
				fmt.Println("walk chmod err: ", err)
			}
			stat, err := os.Stat(path)
			if err != nil {
				fmt.Println("walk stat err: ", err)
				return nil
			}
			fmt.Println("walk post-chown: ", path, stat.Mode().String())
			return nil
		})
		assert.Nil(t, err)
	}

	// create a database
	chainID := protocol.ChainIDForCluster(identities.Filter(filter.HasRole(flow.RoleCollection)))
	db, err := badger.Open(badger.DefaultOptions(colContainer.DataDir).WithLogger(nil))
	require.Nil(t, err)

	state, err := clusterstate.NewState(db, chainID)
	assert.Nil(t, err)
	head, err := state.Final().Head()
	assert.Nil(t, err)

	// should be able to read a valid latest block
	assert.Equal(t, chainID, head.ChainID)
	// t.Fail()
}
