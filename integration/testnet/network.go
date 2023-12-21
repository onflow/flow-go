package testnet

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	gonet "net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dapperlabs/testingdock"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/cmd/bootstrap/dkg"
	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	consensus_follower "github.com/onflow/flow-go/follower"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/insecure/cmd"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/cluster"
	dkgmod "github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/translator"
	clusterstate "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// TmpRoot is the default root directory to create temporary data
	// directories for containers. We use /tmp because $TMPDIR is not exposed
	// to docker by default on macOS
	TmpRoot = "/tmp"

	// integrationNamespace returns the temp directory pattern for the integration test
	integrationNamespace = "flow-integration-test"

	// DefaultBootstrapDir is the default directory for bootstrap files
	DefaultBootstrapDir = "/bootstrap"

	// DefaultFlowDataDir is the root of all database storage
	DefaultFlowDataDir = "/data"
	// DefaultFlowDBDir is the default directory for the node database.
	DefaultFlowDBDir = "/data/protocol"
	// DefaultFlowSecretsDBDir is the default directory for secrets database.
	DefaultFlowSecretsDBDir = "/data/secrets"
	// DefaultExecutionRootDir is the default directory for the execution node state database.
	DefaultExecutionRootDir = "/data/exedb"
	// DefaultRegisterDir is the default directory for the register store database.
	DefaultRegisterDir = "/data/register"
	// DefaultExecutionDataServiceDir for the execution data service blobstore.
	DefaultExecutionDataServiceDir = "/data/execution_data"
	// DefaultExecutionStateDir for the execution data service blobstore.
	DefaultExecutionStateDir = "/data/execution_state"
	// DefaultChunkDataPackDir for the chunk data packs
	DefaultChunkDataPackDir = "/data/chunk_data_pack"
	// DefaultProfilerDir is the default directory for the profiler
	DefaultProfilerDir = "/data/profiler"

	// GRPCPort is the GRPC API port.
	GRPCPort = "9000"
	// GRPCSecurePort is the secure GRPC API port.
	GRPCSecurePort = "9001"
	// GRPCWebPort is the access node GRPC-Web API (HTTP proxy) port.
	GRPCWebPort = "8000"
	// RESTPort is the access node REST API port.
	RESTPort = "8070"
	// MetricsPort is the metrics server port
	MetricsPort = "8080"
	// AdminPort is the admin server port
	AdminPort = "9002"
	// ExecutionStatePort is the execution state server port
	ExecutionStatePort = "9003"
	// PublicNetworkPort is the access node network port accessible from outside any docker container
	PublicNetworkPort = "9876"
	// DebuggerPort is the go debugger port
	DebuggerPort = "2345"

	// DefaultFlowPort default gossip network port
	DefaultFlowPort = 2137

	// PrimaryAN is the container name for the primary access node to use for API requests
	PrimaryAN = "access_1"

	DefaultViewsInStakingAuction uint64 = 5
	DefaultViewsInDKGPhase       uint64 = 50
	DefaultViewsInEpoch          uint64 = 180

	// DefaultMinimumNumOfAccessNodeIDS at-least 1 AN ID must be configured for LN & SN
	DefaultMinimumNumOfAccessNodeIDS = 1

	// defaultPeerUpdateInterval determines the time interval at which each node updates its pubsub connections.
	// Testnet is running at a significantly smaller scale than a production network, hence to
	// hold the mathematics behind topology connectivity valid, we need a faster connection update rate than the default value:
	defaultPeerUpdateInterval = 1 * time.Second
)

func init() {
	testingdock.Verbose = true
}

// FlowNetwork represents a test network of Flow nodes running in Docker containers.
type FlowNetwork struct {
	t                    *testing.T
	log                  zerolog.Logger
	suite                *testingdock.Suite
	config               NetworkConfig
	cli                  *dockerclient.Client
	network              *testingdock.Network
	Containers           map[string]*Container
	ConsensusFollowers   map[flow.Identifier]consensus_follower.ConsensusFollower
	CorruptedPortMapping map[flow.Identifier]string // port binding for corrupted containers.
	root                 *flow.Block
	result               *flow.ExecutionResult
	seal                 *flow.Seal

	// baseTempdir is the root directory for all temporary data used within a test network.
	baseTempdir string

	BootstrapDir      string
	BootstrapSnapshot *inmem.Snapshot
	BootstrapData     *BootstrapData
}

// CorruptedIdentities returns the identities of corrupted nodes in testnet (for BFT testing).
func (net *FlowNetwork) CorruptedIdentities() flow.IdentityList {
	// lists up the corrupted identifiers
	corruptedIdentifiers := flow.IdentifierList{}
	for _, c := range net.config.Nodes {
		if c.Corrupted {
			corruptedIdentifiers = append(corruptedIdentifiers, c.Identifier)
		}
	}

	// extracts corrupted identities to corrupted identifiers
	corruptedIdentities := flow.IdentityList{}
	for _, c := range net.Containers {
		if corruptedIdentifiers.Contains(c.Config.Identity().NodeID) {
			corruptedIdentities = append(corruptedIdentities, c.Config.Identity())
		}
	}
	return corruptedIdentities
}

// Identities returns a list of identities, one for each node in the network.
func (net *FlowNetwork) Identities() flow.IdentityList {
	il := make(flow.IdentityList, 0, len(net.Containers))
	for _, c := range net.Containers {
		il = append(il, c.Config.Identity())
	}
	return il
}

// ContainersByRole returns all the containers in the network with the specified role
func (net *FlowNetwork) ContainersByRole(role flow.Role) []*Container {
	cl := make([]*Container, 0, len(net.Containers))
	for _, c := range net.Containers {
		if c.Config.Role == role {
			cl = append(cl, c)
		}
	}
	return cl
}

// Root returns the root block generated for the network.
func (net *FlowNetwork) Root() *flow.Block {
	return net.root
}

// Seal returns the root block seal generated for the network.
func (net *FlowNetwork) Seal() *flow.Seal {
	return net.seal
}

// Result returns the root block result generated for the network.
func (net *FlowNetwork) Result() *flow.ExecutionResult {
	return net.result
}

// Start starts the network.
func (net *FlowNetwork) Start(ctx context.Context) {
	// makes it easier to see logs for a specific test case
	net.log.Info().Msgf("starting flow network %v with docker containers",
		net.config.Name)

	// print all existing containers before starting our containers, Useful to debug the issue
	// that the tests fail due to "port is already allocated"

	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	require.NoError(net.t, err)

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	require.NoError(net.t, err)

	t := net.t
	t.Logf("%v (%v) before starting flow network, found %d docker containers", time.Now().UTC(), t.Name(), len(containers))

	for _, container := range containers {
		t.Logf("%v (%v) before starting flow network, found docker container %v with ports %v", time.Now().UTC(), t.Name(), container.Names, container.Ports)
	}

	t.Log("starting flow network")
	net.suite.Start(ctx)

	containers, err = cli.ContainerList(ctx, types.ContainerListOptions{})
	require.NoError(net.t, err)

	t.Logf("%v (%v) after starting flow network, found %d docker containers", time.Now().UTC(), t.Name(), len(containers))
	for _, container := range containers {
		t.Logf("%v (%v) after starting flow network, found docker container: %v %v", time.Now().UTC(), t.Name(), container.Names, container.Ports)
	}
}

// Remove stops the network, removes all the containers and cleans up all resources.
// If you need to inspect state, first `Stop` the containers, then check state, then `Cleanup` resources.
// If you need to restart containers, use `Stop` instead, which does not remove containers.
func (net *FlowNetwork) Remove() {
	net.StopContainers()
	net.RemoveContainers()
	net.Cleanup()
}

// StopContainers stops all containers in the network, without removing them. This allows containers to be
// restarted. To remove them, call RemoveContainers.
func (net *FlowNetwork) StopContainers() {
	if net == nil || net.suite == nil {
		return
	}

	err := net.suite.Close()
	assert.NoError(net.t, err)
}

// RemoveContainers removes all the containers in the network. Containers need to be stopped first using StopContainers.
func (net *FlowNetwork) RemoveContainers() {
	if net == nil || net.suite == nil {
		return
	}

	err := net.suite.Remove()
	assert.NoError(net.t, err)
}

// DropDBs resets the protocol state database for all containers in the network
// matching the given filter.
func (net *FlowNetwork) DropDBs(filter flow.IdentityFilter[flow.Identity]) {
	if net == nil || net.suite == nil {
		return
	}
	// clear data directories
	for _, c := range net.Containers {
		if !filter(c.Config.Identity()) {
			continue
		}
		c.DropDB()
	}
}

// Cleanup cleans up all temporary files used by the network.
func (net *FlowNetwork) Cleanup() {
	if net == nil || net.suite == nil {
		return
	}
	// remove data directories
	for _, c := range net.Containers {
		err := os.RemoveAll(c.datadir)
		assert.NoError(net.t, err)
	}
}

// ContainerByID returns the container with the given node ID, if it exists.
// Otherwise fails the test.
func (net *FlowNetwork) ContainerByID(id flow.Identifier) *Container {
	for _, c := range net.Containers {
		if c.Config.NodeID == id {
			return c
		}
	}
	net.t.FailNow()
	return nil
}

// ConsensusFollowerByID returns the ConsensusFollower with the given node ID, if it exists.
// Otherwise fails the test.
func (net *FlowNetwork) ConsensusFollowerByID(id flow.Identifier) consensus_follower.ConsensusFollower {
	follower, ok := net.ConsensusFollowers[id]
	require.True(net.t, ok)
	return follower
}

// ContainerByName returns the container with the given name, if it exists.
// Otherwise fails the test.
func (net *FlowNetwork) ContainerByName(name string) *Container {
	container, exists := net.Containers[name]
	require.True(net.t, exists, "container %s does not exists", name)
	return container
}

func (net *FlowNetwork) PrintPorts() {
	var builder strings.Builder
	builder.WriteString("endpoints by container name:\n")
	for cName, c := range net.Containers {
		builder.WriteString(fmt.Sprintf("\t%s\n", cName))
		for portName, port := range c.Ports {
			switch portName {
			case MetricsPort:
				builder.WriteString(fmt.Sprintf("\t\t%s: localhost:%s/metrics\n", portName, port))
			default:
				builder.WriteString(fmt.Sprintf("\t\t%s: localhost:%s\n", portName, port))
			}
		}
	}
	fmt.Print(builder.String())
}

// PortsByContainerName returns the specified port for each container in the network.
// Args:
//   - portName: name of the port.
//   - withGhost: when set to true will include urls's for ghost containers, otherwise ghost containers will be filtered.
//
// Returns:
//   - map[string]string: a map of container name to the specified port on the host machine.
func (net *FlowNetwork) PortsByContainerName(portName string, withGhost bool) map[string]string {
	portsByContainer := make(map[string]string)
	for cName, c := range net.Containers {
		if !withGhost && c.Config.Ghost {
			continue
		}
		portsByContainer[cName] = c.Ports[portName]
	}
	return portsByContainer
}

// GetMetricFromContainers returns the specified metric for all containers.
// Args:
//
//		t: testing pointer
//		metricName: name of the metric
//	 metricsURLs: map of container name to metrics url
//
// Returns:
//
//	map[string][]*io_prometheus_client.Metric map of container name to metric result.
func (net *FlowNetwork) GetMetricFromContainers(t *testing.T, metricName string, metricsURLs map[string]string) map[string][]*io_prometheus_client.Metric {
	allMetrics := make(map[string][]*io_prometheus_client.Metric, len(metricsURLs))
	for containerName, metricsURL := range metricsURLs {
		allMetrics[containerName] = net.GetMetricFromContainer(t, containerName, metricsURL, metricName)
	}
	return allMetrics
}

// GetMetricFromContainer makes an HTTP GET request to the metrics url and returns the metric families for each container.
func (net *FlowNetwork) GetMetricFromContainer(t *testing.T, containerName, metricsURL, metricName string) []*io_prometheus_client.Metric {
	// download root snapshot from provided URL
	res, err := http.Get(metricsURL)
	require.NoError(t, err, fmt.Sprintf("failed to get metrics for container %s at url %s: %s", containerName, metricsURL, err))
	defer res.Body.Close()

	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(res.Body)
	require.NoError(t, err, fmt.Sprintf("failed to parse metrics for container %s at url %s: %s", containerName, metricsURL, err))
	m, ok := mf[metricName]
	require.True(t, ok, "failed to get metric %s for container %s at url %s metric does not exist", metricName, containerName, metricsURL)
	return m.GetMetric()
}

type ConsensusFollowerConfig struct {
	NodeID            flow.Identifier
	NetworkingPrivKey crypto.PrivateKey
	StakedNodeID      flow.Identifier
	Opts              []consensus_follower.Option
}

func NewConsensusFollowerConfig(t *testing.T, networkingPrivKey crypto.PrivateKey, stakedNodeID flow.Identifier, opts ...consensus_follower.Option) ConsensusFollowerConfig {
	pid, err := keyutils.PeerIDFromFlowPublicKey(networkingPrivKey.PublicKey())
	assert.NoError(t, err)
	nodeID, err := translator.NewPublicNetworkIDTranslator().GetFlowID(pid)
	assert.NoError(t, err)
	return ConsensusFollowerConfig{
		NetworkingPrivKey: networkingPrivKey,
		StakedNodeID:      stakedNodeID,
		NodeID:            nodeID,
		Opts:              opts,
	}
}

// NetworkConfig is the config for the network.
type NetworkConfig struct {
	Nodes                      NodeConfigs
	ConsensusFollowers         []ConsensusFollowerConfig
	Observers                  []ObserverConfig
	Name                       string
	NClusters                  uint
	ViewsInDKGPhase            uint64
	ViewsInStakingAuction      uint64
	ViewsInEpoch               uint64
	EpochCommitSafetyThreshold uint64
}

type NetworkConfigOpt func(*NetworkConfig)

func NewNetworkConfig(name string, nodes NodeConfigs, opts ...NetworkConfigOpt) NetworkConfig {
	c := NetworkConfig{
		Nodes:                 nodes,
		Name:                  name,
		NClusters:             1, // default to 1 cluster
		ViewsInStakingAuction: DefaultViewsInStakingAuction,
		ViewsInDKGPhase:       DefaultViewsInDKGPhase,
		ViewsInEpoch:          DefaultViewsInEpoch,
	}

	for _, apply := range opts {
		apply(&c)
	}

	return c
}

func NewNetworkConfigWithEpochConfig(name string, nodes NodeConfigs, viewsInStakingAuction, viewsInDKGPhase, viewsInEpoch, safetyThreshold uint64, opts ...NetworkConfigOpt) NetworkConfig {
	c := NetworkConfig{
		Nodes:                      nodes,
		Name:                       name,
		NClusters:                  1, // default to 1 cluster
		ViewsInStakingAuction:      viewsInStakingAuction,
		ViewsInDKGPhase:            viewsInDKGPhase,
		ViewsInEpoch:               viewsInEpoch,
		EpochCommitSafetyThreshold: safetyThreshold,
	}

	for _, apply := range opts {
		apply(&c)
	}

	return c
}

func WithViewsInStakingAuction(views uint64) func(*NetworkConfig) {
	return func(config *NetworkConfig) {
		config.ViewsInStakingAuction = views
	}
}

func WithViewsInEpoch(views uint64) func(*NetworkConfig) {
	return func(config *NetworkConfig) {
		config.ViewsInEpoch = views
	}
}

func WithViewsInDKGPhase(views uint64) func(*NetworkConfig) {
	return func(config *NetworkConfig) {
		config.ViewsInDKGPhase = views
	}
}

func WithEpochCommitSafetyThreshold(threshold uint64) func(*NetworkConfig) {
	return func(config *NetworkConfig) {
		config.EpochCommitSafetyThreshold = threshold
	}
}

func WithClusters(n uint) func(*NetworkConfig) {
	return func(conf *NetworkConfig) {
		conf.NClusters = n
	}
}

func WithObservers(observers ...ObserverConfig) func(*NetworkConfig) {
	return func(conf *NetworkConfig) {
		conf.Observers = observers
	}
}

func WithConsensusFollowers(followers ...ConsensusFollowerConfig) func(*NetworkConfig) {
	return func(conf *NetworkConfig) {
		conf.ConsensusFollowers = followers
	}
}

func (n *NetworkConfig) Len() int {
	return len(n.Nodes)
}

func (n *NetworkConfig) Less(i, j int) bool {
	// Always move execution to the front
	if n.Nodes[i].Role == n.Nodes[j].Role {
		return false
	} else if n.Nodes[j].Role == flow.RoleExecution {
		return false
	} else if n.Nodes[i].Role == flow.RoleExecution {
		return true
	}
	return n.Nodes[i].Role < n.Nodes[j].Role
}

func (n *NetworkConfig) Swap(i, j int) {
	n.Nodes[i], n.Nodes[j] = n.Nodes[j], n.Nodes[i]
}

func PrepareFlowNetwork(t *testing.T, networkConf NetworkConfig, chainID flow.ChainID) *FlowNetwork {
	// number of nodes
	nNodes := len(networkConf.Nodes)

	t.Logf("%v (%v) preparing flow network with %v nodes", time.Now().UTC(), t.Name(), nNodes)

	require.NotZero(t, len(networkConf.Nodes), "must specify at least one node")

	// Sort so that access nodes start up last
	sort.Sort(&networkConf)

	// set up docker client
	dockerClient, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	require.NoError(t, err)

	suite, _ := testingdock.GetOrCreateSuite(t, networkConf.Name, testingdock.SuiteOpts{
		Client: dockerClient,
	})
	network := suite.Network(testingdock.NetworkOpts{
		Name: networkConf.Name,
	})

	// create a temporary directory to store all bootstrapping files
	baseTempdir := makeTempDir(t, integrationNamespace)
	bootstrapDir := makeDir(t, baseTempdir, "bootstrap")

	t.Logf("Base Tempdir: %s \n", baseTempdir)
	t.Logf("BootstrapDir: %s \n", bootstrapDir)

	bootstrapData, err := BootstrapNetwork(networkConf, bootstrapDir, chainID)
	require.Nil(t, err)

	root := bootstrapData.Root
	result := bootstrapData.Result
	seal := bootstrapData.Seal
	confs := bootstrapData.StakedConfs
	bootstrapSnapshot := bootstrapData.Snapshot

	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("module", "flownetwork").
		Str("testcase", t.Name()).
		Logger()

	flowNetwork := &FlowNetwork{
		t:                    t,
		cli:                  dockerClient,
		config:               networkConf,
		suite:                suite,
		network:              network,
		log:                  logger,
		Containers:           make(map[string]*Container, nNodes),
		ConsensusFollowers:   make(map[flow.Identifier]consensus_follower.ConsensusFollower, len(networkConf.ConsensusFollowers)),
		CorruptedPortMapping: make(map[flow.Identifier]string),
		root:                 root,
		seal:                 seal,
		result:               result,
		baseTempdir:          baseTempdir,
		BootstrapDir:         bootstrapDir,
		BootstrapSnapshot:    bootstrapSnapshot,
		BootstrapData:        bootstrapData,
	}

	// check that at-least 2 full access nodes must be configured in your test suite
	// in order to provide a secure GRPC connection for LN & SN nodes
	accessNodeIDS := make([]string, 0)
	for _, n := range confs {
		if n.Role == flow.RoleAccess && !n.Ghost {
			accessNodeIDS = append(accessNodeIDS, n.NodeID.String())
		}
	}
	require.GreaterOrEqualf(t, len(accessNodeIDS), DefaultMinimumNumOfAccessNodeIDS,
		fmt.Sprintf("at least %d access nodes that are not a ghost must be configured for test suite", DefaultMinimumNumOfAccessNodeIDS))

	for _, nodeConf := range confs {
		var nodeType = "real"
		if nodeConf.Ghost {
			nodeType = "ghost"
		}

		t.Logf("add docker container type: %s, %v, node id: %v, address: %v", nodeType, nodeConf.ContainerName, nodeConf.NodeID, nodeConf.Address)
		err = flowNetwork.AddNode(t, bootstrapDir, nodeConf)
		require.NoError(t, err)

		// ghost nodes will not need any flags
		if nodeConf.Ghost {
			continue
		}

		// if node is of LN/SN role type add additional flags to node container for secure GRPC connection
		// this is required otherwise collection node will fail to startup
		if nodeConf.Role == flow.RoleCollection || nodeConf.Role == flow.RoleConsensus {
			nodeContainer := flowNetwork.Containers[nodeConf.ContainerName]
			nodeContainer.AddFlag("insecure-access-api", "false")
			nodeContainer.AddFlag("access-node-ids", strings.Join(accessNodeIDS, ","))
		}
	}

	for i, observerConf := range networkConf.Observers {
		if observerConf.ContainerName == "" {
			observerConf.ContainerName = fmt.Sprintf("observer_%d", i+1)
		}
		t.Logf("add observer %v", observerConf.ContainerName)
		flowNetwork.addObserver(t, observerConf)
	}

	rootProtocolSnapshotPath := filepath.Join(bootstrapDir, bootstrap.PathRootProtocolStateSnapshot)

	// add each follower to the network
	for _, followerConf := range networkConf.ConsensusFollowers {
		t.Logf("add consensus follower %v", followerConf.NodeID)
		flowNetwork.addConsensusFollower(t, rootProtocolSnapshotPath, followerConf, confs)
	}

	t.Logf("%v finish preparing flow network for %v", time.Now().UTC(), t.Name())

	return flowNetwork
}

func (net *FlowNetwork) addConsensusFollower(t *testing.T, rootProtocolSnapshotPath string, followerConf ConsensusFollowerConfig, containers []ContainerConfig) {
	tmpdir := makeTempSubDir(t, net.baseTempdir, "flow-consensus-follower")

	// create a directory for the follower database
	dataDir := makeDir(t, tmpdir, DefaultFlowDBDir)

	// create a follower-specific directory for the bootstrap files
	followerBootstrapDir := makeDir(t, tmpdir, DefaultBootstrapDir)
	makeDir(t, followerBootstrapDir, bootstrap.DirnamePublicBootstrap)

	// copy root protocol snapshot to the follower-specific folder
	// bootstrap/public-root-information directory
	err := io.Copy(rootProtocolSnapshotPath, filepath.Join(followerBootstrapDir, bootstrap.PathRootProtocolStateSnapshot))
	require.NoError(t, err)

	// consensus follower
	bindAddr := gonet.JoinHostPort("localhost", testingdock.RandomPort(t))
	opts := append(
		followerConf.Opts,
		consensus_follower.WithDataDir(dataDir),
		consensus_follower.WithBootstrapDir(followerBootstrapDir),
	)

	stakedANContainer := net.ContainerByID(followerConf.StakedNodeID)
	require.NotNil(t, stakedANContainer, "unable to find staked AN for the follower engine %s", followerConf.NodeID.String())

	// capture the public network port as an uint
	// the consensus follower runs within the test suite, and does not have access to the internal docker network.
	portStr := stakedANContainer.Port(PublicNetworkPort)
	port, err := strconv.ParseUint(portStr, 10, 32)
	require.NoError(t, err)

	bootstrapNodeInfo := consensus_follower.BootstrapNodeInfo{
		Host:             "localhost",
		Port:             uint(port),
		NetworkPublicKey: stakedANContainer.Config.NetworkPubKey(),
	}

	// it should be able to figure out the rest on its own.
	follower, err := consensus_follower.NewConsensusFollower(followerConf.NetworkingPrivKey, bindAddr,
		[]consensus_follower.BootstrapNodeInfo{bootstrapNodeInfo}, opts...)
	require.NoError(t, err)

	net.ConsensusFollowers[followerConf.NodeID] = follower
}

func (net *FlowNetwork) StopContainerByName(ctx context.Context, containerName string) error {
	container := net.ContainerByName(containerName)
	if container == nil {
		return fmt.Errorf("%s container not found", containerName)
	}
	return net.cli.ContainerStop(ctx, container.ID, dockercontainer.StopOptions{})
}

type ObserverConfig struct {
	ContainerName       string
	LogLevel            zerolog.Level
	AdditionalFlags     []string
	BootstrapAccessName string
}

func (net *FlowNetwork) addObserver(t *testing.T, conf ObserverConfig) {
	if conf.BootstrapAccessName == "" {
		conf.BootstrapAccessName = PrimaryAN
	}

	// Setup directories
	tmpdir := makeTempSubDir(t, net.baseTempdir, fmt.Sprintf("flow-node-%s-", conf.ContainerName))

	nodeBootstrapDir := makeDir(t, tmpdir, DefaultBootstrapDir)
	flowDataDir := makeDir(t, tmpdir, DefaultFlowDataDir)
	_ = makeDir(t, tmpdir, DefaultProfilerDir)

	err := io.CopyDirectory(net.BootstrapDir, nodeBootstrapDir)
	require.NoError(t, err)

	// Find the public key for the access node
	accessNode := net.ContainerByName(conf.BootstrapAccessName)
	accessPublicKey := hex.EncodeToString(accessNode.Config.NetworkPubKey().Encode())
	require.NotEmptyf(t, accessPublicKey, "failed to find the staked conf for access node with container name '%s'", conf.BootstrapAccessName)

	err = WriteObserverPrivateKey(conf.ContainerName, nodeBootstrapDir)
	require.NoError(t, err)

	containerOpts := testingdock.ContainerOpts{
		ForcePull: false,
		Name:      conf.ContainerName,
		Config: &container.Config{
			Image: "gcr.io/flow-container-registry/observer:latest",
			User:  currentUser(),
			Cmd: append([]string{
				"--bind=0.0.0.0:0",
				fmt.Sprintf("--bootstrapdir=%s", DefaultBootstrapDir),
				fmt.Sprintf("--datadir=%s", DefaultFlowDBDir),
				fmt.Sprintf("--secretsdir=%s", DefaultFlowSecretsDBDir),
				fmt.Sprintf("--profiler-dir=%s", DefaultProfilerDir),
				fmt.Sprintf("--loglevel=%s", conf.LogLevel.String()),
				fmt.Sprintf("--bootstrap-node-addresses=%s", accessNode.ContainerAddr(PublicNetworkPort)),
				fmt.Sprintf("--bootstrap-node-public-keys=%s", accessPublicKey),
				fmt.Sprintf("--upstream-node-addresses=%s", accessNode.ContainerAddr(GRPCSecurePort)),
				fmt.Sprintf("--upstream-node-public-keys=%s", accessPublicKey),
				fmt.Sprintf("--observer-networking-key-path=%s/private-root-information/%s_key", DefaultBootstrapDir, conf.ContainerName),
			}, conf.AdditionalFlags...),
		},
		HostConfig: &container.HostConfig{
			Binds: []string{
				fmt.Sprintf("%s:%s:rw", flowDataDir, DefaultFlowDataDir),
				fmt.Sprintf("%s:%s:ro", nodeBootstrapDir, DefaultBootstrapDir),
			},
		},
	}

	nodeContainer := &Container{
		Ports:   make(map[string]string),
		datadir: tmpdir,
		net:     net,
		opts:    &containerOpts,
	}

	nodeContainer.exposePort(GRPCPort, testingdock.RandomPort(t))
	nodeContainer.AddFlag("rpc-addr", nodeContainer.ContainerAddr(GRPCPort))

	nodeContainer.exposePort(GRPCSecurePort, testingdock.RandomPort(t))
	nodeContainer.AddFlag("secure-rpc-addr", nodeContainer.ContainerAddr(GRPCSecurePort))

	nodeContainer.exposePort(GRPCWebPort, testingdock.RandomPort(t))
	nodeContainer.AddFlag("http-addr", nodeContainer.ContainerAddr(GRPCWebPort))

	nodeContainer.exposePort(AdminPort, testingdock.RandomPort(t))
	nodeContainer.AddFlag("admin-addr", nodeContainer.ContainerAddr(AdminPort))

	nodeContainer.exposePort(RESTPort, testingdock.RandomPort(t))
	nodeContainer.AddFlag("rest-addr", nodeContainer.ContainerAddr(RESTPort))

	nodeContainer.opts.HealthCheck = testingdock.HealthCheckCustom(nodeContainer.HealthcheckCallback())

	suiteContainer := net.suite.Container(containerOpts)
	nodeContainer.Container = suiteContainer
	net.Containers[nodeContainer.Name()] = nodeContainer

	// start after the bootstrap access node
	accessNode.After(suiteContainer)
}

// AddNode creates a node container with the given config and adds it to the
// network.
func (net *FlowNetwork) AddNode(t *testing.T, bootstrapDir string, nodeConf ContainerConfig) error {
	opts := &testingdock.ContainerOpts{
		ForcePull: false,
		Name:      nodeConf.ContainerName,
		Config: &container.Config{
			Image: nodeConf.ImageName(),
			User:  currentUser(),
			Cmd: append([]string{
				fmt.Sprintf("--peerupdate-interval=%s", defaultPeerUpdateInterval.String()),
				fmt.Sprintf("--nodeid=%s", nodeConf.NodeID.String()),
				fmt.Sprintf("--bootstrapdir=%s", DefaultBootstrapDir),
				fmt.Sprintf("--datadir=%s", DefaultFlowDBDir),
				fmt.Sprintf("--profiler-dir=%s", DefaultProfilerDir),
				fmt.Sprintf("--secretsdir=%s", DefaultFlowSecretsDBDir),
				fmt.Sprintf("--loglevel=%s", nodeConf.LogLevel.String()),
				fmt.Sprintf("--herocache-metrics-collector=%t", true), // to cache integration issues with this collector (if any)
			}, nodeConf.AdditionalFlags...),
		},
		HostConfig: &container.HostConfig{},
	}

	tmpdir := makeTempSubDir(t, net.baseTempdir, fmt.Sprintf("flow-node-%s-", nodeConf.ContainerName))

	t.Logf("%v adding container %v for %v node", time.Now().UTC(), nodeConf.ContainerName, nodeConf.Role)

	nodeContainer := &Container{
		Config:  nodeConf,
		Ports:   make(map[string]string),
		datadir: tmpdir,
		net:     net,
		opts:    opts,
	}

	// create a directory for the node database
	flowDataDir := makeDir(t, tmpdir, DefaultFlowDataDir)

	// create the profiler dir for the node
	flowProfilerDir := makeDir(t, tmpdir, DefaultProfilerDir)
	t.Logf("create profiler dir: %v", flowProfilerDir)

	// create a directory for the bootstrap files
	// we create a node-specific bootstrap directory to enable testing nodes
	// bootstrapping from different root state snapshots and epochs
	nodeBootstrapDir := makeDir(t, tmpdir, DefaultBootstrapDir)

	// copy bootstrap files to node-specific bootstrap directory
	err := io.CopyDirectory(bootstrapDir, nodeBootstrapDir)
	require.NoError(t, err)

	// Bind the host directory to the container's database directory
	// Bind the common bootstrap directory to the container
	// NOTE: I did this using the approach from:
	// https://github.com/fsouza/go-dockerclient/issues/132#issuecomment-50694902
	opts.HostConfig.Binds = append(
		opts.HostConfig.Binds,
		fmt.Sprintf("%s:%s:rw", flowDataDir, DefaultFlowDataDir),
		fmt.Sprintf("%s:%s:ro", nodeBootstrapDir, DefaultBootstrapDir),
	)

	hotstuffStartupTime := time.Now().Add(8 * time.Second).Format(time.RFC3339)

	if !nodeConf.Ghost {
		switch nodeConf.Role {
		case flow.RoleCollection:
			nodeContainer.exposePort(GRPCPort, testingdock.RandomPort(t))
			nodeContainer.AddFlag("ingress-addr", nodeContainer.ContainerAddr(GRPCPort))

			// set a low timeout so that all nodes agree on the current view more quickly
			nodeContainer.AddFlag("hotstuff-min-timeout", time.Second.String())
			nodeContainer.AddFlag("hotstuff-startup-time", hotstuffStartupTime)
			t.Logf("%v hotstuff startup time will be in 8 seconds: %v", time.Now().UTC(), hotstuffStartupTime)

		case flow.RoleExecution:
			nodeContainer.exposePort(GRPCPort, testingdock.RandomPort(t))
			nodeContainer.AddFlag("rpc-addr", nodeContainer.ContainerAddr(GRPCPort))

			nodeContainer.AddFlag("triedir", DefaultExecutionRootDir)
			nodeContainer.AddFlag("execution-data-dir", DefaultExecutionDataServiceDir)
			nodeContainer.AddFlag("chunk-data-pack-dir", DefaultChunkDataPackDir)
			nodeContainer.AddFlag("register-dir", DefaultRegisterDir)

		case flow.RoleAccess:
			nodeContainer.exposePort(GRPCPort, testingdock.RandomPort(t))
			nodeContainer.AddFlag("rpc-addr", nodeContainer.ContainerAddr(GRPCPort))

			nodeContainer.exposePort(GRPCSecurePort, testingdock.RandomPort(t))
			nodeContainer.AddFlag("secure-rpc-addr", nodeContainer.ContainerAddr(GRPCSecurePort))

			nodeContainer.exposePort(GRPCWebPort, testingdock.RandomPort(t))
			nodeContainer.AddFlag("http-addr", nodeContainer.ContainerAddr(GRPCWebPort))

			nodeContainer.exposePort(RESTPort, testingdock.RandomPort(t))
			nodeContainer.AddFlag("rest-addr", nodeContainer.ContainerAddr(RESTPort))

			nodeContainer.exposePort(ExecutionStatePort, testingdock.RandomPort(t))
			nodeContainer.AddFlag("state-stream-addr", nodeContainer.ContainerAddr(ExecutionStatePort))

			// uncomment line below to point the access node exclusively to a single collection node
			// nodeContainer.AddFlag("static-collection-ingress-addr", "collection_1:9000")
			nodeContainer.AddFlag("collection-ingress-port", GRPCPort)

			if nodeContainer.IsFlagSet("supports-observer") {
				nodeContainer.exposePort(PublicNetworkPort, testingdock.RandomPort(t))
				nodeContainer.AddFlag("public-network-address", nodeContainer.ContainerAddr(PublicNetworkPort))
			}

			// execution-sync is enabled by default
			nodeContainer.AddFlag("execution-data-dir", DefaultExecutionDataServiceDir)

		case flow.RoleConsensus:
			if !nodeContainer.IsFlagSet("chunk-alpha") {
				// use 1 here instead of the default 5, because most of the integration
				// tests only start 1 verification node
				nodeContainer.AddFlag("chunk-alpha", "1")
			}
			t.Logf("%v hotstuff startup time will be in 8 seconds: %v", time.Now().UTC(), hotstuffStartupTime)
			nodeContainer.AddFlag("hotstuff-startup-time", hotstuffStartupTime)

		case flow.RoleVerification:
			if !nodeContainer.IsFlagSet("chunk-alpha") {
				// use 1 here instead of the default 5, because most of the integration
				// tests only start 1 verification node
				nodeContainer.AddFlag("chunk-alpha", "1")
			}
		}

		// enable Admin server for all real nodes
		nodeContainer.exposePort(AdminPort, testingdock.RandomPort(t))
		nodeContainer.AddFlag("admin-addr", nodeContainer.ContainerAddr(AdminPort))

		// enable healthchecks for all nodes (via admin server)
		nodeContainer.opts.HealthCheck = testingdock.HealthCheckCustom(nodeContainer.HealthcheckCallback())

		if nodeConf.EnableMetricsServer {
			nodeContainer.exposePort(MetricsPort, testingdock.RandomPort(t))
		}
	} else {
		nodeContainer.exposePort(GRPCPort, testingdock.RandomPort(t))
		nodeContainer.AddFlag("rpc-addr", nodeContainer.ContainerAddr(GRPCPort))

		if nodeContainer.IsFlagSet("supports-observer") {
			// TODO: Currently, it is not possible to create a ghost AN which participates
			// in the public network, because connection gating is enabled by default and
			// therefore the ghost node will deny incoming connections from all consensus
			// followers. A flag for the ghost node will need to be created to enable
			// overriding the default behavior (see: https://github.com/dapperlabs/flow-go/issues/5696).
			return fmt.Errorf("currently ghost node for an access node which supports unstaked node is not implemented")
		}
	}

	if nodeConf.Debug {
		nodeContainer.exposePort(DebuggerPort, DebuggerPort)
	}

	if nodeConf.Corrupted {
		// corrupted nodes are running with a Corrupted Conduit Factory (CCF), hence need to bind their
		// CCF port to local host, so they can be accessible by the orchestrator network.
		hostPort := testingdock.RandomPort(t)
		nodeContainer.exposePort(cmd.CorruptNetworkPort, hostPort)
		net.CorruptedPortMapping[nodeConf.NodeID] = hostPort
	}

	suiteContainer := net.suite.Container(*opts)
	nodeContainer.Container = suiteContainer
	net.Containers[nodeContainer.Name()] = nodeContainer

	// start ghost node right away
	// ghost nodes are to help testing other nodes' logic,
	// in order to reduce tests flakyness, we try to prepare the network, so that the real node
	// can be tested with the network that all ghost nodes are up. Therefore, we start
	// ghost nodes right away
	if nodeConf.Ghost {
		net.network.After(suiteContainer)
		return nil
	}

	// if is real node, since the node config has been sorted, execution should always
	// be created before consensus node. so by the time we are creating access/consensus
	// node, the execution has been created, and we can add the dependency here
	if nodeConf.Role == flow.RoleAccess || nodeConf.Role == flow.RoleConsensus {
		execution1 := net.ContainerByName("execution_1")
		execution1.After(suiteContainer)
	} else {
		net.network.After(suiteContainer)
	}
	return nil
}

func (net *FlowNetwork) WriteRootSnapshot(snapshot *inmem.Snapshot) {
	err := WriteJSON(filepath.Join(net.BootstrapDir, bootstrap.PathRootProtocolStateSnapshot), snapshot.Encodable())
	require.NoError(net.t, err)
}

func followerNodeInfos(confs []ConsensusFollowerConfig) ([]bootstrap.NodeInfo, error) {
	var nodeInfos []bootstrap.NodeInfo

	// TODO: currently just stashing a dummy key as staking key to prevent the nodeinfo.Type() function from
	// returning an error. Eventually, a new key type NodeInfoTypePrivateUnstaked needs to be defined
	// (see issue: https://github.com/onflow/flow-go/issues/1214)
	dummyStakingKey := unittest.StakingPrivKeyFixture()

	for _, conf := range confs {
		info := bootstrap.NewPrivateNodeInfo(
			conf.NodeID,
			flow.RoleAccess, // use Access role
			"",              // no address
			0,               // no weight
			conf.NetworkingPrivKey,
			dummyStakingKey,
		)

		nodeInfos = append(nodeInfos, info)
	}

	return nodeInfos, nil
}

type BootstrapData struct {
	Root              *flow.Block
	Result            *flow.ExecutionResult
	Seal              *flow.Seal
	StakedConfs       []ContainerConfig
	Snapshot          *inmem.Snapshot
	ClusterRootBlocks []*cluster.Block
}

func BootstrapNetwork(networkConf NetworkConfig, bootstrapDir string, chainID flow.ChainID) (*BootstrapData, error) {
	chain := chainID.Chain()

	// number of nodes
	nNodes := len(networkConf.Nodes)
	if nNodes == 0 {
		return nil, fmt.Errorf("must specify at least one node")
	}

	// Sort so that access nodes start up last
	sort.Sort(&networkConf)
	// generate staking and networking keys for each configured node
	stakedConfs, err := setupKeys(networkConf)
	if err != nil {
		return nil, fmt.Errorf("failed to setup keys: %w", err)
	}

	// generate the follower node keys (follower nodes do not run as docker containers)
	followerInfos, err := followerNodeInfos(networkConf.ConsensusFollowers)
	if err != nil {
		return nil, fmt.Errorf("failed to generate node info for consensus followers: %w", err)
	}

	allNodeInfos := append(toNodeInfos(stakedConfs), followerInfos...)

	// IMPORTANT: we must use this ordering when writing the DKG keys as
	//            this ordering defines the DKG participant's indices
	stakedNodeInfos := bootstrap.Sort(toNodeInfos(stakedConfs), order.Canonical[flow.Identity])

	dkg, err := runBeaconKG(stakedConfs)
	if err != nil {
		return nil, fmt.Errorf("failed to run DKG: %w", err)
	}

	// write private key files for each DKG participant
	consensusNodes := bootstrap.FilterByRole(stakedNodeInfos, flow.RoleConsensus)
	for i, sk := range dkg.PrivKeyShares {
		nodeID := consensusNodes[i].NodeID
		sk := encodable.RandomBeaconPrivKey{PrivateKey: sk}
		path := fmt.Sprintf(bootstrap.PathRandomBeaconPriv, nodeID)
		err = WriteJSON(filepath.Join(bootstrapDir, path), sk)
		if err != nil {
			return nil, err
		}
	}

	// write staking and machine account private key files
	writeJSONFile := func(relativePath string, val interface{}) error {
		return WriteJSON(filepath.Join(bootstrapDir, relativePath), val)
	}
	writeFile := func(relativePath string, data []byte) error {
		return WriteFile(filepath.Join(bootstrapDir, relativePath), data)
	}
	err = utils.WriteStakingNetworkingKeyFiles(allNodeInfos, writeJSONFile)
	if err != nil {
		return nil, fmt.Errorf("failed to write private key files: %w", err)
	}

	err = utils.WriteSecretsDBEncryptionKeyFiles(allNodeInfos, writeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to write secrets db key files: %w", err)
	}

	err = utils.WriteMachineAccountFiles(chainID, stakedNodeInfos, writeJSONFile)
	if err != nil {
		return nil, fmt.Errorf("failed to write machine account files: %w", err)
	}

	// define root block parameters
	parentID := flow.ZeroID
	height := uint64(0)
	timestamp := time.Now().UTC()
	epochCounter := uint64(0)
	participants := bootstrap.ToIdentityList(stakedNodeInfos)

	// generate root block
	rootHeader := run.GenerateRootHeader(chainID, parentID, height, timestamp)

	// generate root blocks for each collector cluster
	clusterRootBlocks, clusterAssignments, clusterQCs, err := setupClusterGenesisBlockQCs(networkConf.NClusters, epochCounter, stakedConfs)
	if err != nil {
		return nil, err
	}

	// TODO: extract to func to be reused with `constructRootResultAndSeal`
	qcsWithSignerIDs := make([]*flow.QuorumCertificateWithSignerIDs, 0, len(clusterQCs))
	for i, clusterQC := range clusterQCs {
		members := clusterAssignments[i]
		signerIDs, err := signature.DecodeSignerIndicesToIdentifiers(members, clusterQC.SignerIndices)
		if err != nil {
			return nil, fmt.Errorf("could not decode cluster QC signer indices: %w", err)
		}
		qcsWithSignerIDs = append(qcsWithSignerIDs, &flow.QuorumCertificateWithSignerIDs{
			View:      clusterQC.View,
			BlockID:   clusterQC.BlockID,
			SignerIDs: signerIDs,
			SigData:   clusterQC.SigData,
		})
	}

	randomSource := make([]byte, flow.EpochSetupRandomSourceLength)
	_, err = crand.Read(randomSource)
	if err != nil {
		return nil, err
	}

	dkgOffsetView := rootHeader.View + networkConf.ViewsInStakingAuction - 1

	// generate epoch service events
	epochSetup := &flow.EpochSetup{
		Counter:            epochCounter,
		FirstView:          rootHeader.View,
		DKGPhase1FinalView: dkgOffsetView + networkConf.ViewsInDKGPhase,
		DKGPhase2FinalView: dkgOffsetView + networkConf.ViewsInDKGPhase*2,
		DKGPhase3FinalView: dkgOffsetView + networkConf.ViewsInDKGPhase*3,
		FinalView:          rootHeader.View + networkConf.ViewsInEpoch - 1,
		Participants:       participants.ToSkeleton(),
		Assignments:        clusterAssignments,
		RandomSource:       randomSource,
	}

	epochCommit := &flow.EpochCommit{
		Counter:            epochCounter,
		ClusterQCs:         flow.ClusterQCVoteDatasFromQCs(qcsWithSignerIDs),
		DKGGroupKey:        dkg.PubGroupKey,
		DKGParticipantKeys: dkg.PubKeyShares,
	}
	root := &flow.Block{
		Header: rootHeader,
	}
	root.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(
		inmem.ProtocolStateFromEpochServiceEvents(epochSetup, epochCommit).ID())))

	cdcRandomSource, err := cadence.NewString(hex.EncodeToString(randomSource))
	if err != nil {
		return nil, fmt.Errorf("could not convert random source: %w", err)
	}
	epochConfig := epochs.EpochConfig{
		RewardCut:                    cadence.UFix64(0),
		CurrentEpochCounter:          cadence.UInt64(epochCounter),
		NumViewsInEpoch:              cadence.UInt64(networkConf.ViewsInEpoch),
		NumViewsInStakingAuction:     cadence.UInt64(networkConf.ViewsInStakingAuction),
		NumViewsInDKGPhase:           cadence.UInt64(networkConf.ViewsInDKGPhase),
		NumCollectorClusters:         cadence.UInt16(len(clusterQCs)),
		FLOWsupplyIncreasePercentage: cadence.UFix64(0),
		RandomSource:                 cdcRandomSource,
		CollectorClusters:            clusterAssignments,
		ClusterQCs:                   clusterQCs,
		DKGPubKeys:                   encodable.WrapRandomBeaconPubKeys(dkg.PubKeyShares),
	}

	// generate the initial execution state
	trieDir := filepath.Join(bootstrapDir, bootstrap.DirnameExecutionState)
	commit, err := run.GenerateExecutionState(
		trieDir,
		unittest.ServiceAccountPublicKey,
		chain,
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithRootBlock(root.Header),
		fvm.WithEpochConfig(epochConfig),
		fvm.WithIdentities(participants),
	)
	if err != nil {
		return nil, err
	}

	// generate execution result and block seal
	result := run.GenerateRootResult(root, commit, epochSetup, epochCommit)
	seal, err := run.GenerateRootSeal(result)
	if err != nil {
		return nil, fmt.Errorf("generating root seal failed: %w", err)
	}

	// generate QC
	signerData, err := run.GenerateQCParticipantData(consensusNodes, consensusNodes, dkg)
	if err != nil {
		return nil, err
	}
	votes, err := run.GenerateRootBlockVotes(root, signerData)
	if err != nil {
		return nil, err
	}
	qc, invalidVotesErr, err := run.GenerateRootQC(root, votes, signerData, signerData.Identities())
	if err != nil {
		return nil, err
	}

	if len(invalidVotesErr) > 0 {
		return nil, fmt.Errorf("has invalid votes: %v", invalidVotesErr)
	}

	snapshot, err := inmem.SnapshotFromBootstrapStateWithParams(root, result, seal, qc, flow.DefaultProtocolVersion, networkConf.EpochCommitSafetyThreshold)
	if err != nil {
		return nil, fmt.Errorf("could not create bootstrap state snapshot: %w", err)
	}

	err = badger.IsValidRootSnapshotQCs(snapshot)
	if err != nil {
		return nil, fmt.Errorf("invalid root snapshot qcs: %w", err)
	}

	err = WriteJSON(filepath.Join(bootstrapDir, bootstrap.PathRootProtocolStateSnapshot), snapshot.Encodable())
	if err != nil {
		return nil, err
	}

	return &BootstrapData{
		Root:              root,
		Result:            result,
		Seal:              seal,
		StakedConfs:       stakedConfs,
		Snapshot:          snapshot,
		ClusterRootBlocks: clusterRootBlocks,
	}, nil
}

// setupKeys generates private staking and networking keys for each configured
// node. It also assigns each node a unique container name and network address.
func setupKeys(networkConf NetworkConfig) ([]ContainerConfig, error) {

	nNodes := len(networkConf.Nodes)

	// keep track of how many roles we have assigned so we can number containers
	// correctly (consensus_1, consensus_2, etc.)
	roleCounter := make(map[flow.Role]int)

	// get networking keys for all nodes
	networkKeys := unittest.NetworkingKeys(nNodes)

	// get staking keys for all nodes
	stakingKeys := unittest.StakingKeys(nNodes)

	// create node container configs and corresponding public identities
	confs := make([]ContainerConfig, 0, nNodes)
	for i, conf := range networkConf.Nodes {
		// define the node's name <role>_<n> and address <name>:<port>
		name := fmt.Sprintf("%s_%d", conf.Role.String(), roleCounter[conf.Role]+1)

		addr := fmt.Sprintf("%s:%d", name, DefaultFlowPort)
		roleCounter[conf.Role]++

		info := bootstrap.NewPrivateNodeInfo(
			conf.Identifier,
			conf.Role,
			addr,
			conf.Weight,
			networkKeys[i],
			stakingKeys[i],
		)

		containerConf := ContainerConfig{
			NodeInfo:            info,
			ContainerName:       name,
			LogLevel:            conf.LogLevel,
			Ghost:               conf.Ghost,
			AdditionalFlags:     conf.AdditionalFlags,
			Debug:               conf.Debug,
			Corrupted:           conf.Corrupted,
			EnableMetricsServer: conf.EnableMetricsServer,
		}

		confs = append(confs, containerConf)
	}

	return confs, nil
}

// runBeaconKG simulates the distributed key generation process for all consensus nodes
// and returns all DKG data. This includes the group private key, node indices,
// and per-node public and private key-shares.
// Only consensus nodes participate in the DKG.
func runBeaconKG(confs []ContainerConfig) (dkgmod.DKGData, error) {

	// filter by consensus nodes
	consensusNodes := bootstrap.FilterByRole(toNodeInfos(confs), flow.RoleConsensus)
	nConsensusNodes := len(consensusNodes)

	dkgSeed, err := getSeed()
	if err != nil {
		return dkgmod.DKGData{}, err
	}

	dkg, err := dkg.RandomBeaconKG(nConsensusNodes, dkgSeed)
	if err != nil {
		return dkgmod.DKGData{}, err
	}

	return dkg, nil
}

// setupClusterGenesisBlockQCs generates bootstrapping resources necessary for each collector cluster:
//   - a cluster-specific root block
//   - a cluster-specific root QC
func setupClusterGenesisBlockQCs(nClusters uint, epochCounter uint64, confs []ContainerConfig) ([]*cluster.Block, flow.AssignmentList, []*flow.QuorumCertificate, error) {

	participantsUnsorted := toParticipants(confs)
	participants := participantsUnsorted.Sort(order.Canonical[flow.Identity])
	collectors := participants.Filter(filter.HasRole[flow.Identity](flow.RoleCollection)).ToSkeleton()
	assignments := unittest.ClusterAssignment(nClusters, collectors)
	clusters, err := factory.NewClusterList(assignments, collectors)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not create cluster list: %w", err)
	}

	rootBlocks := make([]*cluster.Block, 0, nClusters)
	qcs := make([]*flow.QuorumCertificate, 0, nClusters)

	for _, cluster := range clusters {
		// generate root cluster block
		block := clusterstate.CanonicalRootBlock(epochCounter, cluster)

		lookup := make(map[flow.Identifier]struct{})
		for _, node := range cluster {
			lookup[node.NodeID] = struct{}{}
		}

		// gather cluster participants
		clusterNodeInfos := make([]bootstrap.NodeInfo, 0, len(cluster))
		for _, conf := range confs {
			_, exists := lookup[conf.NodeID]
			if exists {
				clusterNodeInfos = append(clusterNodeInfos, conf.NodeInfo)
			}
		}
		if len(cluster) != len(clusterNodeInfos) { // sanity check
			return nil, nil, nil, fmt.Errorf("requiring a node info for each cluster participant")
		}

		// must order in canonical ordering otherwise decoding signer indices from cluster QC would fail
		clusterCommittee := bootstrap.ToIdentityList(clusterNodeInfos).Sort(order.Canonical[flow.Identity]).ToSkeleton()
		qc, err := run.GenerateClusterRootQC(clusterNodeInfos, clusterCommittee, block)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("fail to generate cluster root QC with clusterNodeInfos %v, %w",
				clusterNodeInfos, err)
		}

		// add block and qc to list
		qcs = append(qcs, qc)
		rootBlocks = append(rootBlocks, block)
	}

	return rootBlocks, assignments, qcs, nil
}
