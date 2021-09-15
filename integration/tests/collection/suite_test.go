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

	ghostclient "github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	clusterstate "github.com/onflow/flow-go/state/cluster"
	clusterstateimpl "github.com/onflow/flow-go/state/cluster/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// the default timeout for individual actions (eg. send a transaction)
	defaultTimeout = 30 * time.Second
)

// Suite represents a test suite for collector nodes.
type Suite struct {
	suite.Suite
	common.TestnetStateTracker

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
	suite.Run(t, new(Suite))
}

// SetupTest generates a test network with the given number of collector nodes
// and clusters and starts the network.
//
// NOTE: This must be called explicitly by each test, since nodes/clusters vary
//       between test cases.
func (s *Suite) SetupTest(name string, nNodes, nClusters uint) {

	// default set of non-collector nodes
	var (
		conNode = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.ErrorLevel), testnet.AsGhost())
		exeNode = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.ErrorLevel), testnet.AsGhost())
		verNode = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.ErrorLevel), testnet.AsGhost())
	)
	colNodes := testnet.NewNodeConfigSet(nNodes, flow.RoleCollection, testnet.WithAdditionalFlag("--block-rate-delay=1ms"))

	s.nClusters = nClusters

	// set one of the non-collector nodes to be the ghost
	s.ghostID = conNode.Identifier

	// instantiate the network
	nodes := append(colNodes, conNode, exeNode, verNode)
	conf := testnet.NewNetworkConfig(name, nodes, testnet.WithClusters(nClusters))
	s.net = testnet.PrepareFlowNetwork(s.T(), conf)

	// start the network
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.net.Start(s.ctx)

	// create an account to use for sending transactions
	s.acct.addr, s.acct.key, s.acct.signer = common.GetAccount(s.net.Root().Header.ChainID.Chain())

	s.Track(s.T(), s.ctx, s.Ghost())
}

func (suite *Suite) TearDownTest() {
	// avoid nil pointer errors for skipped tests
	if suite.cancel != nil {
		defer suite.cancel()
	}
	if suite.net != nil {
		suite.net.Remove()
	}
}

// Ghost returns a client for the ghost node.
func (suite *Suite) Ghost() *ghostclient.GhostClient {
	ghost := suite.net.ContainerByID(suite.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(suite.T(), err, "could not get ghost client")
	return client
}

func (suite *Suite) Clusters() flow.ClusterList {
	result := suite.net.Result()
	setup, ok := result.ServiceEvents[0].Event.(*flow.EpochSetup)
	suite.Require().True(ok)

	collectors := suite.net.Identities().Filter(filter.HasRole(flow.RoleCollection))
	clusters, err := flow.NewClusterList(setup.Assignments, collectors)
	suite.Require().Nil(err)
	return clusters
}

func (suite *Suite) NextTransaction(opts ...func(*sdk.Transaction)) *sdk.Transaction {
	acct := suite.acct

	tx := sdk.NewTransaction().
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(convert.ToSDKID(suite.net.Root().ID())).
		SetProposalKey(acct.addr, acct.key.Index, acct.key.SequenceNumber).
		SetPayer(acct.addr).
		AddAuthorizer(acct.addr)

	for _, apply := range opts {
		apply(tx)
	}

	err := tx.SignEnvelope(acct.addr, acct.key.Index, acct.signer)
	require.Nil(suite.T(), err)

	suite.acct.key.SequenceNumber++

	return tx
}

//transaction ID (hash) is used to route a transaction to a cluster.
//The envelope signature is included in that hash, so this function:
//1) signs the envelope
//2) then computes the hash
//3) then checks whether it routes to the target cluster
func (suite *Suite) TxForCluster(target flow.IdentityList) *sdk.Transaction {
	acct := suite.acct

	tx := suite.NextTransaction()

	clusters := suite.Clusters()

	// hash-grind the script until the transaction will be routed to target cluster
	for {
		// ensure we always exit this loop with exactly 1 envelope signature - otherwise we would get a "duplicate signature for key" error
		tx.EnvelopeSignatures = nil
		tx.SetScript(append(tx.Script, '/', '/'))
		err := tx.SignEnvelope(acct.addr, acct.key.Index, acct.signer)
		require.Nil(suite.T(), err)
		routed, ok := clusters.ByTxID(convert.IDFromSDK(tx.ID()))
		require.True(suite.T(), ok)
		if routed.Fingerprint() == target.Fingerprint() {
			break
		}
	}

	return tx
}

// AwaitProposals waits to observe the given number of cluster block proposals
// and returns them.
func (suite *Suite) AwaitProposals(n uint) []cluster.Block {

	blocks := make([]cluster.Block, 0, n)
	suite.T().Logf("awaiting %d cluster blocks", n)

	waitFor := defaultTimeout + time.Duration(n)*2*time.Second
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

func (suite *Suite) AwaitTransactionsIncluded(txIDs ...flow.Identifier) {

	var (
		// for quickly looking up tx IDs
		lookup = make(map[flow.Identifier]struct{}, len(txIDs))
		// for keeping track of which transactions have been included in a finalized collection
		finalized = make(map[flow.Identifier]struct{}, len(txIDs))
		// for keeping track of proposals we've seen, and which transactions they contain
		proposals = make(map[flow.Identifier][]flow.Identifier)
		// in case we see a guarantee first
		guarantees = make(map[flow.Identifier]bool)
	)
	for _, txID := range txIDs {
		lookup[txID] = struct{}{}
	}

	suite.T().Logf("awaiting %d transactions included", len(txIDs))

	waitFor := defaultTimeout + time.Duration(len(lookup))*100*time.Millisecond
	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {

		_, msg, err := suite.reader.Next()
		require.Nil(suite.T(), err, "could not read next message")

		switch val := msg.(type) {
		case *messages.ClusterBlockProposal:
			header := val.Header
			collection := val.Payload.Collection
			suite.T().Logf("got proposal height=%d col_id=%x size=%d", header.Height, collection.ID(), collection.Len())
			if guarantees[collection.ID()] {
				for _, txID := range collection.Light().Transactions {
					finalized[txID] = struct{}{}
				}

				if len(finalized) == len(lookup) {
					return
				}
			} else {
				proposals[collection.ID()] = collection.Light().Transactions
			}

		case *flow.CollectionGuarantee:
			finalizedTxIDs, ok := proposals[val.CollectionID]
			if !ok {
				suite.T().Logf("got unseen guarantee (id=%x)", val.CollectionID)
				guarantees[val.CollectionID] = true
				continue
			} else {
				suite.T().Logf("got guarantee (id=%x)", val.CollectionID)
			}
			for _, txID := range finalizedTxIDs {
				finalized[txID] = struct{}{}
			}

			if len(finalized) == len(lookup) {
				return
			}

		case *flow.TransactionBody:
			suite.T().Log("got tx: ", val.ID())
		}
	}

	suite.T().Logf(
		"timed out waiting for inclusion (timeout=%s, finalized=%d, expected=%d)",
		waitFor.String(), len(finalized), len(lookup),
	)
	var missing []flow.Identifier
	for id := range lookup {
		if _, ok := finalized[id]; !ok {
			missing = append(missing, id)
		}
	}
	suite.T().Logf("missing: %v", missing)
	suite.T().FailNow()
}

// Collector returns the collector node with the given index in the
// given cluster.
func (suite *Suite) Collector(clusterIdx, nodeIdx uint) *testnet.Container {

	clusters := suite.Clusters()
	require.True(suite.T(), clusterIdx < uint(len(clusters)), "invalid cluster index")

	cluster, ok := clusters.ByIndex(clusterIdx)
	require.True(suite.T(), ok)
	node, ok := cluster.ByIndex(nodeIdx)
	require.True(suite.T(), ok, "invalid node index")

	return suite.net.ContainerByID(node.ID())
}

// ClusterStateFor returns a cluster state instance for the collector node
// with the given ID.
func (suite *Suite) ClusterStateFor(id flow.Identifier) *clusterstateimpl.State {

	myCluster, _, ok := suite.Clusters().ByNodeID(id)
	require.True(suite.T(), ok, "could not get node %s in clusters", id)

	setup, ok := suite.net.Result().ServiceEvents[0].Event.(*flow.EpochSetup)
	suite.Require().True(ok, "could not get root seal setup")
	rootBlock := clusterstate.CanonicalRootBlock(setup.Counter, myCluster)
	node := suite.net.ContainerByID(id)

	db, err := node.DB()
	require.Nil(suite.T(), err, "could not get node db")

	clusterStateRoot, err := clusterstateimpl.NewStateRoot(rootBlock)
	suite.NoError(err)
	clusterState, err := clusterstateimpl.Bootstrap(db, clusterStateRoot)
	require.NoError(suite.T(), err, "could not get cluster state")

	return clusterState
}
