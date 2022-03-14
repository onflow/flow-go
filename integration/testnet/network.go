package testnet

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/onflow/flow-go/cmd/bootstrap/dkg"

	"github.com/dapperlabs/testingdock"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/onflow/cadence"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	consensus_follower "github.com/onflow/flow-go/follower"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/bootstrap"
	dkgmod "github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	clusterstate "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// TmpRoot is the default root directory to create temporary data
	// directories for containers. We use /tmp because $TMPDIR is not exposed
	// to docker by default on macOS
	TmpRoot = "/tmp"

	// DefaultBootstrapDir is the default directory for bootstrap files
	DefaultBootstrapDir = "/bootstrap"

	// DefaultFlowDataDir is the root of all database storage
	DefaultFlowDataDir = "/data"
	// DefaultFlowDBDir is the default directory for the node database.
	DefaultFlowDBDir = "/data/protocol"
	// DefaultFlowSecretsDBDir is the default directory for secrets database.
	DefaultFlowSecretsDBDir = "/data/secrets"
	// DefaultExecutionRootDir is the default directory for the execution node
	// state database.
	DefaultExecutionRootDir = "/exedb"
	// DefaultExecutionDataServiceDir for the execution data service blobstore.
	DefaultExecutionDataServiceDir = "/data/execution_data"

	// ColNodeAPIPort is the name used for the collection node API port.
	ColNodeAPIPort = "col-ingress-port"
	// ExeNodeAPIPort is the name used for the execution node API port.
	ExeNodeAPIPort = "exe-api-port"
	// AccessNodeAPIPort is the name used for the access node API port.
	AccessNodeAPIPort = "access-api-port"
	// AccessNodeAPISecurePort is the name used for the secure access API port.
	AccessNodeAPISecurePort = "access-api-secure-port"
	// AccessNodeAPIProxyPort is the name used for the access node API HTTP proxy port.
	AccessNodeAPIProxyPort = "access-api-http-proxy-port"
	// AccessNodeExternalNetworkPort is the name used for the access node network port accessible from outside any docker container
	AccessNodeExternalNetworkPort = "access-external-network-port"
	// GhostNodeAPIPort is the name used for the access node API port.
	GhostNodeAPIPort = "ghost-api-port"

	// ExeNodeMetricsPort is the name used for the execution node metrics server port
	ExeNodeMetricsPort = "exe-metrics-port"

	// ColNodeMetricsPort is the name used for the collection node metrics server port
	ColNodeMetricsPort = "col-metrics-port"

	// AccessNodeMetricsPort is the name used for the access node metrics server port
	AccessNodeMetricsPort = "access-metrics-port"

	// VerNodeMetricsPort is the name used for the verification node metrics server port
	VerNodeMetricsPort = "verification-metrics-port"

	// ConNodeMetricsPort is the name used for the consensus node metrics server port
	ConNodeMetricsPort = "con-metrics-port"

	// DefaultFlowPort default gossip network port
	DefaultFlowPort = 2137
	// DefaultSecureGRPCPort is the port used to access secure GRPC server running on ANs
	DefaultSecureGRPCPort = 9001
	// AccessNodePublicNetworkPort is the port used by access nodes for the public libp2p network
	AccessNodePublicNetworkPort = 9876

	DefaultViewsInStakingAuction uint64 = 5
	DefaultViewsInDKGPhase       uint64 = 50
	DefaultViewsInEpoch          uint64 = 180

	integrationBootstrap = "flow-integration-bootstrap"

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
	t                           *testing.T
	log                         zerolog.Logger
	suite                       *testingdock.Suite
	config                      NetworkConfig
	cli                         *dockerclient.Client
	network                     *testingdock.Network
	Containers                  map[string]*Container
	ConsensusFollowers          map[flow.Identifier]consensus_follower.ConsensusFollower
	AccessPorts                 map[string]string
	AccessPortsByContainerName  map[string]string
	MetricsPortsByContainerName map[string]string
	root                        *flow.Block
	result                      *flow.ExecutionResult
	seal                        *flow.Seal
	BootstrapDir                string
	BootstrapSnapshot           *inmem.Snapshot
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
func (net *FlowNetwork) DropDBs(filter flow.IdentityFilter) {
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

func (net *FlowNetwork) PrintMetricsPorts() {
	var builder strings.Builder
	builder.WriteString("metrics endpoints by container name:\n")
	for containerName, metricsPort := range net.MetricsPortsByContainerName {
		builder.WriteString(fmt.Sprintf("\t%s: 0.0.0.0:%s/metrics\n", containerName, metricsPort))
	}
	fmt.Print(builder.String())
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
	nodeID, err := p2p.NewUnstakedNetworkIDTranslator().GetFlowID(pid)
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
	Nodes                 []NodeConfig
	ConsensusFollowers    []ConsensusFollowerConfig
	Name                  string
	NClusters             uint
	ViewsInDKGPhase       uint64
	ViewsInStakingAuction uint64
	ViewsInEpoch          uint64
}

type NetworkConfigOpt func(*NetworkConfig)

func NewNetworkConfig(name string, nodes []NodeConfig, opts ...NetworkConfigOpt) NetworkConfig {
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

func NewNetworkConfigWithEpochConfig(name string, nodes []NodeConfig, viewsInStakingAuction, viewsInDKGPhase, viewsInEpoch uint64, opts ...NetworkConfigOpt) NetworkConfig {
	c := NetworkConfig{
		Nodes:                 nodes,
		Name:                  name,
		NClusters:             1, // default to 1 cluster
		ViewsInStakingAuction: viewsInStakingAuction,
		ViewsInDKGPhase:       viewsInDKGPhase,
		ViewsInEpoch:          viewsInEpoch,
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

func WithClusters(n uint) func(*NetworkConfig) {
	return func(conf *NetworkConfig) {
		conf.NClusters = n
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

// NodeConfig defines the input config for a particular node, specified prior
// to network creation.
type NodeConfig struct {
	Role                  flow.Role
	Weight                uint64
	Identifier            flow.Identifier
	LogLevel              zerolog.Level
	Ghost                 bool
	AdditionalFlags       []string
	Debug                 bool
	SupportsUnstakedNodes bool // only applicable to Access node
}

func NewNodeConfig(role flow.Role, opts ...func(*NodeConfig)) NodeConfig {
	c := NodeConfig{
		Role:       role,
		Weight:     flow.DefaultInitialWeight,
		Identifier: unittest.IdentifierFixture(), // default random ID
		LogLevel:   zerolog.DebugLevel,           // log at debug by default
	}

	for _, apply := range opts {
		apply(&c)
	}

	return c
}

// NewNodeConfigSet creates a set of node configs with the given role. The nodes
// are given sequential IDs with a common prefix to make reading logs easier.
func NewNodeConfigSet(n uint, role flow.Role, opts ...func(*NodeConfig)) []NodeConfig {

	// each node in the set has a common 4-digit prefix, separated from their
	// index with a `0` character
	idPrefix := uint(rand.Intn(10000) * 100)

	confs := make([]NodeConfig, n)
	for i := uint(0); i < n; i++ {
		confs[i] = NewNodeConfig(role, append(opts, WithIDInt(idPrefix+i+1))...)
	}

	return confs
}

func WithID(id flow.Identifier) func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.Identifier = id
	}
}

// WithIDInt sets the node ID so the hex representation matches the input.
// Useful for having consistent and easily readable IDs in test logs.
func WithIDInt(id uint) func(config *NodeConfig) {

	idStr := strconv.Itoa(int(id))
	// left pad ID with zeros
	pad := strings.Repeat("0", 64-len(idStr))
	hex := pad + idStr

	// convert hex to ID
	flowID, err := flow.HexStringToIdentifier(hex)
	if err != nil {
		panic(err)
	}

	return WithID(flowID)
}

func WithLogLevel(level zerolog.Level) func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.LogLevel = level
	}
}

func WithDebugImage(debug bool) func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.Debug = debug
	}
}

func AsGhost() func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.Ghost = true
		// using the fully-connectred topology to ensure a ghost node is always connected to all other nodes in the network,
		config.AdditionalFlags = append(config.AdditionalFlags, "--topology=fully-connected")
	}
}

func SupportsUnstakedNodes() func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.SupportsUnstakedNodes = true
	}
}

// WithAdditionalFlag adds additional flags to the command
func WithAdditionalFlag(flag string) func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.AdditionalFlags = append(config.AdditionalFlags, flag)
	}
}

// integrationBootstrapDir creates a temporary directory at /tmp/flow-integration-bootstrap
func integrationBootstrapDir() (string, error) {
	return ioutil.TempDir(TmpRoot, integrationBootstrap)
}

func PrepareFlowNetwork(t *testing.T, networkConf NetworkConfig) *FlowNetwork {
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
	bootstrapDir, err := integrationBootstrapDir()
	require.Nil(t, err)

	t.Logf("BootstrapDir: %s \n", bootstrapDir)

	root, result, seal, confs, bootstrapSnapshot, err := BootstrapNetwork(networkConf, bootstrapDir)
	require.Nil(t, err)

	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("module", "flownetwork").
		Str("testcase", t.Name()).
		Logger()

	flowNetwork := &FlowNetwork{
		t:                           t,
		cli:                         dockerClient,
		config:                      networkConf,
		suite:                       suite,
		network:                     network,
		log:                         logger,
		Containers:                  make(map[string]*Container, nNodes),
		ConsensusFollowers:          make(map[flow.Identifier]consensus_follower.ConsensusFollower, len(networkConf.ConsensusFollowers)),
		AccessPorts:                 make(map[string]string),
		AccessPortsByContainerName:  make(map[string]string),
		MetricsPortsByContainerName: make(map[string]string),
		root:                        root,
		seal:                        seal,
		result:                      result,
		BootstrapDir:                bootstrapDir,
		BootstrapSnapshot:           bootstrapSnapshot,
	}

	// check that at-least 2 full access nodes must be configure in your test suite
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

	rootProtocolSnapshotPath := filepath.Join(bootstrapDir, bootstrap.PathRootProtocolStateSnapshot)

	// add each follower to the network
	for _, followerConf := range networkConf.ConsensusFollowers {
		t.Logf("add consensus follower %v", followerConf.NodeID)
		flowNetwork.addConsensusFollower(t, rootProtocolSnapshotPath, followerConf, confs)
	}

	// flowNetwork.PrintMetricsPorts()

	t.Logf("%v finish preparing flow network for %v", time.Now().UTC(), t.Name())

	return flowNetwork
}

func (net *FlowNetwork) addConsensusFollower(t *testing.T, rootProtocolSnapshotPath string, followerConf ConsensusFollowerConfig, containers []ContainerConfig) {
	tmpdir, err := ioutil.TempDir(TmpRoot, "flow-consensus-follower")
	require.NoError(t, err)

	// create a directory for the follower database
	dataDir := filepath.Join(tmpdir, DefaultFlowDBDir)
	err = os.MkdirAll(dataDir, 0700)
	require.NoError(t, err)

	// create a follower-specific directory for the bootstrap files
	followerBootstrapDir := filepath.Join(tmpdir, DefaultBootstrapDir)
	err = os.Mkdir(followerBootstrapDir, 0700)
	require.NoError(t, err)

	publicRootInformationDir := filepath.Join(followerBootstrapDir, bootstrap.DirnamePublicBootstrap)
	err = os.Mkdir(publicRootInformationDir, 0700)
	require.NoError(t, err)

	// strip out the node addresses from root-protocol-state-snapshot.json and copy it to the follower-specific
	// bootstrap/public-root-information directory
	err = rootProtocolJsonWithoutAddresses(rootProtocolSnapshotPath, filepath.Join(followerBootstrapDir, bootstrap.PathRootProtocolStateSnapshot))
	require.NoError(t, err)

	// consensus follower
	bindPort := testingdock.RandomPort(t)
	bindAddr := fmt.Sprintf("0.0.0.0:%s", bindPort)
	opts := append(
		followerConf.Opts,
		consensus_follower.WithDataDir(dataDir),
		consensus_follower.WithBootstrapDir(followerBootstrapDir),
	)

	var stakedANContainer *ContainerConfig
	// find the upstream Access node container for this follower engine
	for _, cont := range containers {
		if cont.NodeID == followerConf.StakedNodeID {
			stakedANContainer = &cont
			break
		}
	}
	require.NotNil(t, stakedANContainer, "unable to find staked AN for the follower engine %s", followerConf.NodeID.String())

	portStr := net.AccessPorts[AccessNodeExternalNetworkPort]
	portU64, err := strconv.ParseUint(portStr, 10, 32)
	require.NoError(t, err)
	port := uint(portU64)

	bootstrapNodeInfo := consensus_follower.BootstrapNodeInfo{
		Host:             "localhost",
		Port:             port,
		NetworkPublicKey: stakedANContainer.NetworkPubKey(),
	}

	// it should be able to figure out the rest on its own.
	follower, err := consensus_follower.NewConsensusFollower(followerConf.NetworkingPrivKey, bindAddr,
		[]consensus_follower.BootstrapNodeInfo{bootstrapNodeInfo}, opts...)

	net.ConsensusFollowers[followerConf.NodeID] = follower
}

// AddNode creates a node container with the given config and adds it to the
// network.
func (net *FlowNetwork) AddNode(t *testing.T, bootstrapDir string, nodeConf ContainerConfig) error {
	profilerDir := "/profiler"
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
				fmt.Sprintf("--profiler-dir=%s", profilerDir),
				fmt.Sprintf("--secretsdir=%s", DefaultFlowSecretsDBDir),
				fmt.Sprintf("--loglevel=%s", nodeConf.LogLevel.String()),
			}, nodeConf.AdditionalFlags...),
		},
		HostConfig: &container.HostConfig{},
	}

	// get a temporary directory in the host. On macOS the default tmp
	// directory is NOT accessible to Docker by default, so we use /tmp
	// instead.
	tmpdir, err := ioutil.TempDir(TmpRoot, "flow-integration-node")
	if err != nil {
		return fmt.Errorf("could not get tmp dir: %w", err)
	}

	t.Logf("%v adding container %v for %v node", time.Now().UTC(), nodeConf.ContainerName, nodeConf.Role)

	nodeContainer := &Container{
		Config:  nodeConf,
		Ports:   make(map[string]string),
		datadir: tmpdir,
		net:     net,
		opts:    opts,
	}

	// create a directory for the node database
	flowDataDir := filepath.Join(tmpdir, DefaultFlowDataDir)
	err = os.Mkdir(flowDataDir, 0700)
	require.NoError(t, err)

	flowProfilerDir := filepath.Join(tmpdir, profilerDir)
	err = os.Mkdir(flowProfilerDir, 0755)
	require.NoError(t, err)

	// create a directory for the bootstrap files
	// we create a node-specific bootstrap directory to enable testing nodes
	// bootstrapping from different root state snapshots and epochs
	nodeBootstrapDir := filepath.Join(tmpdir, DefaultBootstrapDir)
	err = os.Mkdir(nodeBootstrapDir, 0700)
	require.NoError(t, err)

	// copy bootstrap files to node-specific bootstrap directory
	err = io.CopyDirectory(bootstrapDir, nodeBootstrapDir)
	require.NoError(t, err)

	// Bind the host directory to the container's database directory
	// Bind the common bootstrap directory to the container
	// NOTE: I did this using the approach from:
	// https://github.com/fsouza/go-dockerclient/issues/132#issuecomment-50694902
	opts.HostConfig.Binds = append(
		opts.HostConfig.Binds,
		fmt.Sprintf("%s:%s:rw", flowDataDir, DefaultFlowDataDir),
		fmt.Sprintf("%s:%s:rw", flowProfilerDir, profilerDir),
		fmt.Sprintf("%s:%s:ro", nodeBootstrapDir, DefaultBootstrapDir),
	)

	hotstuffStartupTime := time.Now().Add(8 * time.Second).Format(time.RFC3339)

	if !nodeConf.Ghost {
		switch nodeConf.Role {
		case flow.RoleCollection:

			hostPort := testingdock.RandomPort(t)
			containerPort := "9000/tcp"

			nodeContainer.bindPort(hostPort, containerPort)

			// uncomment this code to expose the metrics server for each node
			// hostMetricsPort := testingdock.RandomPort(t)
			// containerMetricsPort := "8080/tcp"

			// nodeContainer.bindPort(hostMetricsPort, containerMetricsPort)
			// nodeContainer.Ports[ColNodeMetricsPort] = hostMetricsPort
			// net.AccessPorts[ColNodeMetricsPort] = hostMetricsPort
			// net.MetricsPortsByContainerName[nodeContainer.Name()] = hostMetricsPort
			// set a low timeout so that all nodes agree on the current view more quickly
			nodeContainer.AddFlag("hotstuff-timeout", time.Second.String())
			nodeContainer.AddFlag("hotstuff-min-timeout", time.Second.String())
			t.Logf("%v hotstuff startup time will be in 8 seconds: %v", time.Now().UTC(), hotstuffStartupTime)
			nodeContainer.AddFlag("hotstuff-startup-time", hotstuffStartupTime)

			nodeContainer.AddFlag("ingress-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))
			nodeContainer.Ports[ColNodeAPIPort] = hostPort
			nodeContainer.opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckAccessGRPC(hostPort))
			net.AccessPorts[ColNodeAPIPort] = hostPort

		case flow.RoleExecution:

			hostPort := testingdock.RandomPort(t)
			containerPort := "9000/tcp"

			nodeContainer.bindPort(hostPort, containerPort)

			// hostMetricsPort := testingdock.RandomPort(t)
			// containerMetricsPort := "8080/tcp"

			// nodeContainer.bindPort(hostMetricsPort, containerMetricsPort)
			// net.MetricsPortsByContainerName[nodeContainer.Name()] = hostMetricsPort

			nodeContainer.AddFlag("rpc-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))

			nodeContainer.Ports[ExeNodeAPIPort] = hostPort
			nodeContainer.opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckExecutionGRPC(hostPort))
			net.AccessPorts[ExeNodeAPIPort] = hostPort

			// nodeContainer.Ports[ExeNodeMetricsPort] = hostMetricsPort
			// net.AccessPorts[ExeNodeMetricsPort] = hostMetricsPort

			// create directories for execution state trie and values in the tmp
			// host directory.
			tmpLedgerDir, err := ioutil.TempDir(tmpdir, "flow-integration-trie")
			require.NoError(t, err)

			opts.HostConfig.Binds = append(
				opts.HostConfig.Binds,
				fmt.Sprintf("%s:%s:rw", tmpLedgerDir, DefaultExecutionRootDir),
			)

			nodeContainer.AddFlag("triedir", DefaultExecutionRootDir)

			tmpExeDataDir, err := ioutil.TempDir(tmpdir, "execution-data")
			require.NoError(t, err)

			opts.HostConfig.Binds = append(
				opts.HostConfig.Binds,
				fmt.Sprintf("%s:%s:rw", tmpExeDataDir, DefaultExecutionDataServiceDir),
			)

			nodeContainer.AddFlag("execution-data-dir", DefaultExecutionDataServiceDir)

		case flow.RoleAccess:
			hostGRPCPort := testingdock.RandomPort(t)
			hostHTTPProxyPort := testingdock.RandomPort(t)
			hostSecureGRPCPort := testingdock.RandomPort(t)
			containerGRPCPort := "9000/tcp"
			containerSecureGRPCPort := "9001/tcp"
			containerHTTPProxyPort := "8000/tcp"
			nodeContainer.bindPort(hostGRPCPort, containerGRPCPort)
			nodeContainer.bindPort(hostHTTPProxyPort, containerHTTPProxyPort)
			nodeContainer.bindPort(hostSecureGRPCPort, containerSecureGRPCPort)
			nodeContainer.AddFlag("rpc-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))
			nodeContainer.AddFlag("http-addr", fmt.Sprintf("%s:8000", nodeContainer.Name()))
			// uncomment line below to point the access node exclusively to a single collection node
			// nodeContainer.AddFlag("static-collection-ingress-addr", "collection_1:9000")
			nodeContainer.AddFlag("collection-ingress-port", "9000")
			net.AccessPorts[AccessNodeAPISecurePort] = hostSecureGRPCPort
			nodeContainer.opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckAccessGRPC(hostGRPCPort))
			nodeContainer.Ports[AccessNodeAPIPort] = hostGRPCPort
			nodeContainer.Ports[AccessNodeAPIProxyPort] = hostHTTPProxyPort
			net.AccessPorts[AccessNodeAPIPort] = hostGRPCPort
			net.AccessPortsByContainerName[nodeContainer.Name()] = hostGRPCPort
			net.AccessPorts[AccessNodeAPIProxyPort] = hostHTTPProxyPort

			if nodeConf.SupportsUnstakedNodes {
				hostExternalNetworkPort := testingdock.RandomPort(t)
				containerExternalNetworkPort := fmt.Sprintf("%d/tcp", AccessNodePublicNetworkPort)
				nodeContainer.bindPort(hostExternalNetworkPort, containerExternalNetworkPort)
				net.AccessPorts[AccessNodeExternalNetworkPort] = hostExternalNetworkPort
				nodeContainer.AddFlag("supports-unstaked-node", "true")
				nodeContainer.AddFlag("public-network-address", fmt.Sprintf("%s:%d", nodeContainer.Name(), AccessNodePublicNetworkPort))
			}

			// nodeContainer.bindPort(hostMetricsPort, containerMetricsPort)
			// nodeContainer.Ports[AccessNodeMetricsPort] = hostMetricsPort
			// net.AccessPorts[AccessNodeMetricsPort] = hostMetricsPort
			// net.MetricsPortsByContainerName[nodeContainer.Name()] = hostMetricsPort

		case flow.RoleConsensus:
			// use 1 here instead of the default 5, because the integration
			// tests only start 1 verification node
			nodeContainer.AddFlag("chunk-alpha", "1")
			t.Logf("%v hotstuff startup time will be in 8 seconds: %v", time.Now().UTC(), hotstuffStartupTime)
			nodeContainer.AddFlag("hotstuff-startup-time", hotstuffStartupTime)

			// nodeContainer.bindPort(hostMetricsPort, containerMetricsPort)
			// nodeContainer.Ports[ConNodeMetricsPort] = hostMetricsPort
			// net.AccessPorts[ConNodeMetricsPort] = hostMetricsPort
			// net.MetricsPortsByContainerName[nodeContainer.Name()] = hostMetricsPort
		case flow.RoleVerification:
			// use 1 here instead of the default 5, because the integration
			// tests only start 1 verification node
			nodeContainer.AddFlag("chunk-alpha", "1")

			// nodeContainer.bindPort(hostMetricsPort, containerMetricsPort)
			// nodeContainer.Ports[VerNodeMetricsPort] = hostMetricsPort
			// net.AccessPorts[VerNodeMetricsPort] = hostMetricsPort
			// net.MetricsPortsByContainerName[nodeContainer.Name()] = hostMetricsPort
		}
	} else {
		hostPort := testingdock.RandomPort(t)
		containerPort := "9000/tcp"

		nodeContainer.AddFlag("rpc-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))
		nodeContainer.bindPort(hostPort, containerPort)
		nodeContainer.Ports[GhostNodeAPIPort] = hostPort

		if nodeConf.SupportsUnstakedNodes {
			// TODO: Currently, it is not possible to create a ghost AN which participates
			// in the public network, because connection gating is enabled by default and
			// therefore the ghost node will deny incoming connections from all consensus
			// followers. A flag for the ghost node will need to be created to enable
			// overriding the default behavior (see: https://github.com/dapperlabs/flow-go/issues/5696).
			return fmt.Errorf("currently ghost node for an access node which supports unstaked node is not implemented")
		}
	}

	if nodeConf.Debug {
		hostPort := "2345"
		containerPort := "2345/tcp"
		nodeContainer.bindPort(hostPort, containerPort)
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

func BootstrapNetwork(networkConf NetworkConfig, bootstrapDir string) (*flow.Block, *flow.ExecutionResult, *flow.Seal, []ContainerConfig, *inmem.Snapshot, error) {
	chainID := flow.Localnet
	chain := chainID.Chain()

	// number of nodes
	nNodes := len(networkConf.Nodes)
	if nNodes == 0 {
		return nil, nil, nil, nil, nil, fmt.Errorf("must specify at least one node")
	}

	// Sort so that access nodes start up last
	sort.Sort(&networkConf)

	// generate staking and networking keys for each configured node
	stakedConfs, err := setupKeys(networkConf)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to setup keys: %w", err)
	}

	// generate the follower node keys (follower nodes do not run as docker containers)
	followerInfos, err := followerNodeInfos(networkConf.ConsensusFollowers)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to generate node info for consensus followers: %w", err)
	}

	allNodeInfos := append(toNodeInfos(stakedConfs), followerInfos...)

	// IMPORTANT: we must use this ordering when writing the DKG keys as
	//            this ordering defines the DKG participant's indices
	stakedNodeInfos := bootstrap.Sort(toNodeInfos(stakedConfs), order.Canonical)

	dkg, err := runDKG(stakedConfs)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to run DKG: %w", err)
	}

	// write private key files for each DKG participant
	consensusNodes := bootstrap.FilterByRole(stakedNodeInfos, flow.RoleConsensus)
	for i, sk := range dkg.PrivKeyShares {
		nodeID := consensusNodes[i].NodeID
		sk := encodable.RandomBeaconPrivKey{PrivateKey: sk}
		path := fmt.Sprintf(bootstrap.PathRandomBeaconPriv, nodeID)
		err = WriteJSON(filepath.Join(bootstrapDir, path), sk)
		if err != nil {
			return nil, nil, nil, nil, nil, err
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
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to write private key files: %w", err)
	}

	err = utils.WriteSecretsDBEncryptionKeyFiles(allNodeInfos, writeFile)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to write secrets db key files: %w", err)
	}

	err = utils.WriteMachineAccountFiles(chainID, stakedNodeInfos, writeJSONFile)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to write machine account files: %w", err)
	}

	// define root block parameters
	parentID := flow.ZeroID
	height := uint64(0)
	timestamp := time.Now().UTC()
	epochCounter := uint64(0)
	participants := bootstrap.ToIdentityList(stakedNodeInfos)

	// generate root block
	root := run.GenerateRootBlock(chainID, parentID, height, timestamp)

	// generate QC
	signerData, err := run.GenerateQCParticipantData(consensusNodes, consensusNodes, dkg)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	votes, err := run.GenerateRootBlockVotes(root, signerData)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	qc, err := run.GenerateRootQC(root, votes, signerData, signerData.Identities())
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	// generate root blocks for each collector cluster
	clusterAssignments, clusterQCs, err := setupClusterGenesisBlockQCs(networkConf.NClusters, epochCounter, stakedConfs)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	randomSource := make([]byte, flow.EpochSetupRandomSourceLength)
	_, err = rand.Read(randomSource)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	dkgOffsetView := root.Header.View + networkConf.ViewsInStakingAuction - 1

	// generate epoch service events
	epochSetup := &flow.EpochSetup{
		Counter:            epochCounter,
		FirstView:          root.Header.View,
		DKGPhase1FinalView: dkgOffsetView + networkConf.ViewsInDKGPhase,
		DKGPhase2FinalView: dkgOffsetView + networkConf.ViewsInDKGPhase*2,
		DKGPhase3FinalView: dkgOffsetView + networkConf.ViewsInDKGPhase*3,
		FinalView:          root.Header.View + networkConf.ViewsInEpoch - 1,
		Participants:       participants,
		Assignments:        clusterAssignments,
		RandomSource:       randomSource,
	}

	epochCommit := &flow.EpochCommit{
		Counter:            epochCounter,
		ClusterQCs:         flow.ClusterQCVoteDatasFromQCs(clusterQCs),
		DKGGroupKey:        dkg.PubGroupKey,
		DKGParticipantKeys: dkg.PubKeyShares,
	}

	cdcRandomSource, err := cadence.NewString(hex.EncodeToString(randomSource))
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("could not convert random source: %w", err)
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
		DKGPubKeys:                   dkg.PubKeyShares,
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
		return nil, nil, nil, nil, nil, err
	}

	// generate execution result and block seal
	result := run.GenerateRootResult(root, commit, epochSetup, epochCommit)
	seal, err := run.GenerateRootSeal(result)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("generating root seal failed: %w", err)
	}

	snapshot, err := inmem.SnapshotFromBootstrapState(root, result, seal, qc)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("could not create bootstrap state snapshot: %w", err)
	}

	err = WriteJSON(filepath.Join(bootstrapDir, bootstrap.PathRootProtocolStateSnapshot), snapshot.Encodable())
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return root, result, seal, stakedConfs, snapshot, nil
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
			NodeInfo:              info,
			ContainerName:         name,
			LogLevel:              conf.LogLevel,
			Ghost:                 conf.Ghost,
			AdditionalFlags:       conf.AdditionalFlags,
			Debug:                 conf.Debug,
			SupportsUnstakedNodes: conf.SupportsUnstakedNodes,
		}

		confs = append(confs, containerConf)
	}

	return confs, nil
}

// runDKG simulates the distributed key generation process for all consensus nodes
// and returns all DKG data. This includes the group private key, node indices,
// and per-node public and private key-shares.
// Only consensus nodes participate in the DKG.
func runDKG(confs []ContainerConfig) (dkgmod.DKGData, error) {

	// filter by consensus nodes
	consensusNodes := bootstrap.FilterByRole(toNodeInfos(confs), flow.RoleConsensus)
	nConsensusNodes := len(consensusNodes)

	// run the core dkg algorithm
	dkgSeed, err := getSeed()
	if err != nil {
		return dkgmod.DKGData{}, err
	}

	dkg, err := dkg.RunFastKG(nConsensusNodes, dkgSeed)
	if err != nil {
		return dkgmod.DKGData{}, err
	}

	// sanity check
	if nConsensusNodes != len(dkg.PrivKeyShares) {
		return dkgmod.DKGData{}, fmt.Errorf(
			"consensus node count does not match DKG participant count: nodes=%d, participants=%d",
			nConsensusNodes,
			len(dkg.PrivKeyShares),
		)
	}

	return dkg, nil
}

// setupClusterGenesisBlockQCs generates bootstrapping resources necessary for each collector cluster:
//   * a cluster-specific root block
//   * a cluster-specific root QC
func setupClusterGenesisBlockQCs(nClusters uint, epochCounter uint64, confs []ContainerConfig) (flow.AssignmentList, []*flow.QuorumCertificate, error) {

	participants := toParticipants(confs)
	collectors := participants.Filter(filter.HasRole(flow.RoleCollection))
	assignments := unittest.ClusterAssignment(nClusters, collectors)
	clusters, err := flow.NewClusterList(assignments, collectors)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create cluster list: %w", err)
	}

	qcs := make([]*flow.QuorumCertificate, 0, nClusters)

	for _, cluster := range clusters {
		// generate root cluster block
		block := clusterstate.CanonicalRootBlock(epochCounter, cluster)

		lookup := make(map[flow.Identifier]struct{})
		for _, node := range cluster {
			lookup[node.NodeID] = struct{}{}
		}

		// gather cluster participants
		participants := make([]bootstrap.NodeInfo, 0, len(cluster))
		for _, conf := range confs {
			_, exists := lookup[conf.NodeID]
			if exists {
				participants = append(participants, conf.NodeInfo)
			}
		}
		if len(cluster) != len(participants) { // sanity check
			return nil, nil, fmt.Errorf("requiring a node info for each cluster participant")
		}

		// generate qc for root cluster block
		qc, err := run.GenerateClusterRootQC(participants, bootstrap.ToIdentityList(participants), block)
		if err != nil {
			return nil, nil, err
		}

		// add block and qc to list
		qcs = append(qcs, qc)
	}

	return assignments, qcs, nil
}

// writePrivateKeyFiles writes the staking and machine account private key files.
func writePrivateKeyFiles(bootstrapDir string, chainID flow.ChainID, nodeInfos []bootstrap.NodeInfo) error {

	// write private key files for each node (staking key, random beacon key, machine account key)
	//
	// for the machine account key, we keep track of the address index to map
	// the Flow address of the machine account to the key.
	addressIndex := uint64(4)
	for _, nodeInfo := range nodeInfos {
		fmt.Println("writing private files for ", nodeInfo.NodeID)
		path := filepath.Join(bootstrapDir, fmt.Sprintf(bootstrap.PathNodeInfoPriv, nodeInfo.NodeID))

		// retrieve private representation of the node
		private, err := nodeInfo.Private()
		if err != nil {
			return err
		}

		err = WriteJSON(path, private)
		if err != nil {
			return err
		}

		// We use the network key for the machine account. Normally it would be
		// a separate key.

		// Accounts are generated in a known order during bootstrapping, and
		// account addresses are deterministic based on order for a given chain
		// configuration. During the bootstrapping we create 4 Flow accounts besides
		// the service account (index 0) so node accounts will start at index 5.
		//
		// All nodes have a staking account created for them, only collection and
		// consensus nodes have a second machine account created.
		//
		// The accounts are created in the same order defined by the identity list
		// provided to BootstrapProcedure, which is the same order as this iteration.
		if nodeInfo.Role == flow.RoleCollection || nodeInfo.Role == flow.RoleConsensus {
			// increment the address index to account for both the staking account
			// and the machine account.
			// now addressIndex points to the machine account address index
			addressIndex += 2
		} else {
			// increment the address index to account for the staking account
			// we don't need to persist anything related to the staking account
			addressIndex += 1
			continue
		}

		accountAddress, err := chainID.Chain().AddressAtIndex(addressIndex)
		if err != nil {
			return err
		}

		info := bootstrap.NodeMachineAccountInfo{
			Address:           accountAddress.HexWithPrefix(),
			EncodedPrivateKey: private.NetworkPrivKey.Encode(),
			KeyIndex:          0,
			SigningAlgorithm:  private.NetworkPrivKey.Algorithm(),
			HashAlgorithm:     crypto.SHA3_256,
		}

		infoPath := filepath.Join(bootstrapDir, fmt.Sprintf(bootstrap.PathNodeMachineAccountInfoPriv, nodeInfo.NodeID))

		err = WriteJSON(infoPath, info)
		if err != nil {
			return err
		}
	}

	return nil
}
