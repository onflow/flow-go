package testnet

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type NodeConfigs []NodeConfig

type NodeConfigFilter func(n NodeConfig) bool

// NodeConfig defines the input config for a particular node, specified prior
// to network creation.
type NodeConfig struct {
	Role                flow.Role
	Corrupted           bool
	Weight              uint64
	Identifier          flow.Identifier
	LogLevel            zerolog.Level
	Ghost               bool
	AdditionalFlags     []string
	Debug               bool
	EnableMetricsServer bool
}

func (n NodeConfigs) Filter(filters ...NodeConfigFilter) NodeConfigs {
	nodeConfigs := make(NodeConfigs, 0)
	for _, config := range n {
		passedAllFilters := true
		for _, f := range filters {
			if !f(config) {
				passedAllFilters = false
				break
			}
		}

		if passedAllFilters {
			nodeConfigs = append(nodeConfigs, config)
		}
	}

	return nodeConfigs
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
func NewNodeConfigSet(n uint, role flow.Role, opts ...func(*NodeConfig)) NodeConfigs {

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

// AsCorrupted sets the configuration of a node as corrupted, hence the node is pulling
// the corrupted image of its role at the build time.
// A corrupted image is running with Corruptible Conduit Factory hence enabling BFT testing
// on the node.
func AsCorrupted() func(config *NodeConfig) {
	return func(config *NodeConfig) {
		if config.Ghost {
			panic("a node cannot be both corrupted and ghost at the same time")
		}
		config.Corrupted = true
	}
}

func AsGhost() func(config *NodeConfig) {
	return func(config *NodeConfig) {
		if config.Corrupted {
			panic("a node cannot be both corrupted and ghost at the same time")
		}
		config.Ghost = true
	}
}

// WithAdditionalFlag adds additional flags to the command
func WithAdditionalFlag(flag string) func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.AdditionalFlags = append(config.AdditionalFlags, flag)
	}
}

// WithAdditionalFlagf adds additional flags to the command using a formatted string
func WithAdditionalFlagf(format string, a ...any) func(config *NodeConfig) {
	return WithAdditionalFlag(fmt.Sprintf(format, a...))
}

// WithMetricsServer exposes the metrics server
func WithMetricsServer() func(config *NodeConfig) {
	return func(config *NodeConfig) {
		config.EnableMetricsServer = true
	}
}
