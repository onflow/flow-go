package testnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/dapperlabs/testingdock"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/onflow/crypto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sdk "github.com/onflow/flow-go-sdk"
	sdkclient "github.com/onflow/flow-go-sdk/access/grpc"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	ghostclient "github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/client"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	state "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

var (
	defaultRegistry       = "gcr.io/flow-container-registry"
	checkContainerTimeout = time.Second * 10
	checkContainerPeriod  = time.Millisecond * 50
)

func init() {
	registry := os.Getenv("CONTAINER_REGISTRY")
	if len(registry) > 0 {
		defaultRegistry = registry
	}
}

// ContainerConfig represents configuration for a node container in the network.
type ContainerConfig struct {
	bootstrap.NodeInfo
	// Corrupted indicates a container is running a binary implementing a malicious node
	Corrupted           bool
	ContainerName       string
	LogLevel            zerolog.Level
	Ghost               bool
	AdditionalFlags     []string
	Debug               bool
	EnableMetricsServer bool
}

func (c ContainerConfig) WriteKeyFiles(bootstrapDir string, machineAccountAddr sdk.Address, machineAccountKey encodable.MachineAccountPrivKey, role flow.Role) error {
	// write staking and machine account private key files
	writeJSONFile := func(relativePath string, val interface{}) error {
		return WriteJSON(filepath.Join(bootstrapDir, relativePath), val)
	}

	writeFile := func(relativePath string, data []byte) error {
		return WriteFile(filepath.Join(bootstrapDir, relativePath), data)
	}

	nodeInfos := []bootstrap.NodeInfo{c.NodeInfo}
	err := utils.WriteStakingNetworkingKeyFiles(nodeInfos, writeJSONFile)
	if err != nil {
		return fmt.Errorf("failed to write private key file: %w", err)
	}

	err = utils.WriteSecretsDBEncryptionKeyFiles(nodeInfos, writeFile)
	if err != nil {
		return fmt.Errorf("failed to write secrets db key file: %w", err)
	}

	if role == flow.RoleConsensus || role == flow.RoleCollection {
		err = utils.WriteMachineAccountFile(c.NodeID, machineAccountAddr, machineAccountKey, writeJSONFile)
		if err != nil {
			return fmt.Errorf("failed to write machine account file: %w", err)
		}
	}

	return nil
}

// GetPrivateNodeInfoAddress returns the node's address <name>:<port>
func GetPrivateNodeInfoAddress(nodeName string) string {
	return fmt.Sprintf("%s:%d", nodeName, DefaultFlowPort)
}

func NewContainerConfig(nodeName string, conf NodeConfig, networkKey, stakingKey crypto.PrivateKey,
) (ContainerConfig, error) {
	info, err := bootstrap.NewPrivateNodeInfo(
		conf.Identifier,
		conf.Role,
		GetPrivateNodeInfoAddress(nodeName),
		conf.Weight,
		networkKey,
		stakingKey,
	)
	if err != nil {
		return ContainerConfig{}, err
	}

	containerConf := ContainerConfig{
		NodeInfo:            info,
		ContainerName:       nodeName,
		LogLevel:            conf.LogLevel,
		Ghost:               conf.Ghost,
		AdditionalFlags:     conf.AdditionalFlags,
		Debug:               conf.Debug,
		EnableMetricsServer: conf.EnableMetricsServer,
		Corrupted:           conf.Corrupted,
	}

	return containerConf, nil
}

// ImageName returns the Docker image name for the given config.
func (c *ContainerConfig) ImageName() string {
	if c.Ghost {
		return defaultRegistry + "/ghost:latest"
	}
	imageSuffix := ""
	if c.Debug {
		imageSuffix = "-debug"
	} else if c.Corrupted {
		imageSuffix = "-corrupted"
	}

	return fmt.Sprintf("%s/%s%s:latest", defaultRegistry, c.Role.String(), imageSuffix)
}

// Container represents a test Docker container for a generic Flow node.
type Container struct {
	*testingdock.Container
	Config  ContainerConfig
	Ports   map[string]string // port mapping
	datadir string            // host directory bound to container's database
	net     *FlowNetwork      // reference to the network we are a part of
	opts    *testingdock.ContainerOpts
}

// Addr returns the host-accessible listening address of the container for the given container port.
// Panics if the port was not exposed.
func (c *Container) Addr(containerPort string) string {
	return fmt.Sprintf(":%s", c.Port(containerPort))
}

// ContainerAddr returns the container address for the provided port.
// Panics if the port was not exposed.
func (c *Container) ContainerAddr(containerPort string) string {
	return fmt.Sprintf("%s:%s", c.Name(), containerPort)
}

// Port returns the container's host port for the given container port.
// Panics if the port was not exposed.
func (c *Container) Port(containerPort string) string {
	port, ok := c.Ports[containerPort]
	if !ok {
		panic(fmt.Sprintf("port %s is not registered for %s", containerPort, c.Config.ContainerName))
	}
	return port
}

// exposePort exposes the given container port and binds it to the given host port.
// If no protocol is specified, assumes TCP.
func (c *Container) exposePort(containerPort, hostPort string) {
	// keep track of port mapping for easy lookups
	c.Ports[containerPort] = hostPort

	// use TCP protocol if none specified
	containerNATPort := nat.Port(containerPort)
	if containerNATPort.Proto() == "" {
		containerNATPort = nat.Port(fmt.Sprintf("%s/tcp", containerPort))
	}

	if c.opts.Config.ExposedPorts == nil {
		c.opts.Config.ExposedPorts = nat.PortSet{
			containerNATPort: {},
		}
	} else {
		c.opts.Config.ExposedPorts[containerNATPort] = struct{}{}
	}

	if c.opts.HostConfig.PortBindings == nil {
		c.opts.HostConfig.PortBindings = nat.PortMap{
			containerNATPort: []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: hostPort,
				},
			},
		}
	} else {
		c.opts.HostConfig.PortBindings[containerNATPort] = []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: hostPort,
			},
		}
	}

}

// AddFlag adds a command line flag to the container's startup command.
func (c *Container) AddFlag(flag, val string) {
	c.opts.Config.Cmd = append(
		c.opts.Config.Cmd,
		fmt.Sprintf("--%s=%s", flag, val),
	)
}

func (c Container) IsFlagSet(flag string) bool {
	for _, cmd := range c.opts.Config.Cmd {
		if strings.HasPrefix(cmd, fmt.Sprintf("--%s", flag)) {
			return true
		}
	}

	return false
}

// Name returns the container name. This is the name that appears in logs as
// well as the hostname that container can be reached at over the Docker network.
func (c *Container) Name() string {
	return c.opts.Name
}

// DB returns the node's database.
func (c *Container) DB() (storage.DB, error) {
	pdb, err := storagepebble.SafeOpen(unittest.Logger(), c.DBPath())
	if err != nil {
		return nil, err
	}
	return pebbleimpl.ToDB(pdb), nil
}

// DB returns the node's execution data database.
func (c *Container) ExecutionDataDB() (*pebble.DB, error) {
	return storagepebble.SafeOpen(unittest.Logger(), c.ExecutionDataDBPath())
}

func (c *Container) DBPath() string {
	return filepath.Join(c.datadir, DefaultFlowPebbleDBDir)
}

func (c *Container) ExecutionDataDBPath() string {
	return filepath.Join(c.datadir, DefaultExecutionDataServiceDir)
}

func (c *Container) BootstrapPath() string {
	return filepath.Join(c.datadir, DefaultBootstrapDir)
}

// DropDB resets the node's database.
func (c *Container) DropDB() {
	err := os.RemoveAll(c.DBPath())
	require.NoError(c.net.t, err)
	err = os.Mkdir(c.DBPath(), 0700)
	require.NoError(c.net.t, err)
}

// WriteRootSnapshot overwrites the root protocol state snapshot file with the
// provided state snapshot.
func (c *Container) WriteRootSnapshot(snap *inmem.Snapshot) {
	rootSnapshotPath := filepath.Join(c.BootstrapPath(), bootstrap.PathRootProtocolStateSnapshot)
	err := WriteJSON(rootSnapshotPath, snap.Encodable())
	require.NoError(c.net.t, err)
}

// Pause stops this container temporarily, preserving its state. It can be
// re-started with Start.
func (c *Container) Pause() error {

	ctx, cancel := context.WithTimeout(context.Background(), checkContainerTimeout)
	defer cancel()

	timeout := int(checkContainerTimeout.Seconds())
	err := c.net.cli.ContainerStop(ctx, c.ID,
		container.StopOptions{
			Timeout: &timeout,
		})
	if err != nil {
		return fmt.Errorf("could not stop container with ID (%s): %w", c.ID, err)
	}

	err = c.waitForCondition(ctx, containerStopped)
	if err != nil {
		return fmt.Errorf("error waiting for container to stop: %w", err)
	}

	return nil
}

// Start starts this container that has been stopped temporarily with Pause,
// preserving existing state.
func (c *Container) Start() error {

	ctx, cancel := context.WithTimeout(context.Background(), checkContainerTimeout)
	defer cancel()

	err := c.net.cli.ContainerStart(ctx, c.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("could not start container: %w", err)
	}

	err = c.waitForCondition(ctx, containerRunning)
	if err != nil {
		return fmt.Errorf("error waiting for container to start: %w", err)
	}

	return nil
}

// Disconnect disconnects this container from the network.
func (c *Container) Disconnect() error {

	ctx, cancel := context.WithTimeout(context.Background(), checkContainerTimeout)
	defer cancel()

	err := c.net.cli.NetworkDisconnect(ctx, c.net.network.ID(), c.ID, false)
	if err != nil {
		return fmt.Errorf("could not disconnect container (%s) from network: %w", c.Name(), err)
	}

	err = c.waitForCondition(ctx, containerDisconnected)
	if err != nil {
		return fmt.Errorf("error waiting for container to disconnect: %w", err)
	}

	return nil
}

// WaitForContainerStopped waits until the container is stopped
func (c *Container) WaitForContainerStopped(timeout time.Duration) error {

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := c.waitForCondition(ctx, containerStopped)
	if err != nil {
		return fmt.Errorf("error waiting for container stopped: %w", err)
	}

	return nil
}

// Connect connects this container to the network.
func (c *Container) Connect() error {

	ctx, cancel := context.WithTimeout(context.Background(), checkContainerTimeout)
	defer cancel()

	err := c.net.cli.NetworkConnect(ctx, c.net.network.ID(), c.ID, nil)
	if err != nil {
		return fmt.Errorf("could not connect container (%s) to network: %w", c.Name(), err)
	}

	err = c.waitForCondition(ctx, containerConnected)
	if err != nil {
		return fmt.Errorf("error waiting for container to connect: %w", err)
	}

	return nil
}

func (c *Container) OpenState() (*state.State, error) {
	lockManager := storage.NewTestingLockManager()
	db, err := c.DB()
	if err != nil {
		return nil, err
	}
	metrics := metrics.NewNoopCollector()
	index := store.NewIndex(metrics, db)
	headers := store.NewHeaders(metrics, db)
	seals := store.NewSeals(metrics, db)
	results := store.NewExecutionResults(metrics, db)
	receipts := store.NewExecutionReceipts(metrics, db, results, store.DefaultCacheSize)
	guarantees := store.NewGuarantees(metrics, db, store.DefaultCacheSize, store.DefaultCacheSize)
	payloads := store.NewPayloads(db, index, guarantees, seals, receipts, results)
	blocks := store.NewBlocks(db, headers, payloads)
	qcs := store.NewQuorumCertificates(metrics, db, store.DefaultCacheSize)
	setups := store.NewEpochSetups(metrics, db)
	commits := store.NewEpochCommits(metrics, db)
	protocolState := store.NewEpochProtocolStateEntries(metrics, setups, commits, db,
		store.DefaultEpochProtocolStateCacheSize, store.DefaultProtocolStateIndexCacheSize)
	protocolKVStates := store.NewProtocolKVStore(metrics, db,
		store.DefaultProtocolKVStoreCacheSize, store.DefaultProtocolKVStoreByBlockIDCacheSize)
	versionBeacons := store.NewVersionBeacons(db)

	return state.OpenState(
		metrics,
		db,
		lockManager,
		headers,
		seals,
		results,
		blocks,
		qcs,
		setups,
		commits,
		protocolState,
		protocolKVStates,
		versionBeacons,
	)
}

// containerStopped returns true if the container is not running.
func containerStopped(state *types.ContainerJSON) bool {
	return !state.State.Running
}

// containerRunning returns true if the container is running.
func containerRunning(state *types.ContainerJSON) bool {
	return state.State.Running
}

// containerDisconnected returns true if the container is not connected to a
// network.
func containerDisconnected(state *types.ContainerJSON) bool {
	return len(state.NetworkSettings.Networks) == 0
}

// containerConnected returns true if the container is connected to a network.
func containerConnected(state *types.ContainerJSON) bool {
	return len(state.NetworkSettings.Networks) == 1
}

// waitForCondition waits for the given condition to be true, checking the
// condition with an exponential backoff. Returns an error if inspecting fails
// or when the context expires. Returns nil when the condition is true.
func (c *Container) waitForCondition(ctx context.Context, condition func(*types.ContainerJSON) bool) error {

	retryAfter := checkContainerPeriod
	for {
		res, err := c.net.cli.ContainerInspect(ctx, c.ID)
		if err != nil {
			return fmt.Errorf("could not inspect container: %w", err)
		}
		if condition(&res) {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("condition not met after timeout (%s)", checkContainerTimeout.String())
		case <-time.After(retryAfter):
			retryAfter *= 2
			continue
		}
	}
}

// TestnetClient returns a testnet client that connects to this node.
func (c *Container) TestnetClient() (*Client, error) {
	if c.Config.Role != flow.RoleAccess && c.Config.Role != flow.RoleCollection {
		return nil, fmt.Errorf("container does not implement flow.access.AccessAPI")
	}

	chain := c.net.Root().ChainID.Chain()
	return NewClient(c.Addr(GRPCPort), chain)
}

// SDKClient returns a flow-go-sdk client that connects to this node.
func (c *Container) SDKClient() (*sdkclient.Client, error) {
	if c.Config.Role != flow.RoleAccess && c.Config.Role != flow.RoleCollection {
		return nil, fmt.Errorf("container does not implement flow.access.AccessAPI")
	}

	return sdkclient.NewClient(
		c.Addr(GRPCPort),
		sdkclient.WithGRPCDialOptions(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)
}

// GhostClient returns a ghostnode client that connects to this node.
func (c *Container) GhostClient() (*ghostclient.GhostClient, error) {
	if !c.Config.Ghost {
		return nil, fmt.Errorf("container is not a ghost node")
	}

	return ghostclient.NewGhostClient(c.Addr(GRPCPort))
}

// HealthcheckCallback returns a Docker healthcheck function that pings the node's GRPC
// service exposed at the given port.
func (c *Container) HealthcheckCallback() func() error {
	return func() error {
		fmt.Printf("healthchecking %s...", c.Name())

		ctx := context.Background()

		// The admin server starts last, so it's a rough approximation of the node being ready.
		adminAddress := fmt.Sprintf("localhost:%s", c.Port(AdminPort))
		err := client.NewAdminClient(adminAddress).Ping(ctx)
		if err != nil {
			return fmt.Errorf("could not ping admin server: %w", err)
		}

		// also ping the GRPC server if it's enabled
		if _, ok := c.Ports[GRPCPort]; !ok {
			return nil
		}

		switch c.Config.Role {
		case flow.RoleExecution:
			apiClient, err := client.NewExecutionClient(c.Addr(GRPCPort))
			if err != nil {
				return fmt.Errorf("could not create execution client: %w", err)
			}
			defer apiClient.Close()

			return apiClient.Ping(ctx)

		default:
			apiClient, err := client.NewAccessClient(c.Addr(GRPCPort))
			if err != nil {
				return fmt.Errorf("could not create access client: %w", err)
			}
			defer apiClient.Close()

			return apiClient.Ping(ctx)
		}
	}
}
