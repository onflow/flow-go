package collection

import (
	"context"
	"testing"
	"time"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	ghostclient "github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/convert"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	clusterstate "github.com/dapperlabs/flow-go/state/cluster/badger"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	// the default timeout for individual actions (eg. send a transaction)
	defaultTimeout = 10 * time.Second
)

// CollectorSuite represents a test suite for collector nodes.
type CollectorSuite struct {
	suite.Suite

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net       *testnet.FlowNetwork
	nClusters uint

	// account info
	acct struct {
		key    *sdk.AccountKey
		addr   sdk.Address
		signer sdkcrypto.Signer
	}

	// ghost node
	ghostID flow.Identifier
	reader  *ghostclient.FlowMessageStreamReader
}

func TestCollectorSuite(t *testing.T) {
	suite.Run(t, new(CollectorSuite))
}

// SetupTest generates a test network with the given number of collector nodes
// and clusters and starts the network.
//
// NOTE: This must be called explicitly by each test, since nodes/clusters vary
//       between test cases.
func (suite *CollectorSuite) SetupTest(name string, nNodes, nClusters uint) {

	// default set of non-collector nodes
	var (
		conNode = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.ErrorLevel), testnet.AsGhost())
		exeNode = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.ErrorLevel), testnet.AsGhost())
		verNode = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.ErrorLevel), testnet.AsGhost())
	)
	colNodes := testnet.NewNodeConfigSet(nNodes, flow.RoleCollection)

	suite.nClusters = nClusters

	// set one of the non-collector nodes to be the ghost
	suite.ghostID = conNode.Identifier

	// instantiate the network
	nodes := append(colNodes, conNode, exeNode, verNode)
	conf := testnet.NewNetworkConfig(name, nodes, testnet.WithClusters(nClusters))
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	// start the network
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)

	// create an account to use for sending transactions
	suite.acct.addr, suite.acct.key, suite.acct.signer = common.GetAccount()

	// subscribe to the ghost
	for attempts := 0; ; attempts++ {
		reader, err := suite.Ghost().Subscribe(suite.ctx)
		if err == nil {
			suite.reader = reader
			break
		}
		if attempts >= 10 {
			require.NoError(suite.T(), err, "could not subscribe to ghost (%d attempts)", attempts)
		}
	}
}

func (suite *CollectorSuite) TearDownTest() {
	suite.net.Remove()
	suite.cancel()
}

// Ghost returns a client for the ghost node.
func (suite *CollectorSuite) Ghost() *ghostclient.GhostClient {
	ghost := suite.net.ContainerByID(suite.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(suite.T(), err, "could not get ghost client")
	return client
}

func (suite *CollectorSuite) Clusters() *flow.ClusterList {
	identities := suite.net.Identities()
	clusters := protocol.Clusters(suite.nClusters, identities)
	return clusters
}

func (suite *CollectorSuite) NextTransaction(opts ...func(*sdk.Transaction)) *sdk.Transaction {
	acct := suite.acct

	tx := sdk.NewTransaction().
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(sdk.BytesToID([]byte{1})).
		SetProposalKey(acct.addr, acct.key.ID, acct.key.SequenceNumber).
		SetPayer(acct.addr).
		AddAuthorizer(acct.addr)

	for _, apply := range opts {
		apply(tx)
	}

	err := tx.SignEnvelope(acct.addr, acct.key.ID, acct.signer)
	require.Nil(suite.T(), err)

	suite.acct.key.SequenceNumber++

	return tx
}

func (suite *CollectorSuite) TxForCluster(target flow.IdentityList) *sdk.Transaction {
	acct := suite.acct

	tx := sdk.NewTransaction().
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(convert.ToSDKID(unittest.IdentifierFixture())).
		SetProposalKey(acct.addr, acct.key.ID, acct.key.SequenceNumber).
		SetPayer(acct.addr).
		AddAuthorizer(acct.addr)

	clusters := suite.Clusters()

	// hash-grind the script until the transaction will be routed to target cluster
	for {
		tx.SetScript(append(tx.Script, '/', '/'))
		err := tx.SignEnvelope(sdk.RootAddress, acct.key.ID, acct.signer)
		require.Nil(suite.T(), err)
		routed := clusters.ByTxID(convert.IDFromSDK(tx.ID()))
		if routed.Fingerprint() == target.Fingerprint() {
			break
		}
	}

	return tx
}

// AwaitClusterBlocks waits to observe the given number of cluster blocks,
// and returns them.
func (suite *CollectorSuite) AwaitClusterBlocks(n uint) []cluster.Block {

	blocks := make([]cluster.Block, 0, n)

	// allow 5 seconds for each block
	waitFor := time.Duration(n) * time.Second * 5
	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {

		_, msg, err := suite.reader.Next()
		suite.Require().Nil(err, "could not read next message")
		suite.T().Logf("ghost recv: %T", msg)

		switch val := msg.(type) {
		case *messages.ClusterBlockProposal:
			block := cluster.Block{
				Header:  val.Header,
				Payload: val.Payload,
			}
			blocks = append(blocks, block)
			if len(blocks) == int(n) {
				return blocks
			}
		}
	}

	suite.T().Logf("timed out waiting for blocks (timeout=%s, saw=%d, expected=%d)", waitFor.String(), len(blocks), n)
	suite.T().FailNow()
	return nil
}

func (suite *CollectorSuite) AwaitTransactionsIncluded(txIDs ...flow.Identifier) {

	var (
		// for quickly looking up tx IDs
		lookup = make(map[flow.Identifier]struct{}, len(txIDs))
		// for keeping track of which transactions we've seen
		seen = make(map[flow.Identifier]struct{}, len(txIDs))
	)
	for _, txID := range txIDs {
		lookup[txID] = struct{}{}
	}

	// the height at which the collection is included
	height := uint64(0)

	waitFor := time.Second * 30
	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {

		_, msg, err := suite.reader.Next()
		require.Nil(suite.T(), err, "could not read next message")
		suite.T().Logf("ghost recv: %T", msg)

		switch val := msg.(type) {
		case *messages.ClusterBlockProposal:
			header := val.Header
			collection := val.Payload.Collection
			suite.T().Logf("got proposal height=%d col_id=%x size=%d", header.Height, collection.ID(), collection.Len())

			for _, txID := range collection.Light().Transactions {
				if _, exists := lookup[txID]; exists {
					seen[txID] = struct{}{}
					height = header.Height
				}
			}

			// once we have seen all the transactions, and 2 blocks have proposed
			// above the highest, we know they are incorporated (w/ Coldstuff)
			// TODO replace this with finalization by listening to guarantees
			if len(seen) == len(lookup) && header.Height-height >= 2 {
				return
			}

		case *flow.CollectionGuarantee:
			// TODO use this as indication of finalization w/ HotStuff
		}
	}

	suite.T().Logf("timed out waiting for inclusion (timeout=%s, saw=%d, expected=%d)", waitFor.String(), len(seen), len(lookup))
	suite.T().FailNow()
}

// Collector returns the collector node with the given index in the
// given cluster.
func (suite *CollectorSuite) Collector(clusterIdx, nodeIdx uint) *testnet.Container {

	clusters := suite.Clusters()
	require.True(suite.T(), clusterIdx < uint(clusters.Size()), "invalid cluster index")

	cluster := clusters.ByIndex(clusterIdx)
	node, ok := cluster.ByIndex(nodeIdx)
	require.True(suite.T(), ok, "invalid node index")

	return suite.net.ContainerByID(node.ID())
}

// ClusterStateFor returns a cluster state instance for the collector node
// with the given ID.
func (suite *CollectorSuite) ClusterStateFor(id flow.Identifier) *clusterstate.State {

	myCluster, ok := suite.Clusters().ByNodeID(id)
	require.True(suite.T(), ok, "could not get node %s in clusters", id)

	chainID := protocol.ChainIDForCluster(myCluster)
	node := suite.net.ContainerByID(id)

	db, err := node.DB()
	require.Nil(suite.T(), err, "could not get node db")

	state, err := clusterstate.NewState(db, chainID)
	require.Nil(suite.T(), err, "could not get cluster state")

	return state
}
