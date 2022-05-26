package testnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"

	"github.com/dgraph-io/badger/v2"
	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/testingdock"

	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/module/metrics"
	state "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/inmem"
	storage "github.com/onflow/flow-go/storage/badger"
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
	ContainerName         string
	LogLevel              zerolog.Level
	Ghost                 bool
	AdditionalFlags       []string
	Debug                 bool
	SupportsUnstakedNodes bool
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

func NewContainerConfig(nodeName string, conf NodeConfig, networkKey, stakingKey crypto.PrivateKey) ContainerConfig {
	info := bootstrap.NewPrivateNodeInfo(
		conf.Identifier,
		conf.Role,
		GetPrivateNodeInfoAddress(nodeName),
		conf.Weight,
		networkKey,
		stakingKey,
	)

	containerConf := ContainerConfig{
		NodeInfo:              info,
		ContainerName:         nodeName,
		LogLevel:              conf.LogLevel,
		Ghost:                 conf.Ghost,
		AdditionalFlags:       conf.AdditionalFlags,
		Debug:                 conf.Debug,
		SupportsUnstakedNodes: conf.SupportsUnstakedNodes,
	}

	return containerConf
}

// ImageName returns the Docker image name for the given config.
func (c *ContainerConfig) ImageName() string {
	if c.Ghost {
		return defaultRegistry + "/ghost:latest"
	}
	debugSuffix := ""
	if c.Debug {
		debugSuffix = "-debug"
	}
	return fmt.Sprintf("%s/%s%s:latest", defaultRegistry, c.Role.String(), debugSuffix)
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

// Addr returns the host-accessible listening address of the container for the
// given port name. Panics if the port does not exist.
func (c *Container) Addr(portName string) string {
	port, ok := c.Ports[portName]
	if !ok {
		panic("could not find port " + portName)
	}
	return fmt.Sprintf(":%s", port)
}

// bindPort exposes the given container port and binds it to the given host port.
// If no protocol is specified, assumes TCP.
func (c *Container) bindPort(hostPort, containerPort string) {

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

// Name returns the container name. This is the name that appears in logs as
// well as the hostname that container can be reached at over the Docker network.
func (c *Container) Name() string {
	return c.opts.Name
}

// DB returns the node's database.
func (c *Container) DB() (*badger.DB, error) {
	opts := badger.
		DefaultOptions(c.DBPath()).
		WithKeepL0InMemory(true).
		WithLogger(nil)

	db, err := badger.Open(opts)
	return db, err
}

func (c *Container) DBPath() string {
	return filepath.Join(c.datadir, DefaultFlowDBDir)
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

	err := c.net.cli.ContainerStop(ctx, c.ID, &checkContainerTimeout)
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
	db, err := c.DB()
	if err != nil {
		return nil, err
	}

	metrics := metrics.NewNoopCollector()
	index := storage.NewIndex(metrics, db)
	headers := storage.NewHeaders(metrics, db)
	seals := storage.NewSeals(metrics, db)
	results := storage.NewExecutionResults(metrics, db)
	receipts := storage.NewExecutionReceipts(metrics, db, results, storage.DefaultCacheSize)
	guarantees := storage.NewGuarantees(metrics, db, storage.DefaultCacheSize)
	payloads := storage.NewPayloads(db, index, guarantees, seals, receipts, results)
	blocks := storage.NewBlocks(db, headers, payloads)
	setups := storage.NewEpochSetups(metrics, db)
	commits := storage.NewEpochCommits(metrics, db)
	statuses := storage.NewEpochStatuses(metrics, db)

	return state.OpenState(
		metrics,
		db,
		headers,
		seals,
		results,
		blocks,
		setups,
		commits,
		statuses,
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
