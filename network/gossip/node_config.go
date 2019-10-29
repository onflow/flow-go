package gossip

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

// DefaultQueueSize is the default size of node queue
const DefaultQueueSize = 10

// NodeConfig type is wrapper for the parameters used to construct a gossip node
// logger is an instance of the zerolog for printing the log messages by the node
// address is the (IP) address of the node itself
// peers is the list of all the nodes in the system as IP:port, e.g., localhost:8080
// static fanout is the number of gossip partners of the node, a typical number may be 10
// queueSize is the buffer size of the node on processing the gossip messages, a typical number may be 10
type NodeConfig struct {
	logger          zerolog.Logger
	regMnger        *registryManager
	address         *messages.Socket
	peers           []string
	staticFanoutNum int
	queueSize       int
}

// NewNodeConfig returns a new instance NodeConfig
// addr is the (IP) address of the node itself
// peers is the list of all the nodes in the system as IP:port, e.g., localhost:8080
// static fanout is the number of gossip partners of the node, a typical number may be 10
// qSize is the buffer size of the node on processing the gossip messages, a typical number may be 10
func NewNodeConfig(reg Registry, addr string, peers []string, staticFN int, qSize int) *NodeConfig {
	address, err := newSocket(addr)
	if err != nil {
		fmt.Printf("Error creating socket: %v", err)
		return nil
	}
	if qSize <= 0 {
		qSize = DefaultQueueSize
	}
	return &NodeConfig{
		logger:          log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger(),
		regMnger:        newRegistryManager(reg),
		address:         address,
		peers:           peers,
		staticFanoutNum: staticFN,
		queueSize:       qSize,
	}
}
