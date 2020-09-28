package testnet

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/testingdock"

	"github.com/onflow/flow-go/model/bootstrap"
)

var (
	checkContainerTimeout = time.Second * 10
	checkContainerPeriod  = time.Millisecond * 50
)

// ContainerConfig represents configuration for a node container in the network.
type ContainerConfig struct {
	bootstrap.NodeInfo
	ContainerName   string
	LogLevel        zerolog.Level
	Ghost           bool
	AdditionalFlags []string
}

// ImageName returns the Docker image name for the given config.
func (c *ContainerConfig) ImageName() string {
	if c.Ghost {
		return "gcr.io/dl-flow/ghost:latest"
	}
	return fmt.Sprintf("gcr.io/dl-flow/%s:latest", c.Role.String())
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

// addFlag adds a command line flag to the container's startup command.
func (c *Container) addFlag(flag, val string) {
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
	dbPath := filepath.Join(c.datadir, DefaultFlowDBDir)
	opts := badger.
		DefaultOptions(dbPath).
		WithKeepL0InMemory(true).
		WithLogger(nil)

	db, err := badger.Open(opts)
	return db, err
}

// Pause stops this container temporarily, preserving its state. It can be
// re-started with Start.
func (c *Container) Pause() error {

	ctx, cancel := context.WithTimeout(context.Background(), checkContainerTimeout)
	defer cancel()

	err := c.net.cli.ContainerStop(ctx, c.ID, &checkContainerTimeout)
	if err != nil {
		return fmt.Errorf("could not stop container: %w", err)
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
