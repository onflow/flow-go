package collection

import (
	"context"
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

// CollectorSuite represents a test suite for collector nodes.
type CollectorSuite struct {
	suite.Suite

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	log zerolog.Logger

	net       *testnet.FlowNetwork
	nClusters uint

	serviceAccountIdx uint64

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

// SetupTest generates a test network with the given number of collector nodes
// and clusters and starts the network.
//
// NOTE: This must be called explicitly by each test, since nodes/clusters vary
//       between test cases.
func (suite *CollectorSuite) SetupTest(name string, nNodes, nClusters uint) {
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "suite.go").
		Str("testcase", suite.T().Name()).
		Logger()
	suite.log = logger
	suite.log.Info().Msgf("================> SetupTest")

	var (
		conNode = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
		// DKG require at least 2 consensus nodes
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
		exeNode  = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
		verNode  = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	)
	nodes := []testnet.NodeConfig{
		conNode,
		conNode2,
		exeNode,
		verNode,
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),
	}
	colNodes := testnet.NewNodeConfigSet(nNodes, flow.RoleCollection,
		testnet.WithLogLevel(zerolog.InfoLevel),
		testnet.WithAdditionalFlag("--block-rate-delay=1ms"),
	)

	suite.nClusters = nClusters

	// set consensus node as ghost node, since collection node
	// will send collection guarantees to consensus nodes.
	suite.ghostID = conNode.Identifier

	// instantiate the network
	nodes = append(nodes, colNodes...)
	conf := testnet.NewNetworkConfig(name, nodes, testnet.WithClusters(nClusters))
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	// start the network
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)

	// create an account to use for sending transactions
	suite.acct.addr, suite.acct.key, suite.acct.signer = common.GetAccount(suite.net.Root().Header.ChainID.Chain())
	suite.serviceAccountIdx = 2

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

func (s *CollectorSuite) TearDownTest() {
	s.log.Info().Msgf("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msgf("================> Finish TearDownTest")
}

// Ghost returns a client for the ghost node.
func (suite *CollectorSuite) Ghost() *ghostclient.GhostClient {
	ghost := suite.net.ContainerByID(suite.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(suite.T(), err, "could not get ghost client")
	return client
}

func (suite *CollectorSuite) Clusters() flow.ClusterList {
	result := suite.net.Result()
	setup, ok := result.ServiceEvents[0].Event.(*flow.EpochSetup)
	suite.Require().True(ok)

	collectors := suite.net.Identities().Filter(filter.HasRole(flow.RoleCollection))
	clusters, err := flow.NewClusterList(setup.Assignments, collectors)
	suite.Require().Nil(err)
	return clusters
}

func (suite *CollectorSuite) NextTransaction(opts ...func(*sdk.Transaction)) *sdk.Transaction {
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

func (suite *CollectorSuite) TxForCluster(target flow.IdentityList) *sdk.Transaction {
	acct := suite.acct

	tx := suite.NextTransaction()

	clusters := suite.Clusters()

	// hash-grind the script until the transaction will be routed to target cluster
	for {
		serviceAccountAddr, err := suite.net.Root().Header.ChainID.Chain().AddressAtIndex(suite.serviceAccountIdx)
		suite.Require().NoError(err)
		suite.serviceAccountIdx++
		tx.SetScript(append(tx.Script, '/', '/'))
		err = tx.SignEnvelope(sdk.Address(serviceAccountAddr), acct.key.Index, acct.signer)
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
func (suite *CollectorSuite) AwaitProposals(n uint) []cluster.Block {

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

func (suite *CollectorSuite) AwaitTransactionsIncluded(txIDs ...flow.Identifier) {

	var (
		// for quickly looking up tx IDs
		// if the tx has been finalized, we remove it from the lookup
		// we exit the loop when there is no more tx in the lookup
		lookup = make(map[flow.Identifier]struct{}, len(txIDs))
		// for keeping track of proposals we've seen, and which transactions they contain
		proposals = make(map[flow.Identifier][]flow.Identifier)
		// in case we see a guarantee first
		guarantees = make(map[flow.Identifier]bool)
	)

	if len(txIDs) == 0 {
		suite.T().Logf("no transaction to wait for")
		return
	}

	suite.T().Logf("waiting for %v transaction(s) to be included in a collection guarantee", len(txIDs))

	// we have sent transactions, we need to verify that they will be included in a
	// collection guarantee.
	// a collection guarantee is generated when a collection is finalized.
	for _, txID := range txIDs {
		lookup[txID] = struct{}{}
	}

	waitFor := defaultTimeout + time.Duration(len(lookup))*200*time.Millisecond
	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {

		originID, msg, err := suite.reader.Next()
		require.Nil(suite.T(), err, "could not read next message")

		switch val := msg.(type) {
		case *messages.ClusterBlockProposal:
			header := val.Header
			collection := val.Payload.Collection
			suite.T().Logf("got collection from %v height=%d col_id=%x size=%d", originID, header.Height, collection.ID(), collection.Len())
			if guarantees[collection.ID()] {
				for _, txID := range collection.Light().Transactions {
					delete(lookup, txID)
				}

				// we exit the loop when we found all the transactions are included in the collection guarantees
				suite.T().Logf("remaining transaction to wait for %v", len(lookup))

				if len(lookup) == 0 {
					suite.T().Logf("all transactions are included in collection guarantees !!")
					return
				}
			} else {
				// caching the proposal, so that when we receive the guarantee, we can finalized all the transactions
				// in it.
				suite.T().Logf("no guarantee for the received collection proposal %v, caching the collection proposal",
					collection.ID())
				proposals[collection.ID()] = collection.Light().Transactions
			}

		case *flow.CollectionGuarantee:
			finalizedTxIDs, ok := proposals[val.CollectionID]
			if !ok {
				suite.T().Logf("got guarantee from %v before the collection proposal (collection id=%x)", originID, val.CollectionID)
				guarantees[val.CollectionID] = true
				continue
			} else {
				suite.T().Logf("got guarantee from %v (id=%x)", originID, val.CollectionID)
			}
			for _, txID := range finalizedTxIDs {
				delete(lookup, txID)
			}

			suite.T().Logf("remaining transaction to wait for %v", len(lookup))
			if len(lookup) == 0 {
				suite.T().Logf("all transactions are included in collection guarantees !!")
				return
			}

		case *flow.TransactionBody:
			suite.T().Logf("got tx from %v: %v", originID, val.ID())
		}
	}

	suite.T().Logf(
		"timed out waiting for inclusion (timeout=%s, remaining=%d)",
		waitFor.String(), len(lookup),
	)
	var missing []flow.Identifier
	for id := range lookup {
		missing = append(missing, id)
	}
	suite.T().Fatalf("missing transactions: %v", missing)
}

// Collector returns the collector node with the given index in the
// given cluster.
func (suite *CollectorSuite) Collector(clusterIdx, nodeIdx uint) *testnet.Container {

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
func (suite *CollectorSuite) ClusterStateFor(id flow.Identifier) *clusterstateimpl.State {

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
	clusterState, err := clusterstateimpl.OpenState(db, nil, nil, nil, clusterStateRoot.ClusterID())
	require.NoError(suite.T(), err, "could not get cluster state")

	return clusterState
}
