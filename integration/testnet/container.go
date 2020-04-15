package testnet

import (
	"fmt"
	"path/filepath"

	"github.com/dapperlabs/testingdock"
	"github.com/dgraph-io/badger/v2"
	"github.com/docker/go-connections/nat"

	"github.com/dapperlabs/flow-go/model/bootstrap"
)

// ContainerConfig represents configuration for a node container in the network.
type ContainerConfig struct {
	bootstrap.NodeInfo
	ContainerName string
	LogLevel      string
	Ghost         bool
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
	DataDir string            // host directory bound to container's database
	Opts    *testingdock.ContainerOpts
}

// bindPort exposes the given container port and binds it to the given host port.
// If no protocol is specified, assumes TCP.
func (c *Container) bindPort(hostPort, containerPort string) {

	// use TCP protocol if none specified
	containerNATPort := nat.Port(containerPort)
	if containerNATPort.Proto() == "" {
		containerNATPort = nat.Port(fmt.Sprintf("%s/tcp", containerPort))
	}

	c.Opts.Config.ExposedPorts = nat.PortSet{
		containerNATPort: {},
	}
	c.Opts.HostConfig.PortBindings = nat.PortMap{
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
	c.Opts.Config.Cmd = append(
		c.Opts.Config.Cmd,
		fmt.Sprintf("--%s=%s", flag, val),
	)
}

// Name returns the container name. This is the name that appears in logs as
// well as the hostname that container can be reached at over the Docker network.
func (c *Container) Name() string {
	return c.Opts.Name
}

// DB returns the node's database.
func (c *Container) DB() (*badger.DB, error) {
	dbPath := filepath.Join(c.DataDir, DefaultFlowDBDir)
	db, err := badger.Open(badger.DefaultOptions(dbPath).WithLogger(nil))
	return db, err
}
