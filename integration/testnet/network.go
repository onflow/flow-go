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

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/testingdock"

	bootstrapcmd "github.com/dapperlabs/flow-go/cmd/bootstrap/cmd"
	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	bootstraprun "github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	hotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const (
	// TmpRoot is the default root directory to create temporary data
	// directories for containers. We use /tmp because $TMPDIR is not exposed
	// to docker by default on macOS
	TmpRoot = "/tmp"

	// DefaultBootstrapDir is the default directory for bootstrap files
	DefaultBootstrapDir = "/bootstrap"

	// DefaultFlowDBDir is the default directory for the node database.
	DefaultFlowDBDir = "/flowdb"
	// DefaultExecutionRootDir is the default directory for the execution node
	// state database.
	DefaultExecutionRootDir = "/exedb"

	// ColNodeAPIPort is the name used for the collection node API port.
	ColNodeAPIPort = "col-ingress-port"
	// ExeNodeAPIPort is the name used for the execution node API port.
	ExeNodeAPIPort = "exe-api-port"
	// AccessNodeAPIPort is the name used for the access node API port.
	AccessNodeAPIPort = "access-api-port"
	// GhostNodeAPIPort is the name used for the access node API port.
	GhostNodeAPIPort = "ghost-api-port"

	// ExeNodeMetricsPort
	ExeNodeMetricsPort = "exe-metrics-port"
)

func init() {
	testingdock.Verbose = true
}

// FlowNetwork represents a test network of Flow nodes running in Docker containers.
type FlowNetwork struct {
	t           *testing.T
	suite       *testingdock.Suite
	config      NetworkConfig
	cli         *dockerclient.Client
	network     *testingdock.Network
	Containers  map[string]*Container
	AccessPorts map[string]string
	genesis     flow.Block
}

// Identities returns a list of identities, one for each node in the network.
func (net *FlowNetwork) Identities() flow.IdentityList {
	il := make(flow.IdentityList, 0, len(net.Containers))
	for _, c := range net.Containers {
		il = append(il, c.Config.Identity())
	}
	return il
}

// Genesis returns the genesis block generated for the network.
func (net *FlowNetwork) Genesis() flow.Block {
	return net.genesis
}

// Start starts the network.
func (net *FlowNetwork) Start(ctx context.Context) {
	// makes it easier to see logs for a specific test case
	fmt.Println(">>>> starting network: ", net.config.Name)
	net.suite.Start(ctx)
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
// restarted. To remove them, call `RemoveContainers`.
func (net *FlowNetwork) StopContainers() {

	err := net.suite.Close()
	if err != nil {
		net.t.Log("failed to stop network", err)
	}
}

// RemoveContainers removes all the containers in the network. Containers need to be stopped first using `Stop`.
func (net *FlowNetwork) RemoveContainers() {

	err := net.suite.Remove()
	if err != nil {

	}
}

// Cleanup cleans up all temporary files used by the network.
func (net *FlowNetwork) Cleanup() {

	// remove data directories
	for _, c := range net.Containers {
		err := os.RemoveAll(c.datadir)
		if err != nil {
			net.t.Log("failed to cleanup", err)
		}
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

// ContainerByName returns the container with the given name, if it exists.
// Otherwise fails the test.
func (net *FlowNetwork) ContainerByName(name string) *Container {
	container, exists := net.Containers[name]
	if !exists {
		net.t.FailNow()
	}
	return container
}

// NetworkConfig is the config for the network.
type NetworkConfig struct {
	Nodes     []NodeConfig
	Name      string
	NClusters uint
}

func NewNetworkConfig(name string, nodes []NodeConfig, opts ...func(*NetworkConfig)) NetworkConfig {
	c := NetworkConfig{
		Nodes:     nodes,
		Name:      name,
		NClusters: 1, // default to 1 cluster
	}

	for _, apply := range opts {
		apply(&c)
	}

	return c
}

func WithClusters(n uint) func(*NetworkConfig) {
	return func(conf *NetworkConfig) {
		conf.NClusters = n
	}
}

func (n *NetworkConfig) Len() int {
	return len(n.Nodes)
}

func (n *NetworkConfig) Less(i, j int) bool {
	return n.Nodes[i].Role < n.Nodes[j].Role
}

func (n *NetworkConfig) Swap(i, j int) {
	n.Nodes[i], n.Nodes[j] = n.Nodes[j], n.Nodes[i]
}

// NodeConfig defines the input config for a particular node, specified prior
// to network creation.
type NodeConfig struct {
	Role            flow.Role
	Stake           uint64
	Identifier      flow.Identifier
	LogLevel        zerolog.Level
	Ghost           bool
	AdditionalFlags []string
}

func NewNodeConfig(role flow.Role, opts ...func(*NodeConfig)) NodeConfig {
	c := NodeConfig{
		Role:       role,
		Stake:      1000,                         // default stake
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

func AsGhost() func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.Ghost = true
	}
}

// WithAdditionalFlag adds additional flags to the command
func WithAdditionalFlag(flag string) func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.AdditionalFlags = append(config.AdditionalFlags, flag)
	}
}

func PrepareFlowNetwork(t *testing.T, networkConf NetworkConfig) *FlowNetwork {

	// number of nodes
	nNodes := len(networkConf.Nodes)
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

	// create a temporary directory to store all bootstrapping files, these
	// will be shared between all nodes
	bootstrapDir, err := ioutil.TempDir(TmpRoot, "flow-integration-bootstrap")
	require.Nil(t, err)

	genesis, confs, err := BootstrapNetwork(networkConf, bootstrapDir)
	require.Nil(t, err)

	flowNetwork := &FlowNetwork{
		t:           t,
		cli:         dockerClient,
		config:      networkConf,
		suite:       suite,
		network:     network,
		Containers:  make(map[string]*Container, nNodes),
		AccessPorts: make(map[string]string),
		genesis:     *genesis,
	}

	// add each node to the network
	for _, nodeConf := range confs {
		err = flowNetwork.AddNode(t, bootstrapDir, nodeConf)
		require.NoError(t, err)
	}

	return flowNetwork
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
				fmt.Sprintf("--nodeid=%s", nodeConf.NodeID.String()),
				fmt.Sprintf("--bootstrapdir=%s", DefaultBootstrapDir),
				fmt.Sprintf("--datadir=%s", DefaultFlowDBDir),
				fmt.Sprintf("--loglevel=%s", nodeConf.LogLevel.String()),
				fmt.Sprintf("--nclusters=%d", net.config.NClusters),
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

	nodeContainer := &Container{
		Config:  nodeConf,
		Ports:   make(map[string]string),
		datadir: tmpdir,
		net:     net,
		opts:    opts,
	}

	// create a directory for the node database
	flowDBDir := filepath.Join(tmpdir, DefaultFlowDBDir)
	err = os.Mkdir(flowDBDir, 0700)
	require.NoError(t, err)

	// Bind the host directory to the container's database directory
	// Bind the common bootstrap directory to the container
	// NOTE: I did this using the approach from:
	// https://github.com/fsouza/go-dockerclient/issues/132#issuecomment-50694902
	opts.HostConfig.Binds = append(
		opts.HostConfig.Binds,
		fmt.Sprintf("%s:%s:rw", flowDBDir, DefaultFlowDBDir),
		fmt.Sprintf("%s:%s:ro", bootstrapDir, DefaultBootstrapDir),
	)

	if !nodeConf.Ghost {
		switch nodeConf.Role {
		case flow.RoleCollection:

			hostPort := testingdock.RandomPort(t)
			containerPort := "9000/tcp"

			nodeContainer.bindPort(hostPort, containerPort)

			nodeContainer.addFlag("ingress-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))
			nodeContainer.Ports[ColNodeAPIPort] = hostPort
			nodeContainer.opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckAccessGRPC(hostPort))
			net.AccessPorts[ColNodeAPIPort] = hostPort

		case flow.RoleExecution:

			hostPort := testingdock.RandomPort(t)
			containerPort := "9000/tcp"

			nodeContainer.bindPort(hostPort, containerPort)

			hostMetricsPort := testingdock.RandomPort(t)
			containerMetricsPort := "8080/tcp"

			nodeContainer.bindPort(hostMetricsPort, containerMetricsPort)

			nodeContainer.addFlag("rpc-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))
			if !nodeContainer.Config.Ghost {
			}
			nodeContainer.Ports[ExeNodeAPIPort] = hostPort
			nodeContainer.opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckExecutionGRPC(hostPort))
			net.AccessPorts[ExeNodeAPIPort] = hostPort

			nodeContainer.Ports[ExeNodeMetricsPort] = hostMetricsPort
			net.AccessPorts[ExeNodeMetricsPort] = hostMetricsPort

			// create directories for execution state trie and values in the tmp
			// host directory.
			tmpLedgerDir, err := ioutil.TempDir(tmpdir, "flow-integration-trie")
			require.NoError(t, err)

			opts.HostConfig.Binds = append(
				opts.HostConfig.Binds,
				fmt.Sprintf("%s:%s:rw", tmpLedgerDir, DefaultExecutionRootDir),
			)

			nodeContainer.addFlag("triedir", DefaultExecutionRootDir)

		case flow.RoleAccess:
			hostPort := testingdock.RandomPort(t)
			containerPort := "9000/tcp"

			nodeContainer.bindPort(hostPort, containerPort)

			nodeContainer.addFlag("rpc-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))
			// Should always have at least 1 collection and execution node
			nodeContainer.addFlag("ingress-addr", "collection_1:9000")
			nodeContainer.addFlag("script-addr", "execution_1:9000")
			nodeContainer.opts.HealthCheck = testingdock.HealthCheckCustom(healthcheckAccessGRPC(hostPort))
			nodeContainer.Ports[AccessNodeAPIPort] = hostPort
			net.AccessPorts[AccessNodeAPIPort] = hostPort

		case flow.RoleVerification:
			nodeContainer.addFlag("alpha", "1")
		}
	} else {
		hostPort := testingdock.RandomPort(t)
		containerPort := "9000/tcp"

		nodeContainer.addFlag("rpc-addr", fmt.Sprintf("%s:9000", nodeContainer.Name()))
		nodeContainer.bindPort(hostPort, containerPort)
		nodeContainer.Ports[GhostNodeAPIPort] = hostPort
	}

	suiteContainer := net.suite.Container(*opts)
	nodeContainer.Container = suiteContainer
	net.Containers[nodeContainer.Name()] = nodeContainer
	if nodeConf.Role == flow.RoleAccess {
		// collection1, _ := net.ContainerByName("collection_1")
		execution1 := net.ContainerByName("execution_1")
		// collection1.After(suiteContainer)
		execution1.After(suiteContainer)
	} else {
		net.network.After(suiteContainer)
	}
	return nil
}

func BootstrapNetwork(networkConf NetworkConfig, bootstrapDir string) (*flow.Block, []ContainerConfig, error) {
	// number of nodes
	nNodes := len(networkConf.Nodes)
	if nNodes == 0 {
		return nil, nil, fmt.Errorf("must specify at least one node")
	}

	// Sort so that access nodes start up last
	sort.Sort(&networkConf)

	// generate staking and networking keys for each configured node
	confs, err := setupKeys(networkConf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to setup keys: %w", err)
	}

	// run DKG for all consensus nodes
	dkg, err := runDKG(confs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to run DKG: %w", err)
	}

	// generate the root account
	hardcoded, err := hex.DecodeString(flow.RootAccountPrivateKeyHex)
	if err != nil {
		return nil, nil, err
	}

	account, err := flow.DecodeAccountPrivateKey(hardcoded)
	if err != nil {
		return nil, nil, err
	}

	// generate the initial execution state
	commit, err := run.GenerateExecutionState(filepath.Join(bootstrapDir, bootstrap.DirnameExecutionState), account)
	if err != nil {
		return nil, nil, err
	}

	// generate genesis block
	genesis := bootstraprun.GenerateRootBlock(toIdentityList(confs))

	// generate QC
	nodeInfos := bootstrap.FilterByRole(toNodeInfoList(confs), flow.RoleConsensus)
	signerData := bootstrapcmd.GenerateQCParticipantData(nodeInfos, nodeInfos, dkg)

	qc, err := bootstraprun.GenerateGenesisQC(signerData, genesis)
	if err != nil {
		return nil, nil, err
	}

	// write common genesis bootstrap files
	err = writeJSON(filepath.Join(bootstrapDir, bootstrap.PathAccount0Priv), account)
	if err != nil {
		return nil, nil, err
	}

	err = writeJSON(filepath.Join(bootstrapDir, bootstrap.PathGenesisCommit), commit)
	if err != nil {
		return nil, nil, err
	}

	err = writeJSON(filepath.Join(bootstrapDir, bootstrap.PathGenesisBlock), genesis)
	if err != nil {
		return nil, nil, err
	}

	err = writeJSON(filepath.Join(bootstrapDir, bootstrap.PathGenesisQC), qc)
	if err != nil {
		return nil, nil, err
	}

	err = writeJSON(filepath.Join(bootstrapDir, bootstrap.PathDKGDataPub), dkg.Public())
	if err != nil {
		return nil, nil, err
	}

	// write private key files for each DKG participant
	for _, part := range dkg.Participants {
		path := fmt.Sprintf(bootstrap.PathRandomBeaconPriv, part.NodeID)
		err = writeJSON(filepath.Join(bootstrapDir, path), part.Private())
		if err != nil {
			return nil, nil, err
		}
	}

	// write private key files for each node
	for _, nodeConfig := range confs {
		path := filepath.Join(bootstrapDir, fmt.Sprintf(bootstrap.PathNodeInfoPriv, nodeConfig.NodeID))

		// retrieve private representation of the node
		private, err := nodeConfig.NodeInfo.Private()
		if err != nil {
			return nil, nil, err
		}

		err = writeJSON(path, private)
		if err != nil {
			return nil, nil, err
		}
	}

	// generate genesis blocks for each collector cluster
	clusterBlocks, clusterQCs, err := setupClusterGenesisBlockQCs(networkConf.NClusters, confs, genesis)
	if err != nil {
		return nil, nil, err
	}

	// write collector-specific genesis bootstrap files for each cluster
	for i := 0; i < len(clusterBlocks); i++ {
		clusterGenesis := clusterBlocks[i]
		clusterQC := clusterQCs[i]

		// cluster ID is equivalent to chain ID
		clusterID := clusterGenesis.Header.ChainID

		clusterGenesisPath := fmt.Sprintf(bootstrap.PathGenesisClusterBlock, clusterID)
		err = writeJSON(filepath.Join(bootstrapDir, clusterGenesisPath), clusterGenesis)
		if err != nil {
			return nil, nil, err
		}

		clusterQCPath := fmt.Sprintf(bootstrap.PathGenesisClusterQC, clusterID)
		err = writeJSON(filepath.Join(bootstrapDir, clusterQCPath), clusterQC)
		if err != nil {
			return nil, nil, err
		}
	}

	return genesis, confs, nil
}

// setupKeys generates private staking and networking keys for each configured
// node. It also assigns each node a unique container name and network address.
func setupKeys(networkConf NetworkConfig) ([]ContainerConfig, error) {

	nNodes := len(networkConf.Nodes)

	// keep track of how many roles we have assigned so we can number containers
	// correctly (consensus_1, consensus_2, etc.)
	roleCounter := make(map[flow.Role]int)

	// get networking keys for all nodes
	networkKeys, err := unittest.NetworkingKeys(nNodes)
	if err != nil {
		return nil, err
	}

	// get staking keys for all nodes
	stakingKeys, err := unittest.StakingKeys(nNodes)
	if err != nil {
		return nil, err
	}

	// create node container configs and corresponding public identities
	confs := make([]ContainerConfig, 0, nNodes)
	for i, conf := range networkConf.Nodes {

		// define the node's name <role>_<n> and address <name>:<port>
		name := fmt.Sprintf("%s_%d", conf.Role.String(), roleCounter[conf.Role]+1)

		addr := fmt.Sprintf("%s:%d", name, 2137)
		roleCounter[conf.Role]++

		info := bootstrap.NewPrivateNodeInfo(
			conf.Identifier,
			conf.Role,
			addr,
			conf.Stake,
			networkKeys[i],
			stakingKeys[i],
		)

		containerConf := ContainerConfig{
			NodeInfo:        info,
			ContainerName:   name,
			LogLevel:        conf.LogLevel,
			Ghost:           conf.Ghost,
			AdditionalFlags: conf.AdditionalFlags,
		}

		confs = append(confs, containerConf)
	}

	return confs, nil
}

// runDKG runs the distributed key generation process for all consensus nodes
// and returns all DKG data. This includes the group private key, node indices,
// and per-node public and private key-shares.
// Only consensus nodes are participate in the DKG.
func runDKG(confs []ContainerConfig) (bootstrap.DKGData, error) {

	// filter by consensus nodes
	consensusNodes := bootstrap.FilterByRole(toNodeInfoList(confs), flow.RoleConsensus)
	nConsensusNodes := len(consensusNodes)

	// run the core dkg algorithm
	dkgSeed, err := getSeed()
	if err != nil {
		return bootstrap.DKGData{}, err
	}

	dkg, err := bootstraprun.RunFastKG(nConsensusNodes, dkgSeed)
	if err != nil {
		return bootstrap.DKGData{}, err
	}

	// sanity check
	if nConsensusNodes != len(dkg.Participants) {
		return bootstrap.DKGData{}, fmt.Errorf(
			"consensus node count does not match DKG participant count: nodes=%d, participants=%d",
			nConsensusNodes,
			len(dkg.Participants),
		)
	}

	// set the node IDs in the dkg data
	for i := range dkg.Participants {
		nodeID := consensusNodes[i].NodeID
		dkg.Participants[i].NodeID = nodeID
	}

	return dkg, nil
}

// setupClusterGenesisBlockQCs generates bootstrapping resources necessary for each collector cluster:
//   * a cluster-specific genesis block
//   * a cluster-specific genesis QC
func setupClusterGenesisBlockQCs(nClusters uint, confs []ContainerConfig, genesis *flow.Block) ([]*cluster.Block, []*hotstuff.QuorumCertificate, error) {

	identities := toIdentityList(confs)
	clusters := protocol.Clusters(nClusters, identities)

	blocks := make([]*cluster.Block, 0, nClusters)
	qcs := make([]*hotstuff.QuorumCertificate, 0, nClusters)

	for _, cluster := range clusters.All() {
		// generate genesis cluster block
		block := bootstraprun.GenerateGenesisClusterBlock(cluster)

		// gather cluster participants
		// ToDo: optimize. This has quadratic scaling with the number of collectors:
		//       Let N be the number of collectors. The number of clusters Xi = N / c where c is nearly a constant.
		//       Furthermore, cluster.ByNodeID iterate over all c cluster members to check whether their ID matches.
		//       Hence, we get a runtime cost: Xi * N * c = N^2. This could probably be reduced to linear cost of O(N).
		participants := make([]bootstrap.NodeInfo, 0, len(cluster))
		for _, conf := range confs {
			_, exists := cluster.ByNodeID(conf.NodeID)
			if exists {
				participants = append(participants, conf.NodeInfo)
			}
		}
		if len(cluster) != len(participants) { // sanity check
			return nil, nil, fmt.Errorf("requiring a node info for each cluster participant")
		}

		// generate qc for genesis cluster block
		qc, err := bootstraprun.GenerateClusterGenesisQC(participants, genesis, block)
		if err != nil {
			return nil, nil, err
		}

		// add block and qc to list
		blocks = append(blocks, block)
		qcs = append(qcs, qc)
	}

	return blocks, qcs, nil
}
