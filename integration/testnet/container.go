package testnet

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dapperlabs/testingdock"
	"github.com/dgraph-io/badger/v2"
	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"

	"github.com/dapperlabs/flow-go/model/bootstrap"
)

var (
	checkContainerTimeout = time.Second * 10
	checkContainerPeriod  = time.Millisecond * 50
)

// ContainerConfig represents configuration for a node container in the network.
type ContainerConfig struct {
	bootstrap.NodeInfo
	ContainerName string
	LogLevel      string
}

// ImageName returns the Docker image name for the given config.
func (c *ContainerConfig) ImageName() string {
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

// Client returns a Access API to the given container on the given port.
func (c *Container) Client(portName string) (*Client, error) {
	port, ok := c.Ports[portName]
	if !ok {
		return nil, fmt.Errorf("could not find port (name=%s)", portName)
	}

	return NewClient(fmt.Sprintf(":%s", port))
}

// bindPort exposes the given container port and binds it to the given host port.
// If no protocol is specified, assumes TCP.
func (c *Container) bindPort(hostPort, containerPort string) {

	// use TCP protocol if none specified
	containerNATPort := nat.Port(containerPort)
	if containerNATPort.Proto() == "" {
		containerNATPort = nat.Port(fmt.Sprintf("%s/tcp", containerPort))
	}

	c.opts.Config.ExposedPorts = nat.PortSet{
		containerNATPort: {},
	}
	c.opts.HostConfig.PortBindings = nat.PortMap{
		containerNATPort: []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: hostPort,
			},
		},
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
	db, err := badger.Open(badger.DefaultOptions(dbPath).WithLogger(nil))
	return db, err
}

// Stop stops this container temporarily, preserving its state. It can be
// re-started with Start.
func (c *Container) Stop() error {
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

// Start starts this container that has been stopped temporarily with Stop,
// preserving existing state.
func (c *Container) Start() error {
	ctx, cancel := context.WithTimeout(context.Background(), checkContainerTimeout)
	defer cancel()

	err := c.net.cli.ContainerStart(ctx, c.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("could not stop container: %w", err)
	}

	err = c.waitForCondition(ctx, containerRunning)
	if err != nil {
		return fmt.Errorf("error waiting for container to stop: %w", err)
	}

	return nil
}

// Disconnect disconnects this container from the network.
func (c *Container) Disconnect() error {
	// TODO
	panic("not implemented")
}

// Connect connects this container to the network.
func (net *FlowNetwork) Connect() error {
	// TODO
	panic("not implemented")
}

// containerStopped returns true if the container is not running.
func containerStopped(state *types.ContainerJSON) bool {
	return !state.State.Running
}

// containerRunning returns true if the container is running.
func containerRunning(state *types.ContainerJSON) bool {
	return state.State.Running
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
