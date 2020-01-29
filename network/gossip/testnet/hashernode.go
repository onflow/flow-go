package testnet

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/network/gossip"
	protocols "github.com/dapperlabs/flow-go/network/gossip/protocols/grpc"
	"github.com/dapperlabs/flow-go/network/gossip/registry"
)

// hashernode implements a simple node that has a "hash" function which takes the hash of any payload delivered to it and saves it.

type hasherNode struct {
	messages []string
	msgmu    sync.RWMutex
	hasher   crypto.Hasher
	wg       *sync.WaitGroup
}

func newHasherNode(wg *sync.WaitGroup) (*hasherNode, error) {
	h := crypto.NewSHA3_256()
	return &hasherNode{
		wg:       wg,
		messages: make([]string, 0),
		hasher:   h,
	}, nil
}

// receive method hashes the payload and stores it in the messages slice
func (hn *hasherNode) receive(ctx context.Context, payload []byte) ([]byte, error) {
	hash := hn.hasher.ComputeHash(payload)
	hn.msgmu.Lock()
	defer hn.msgmu.Unlock()
	defer hn.wg.Done()
	hn.messages = append(hn.messages, string(hash[:]))
	return []byte("received"), nil
}

// store method stores a message in the underlying message array
func (hn *hasherNode) store(payload []byte) {
	hash := hn.hasher.ComputeHash(payload)
	hn.msgmu.Lock()
	defer hn.msgmu.Unlock()
	hn.messages = append(hn.messages, string(hash[:]))
}

var (
	// Receive is a type to be registered for testing
	Receive registry.MessageType = 4
)

// startNode initializes a hasherNode. As part of the initialization, it creates a gNode, registers the receive method of the
// hasherNode on it, and returns the gNode instance. Any gossip with the message type of "Receive" for that instance of the gNode
// is routed to the corresponding instance of its hashNode
func (hn *hasherNode) startNode(logger zerolog.Logger, fanoutSize int, totalNumNodes int, index int, listeners []net.Listener, addresses []string) (*gossip.Node, error) {
	listener := listeners[index]
	myPort := addresses[index]
	othersPort := make([]string, 0)
	for _, port := range addresses {
		if port != myPort {
			othersPort = append(othersPort, port)
		}
	}

	node := gossip.NewNode(gossip.WithLogger(defaultLogger), gossip.WithAddress(myPort), gossip.WithPeers(othersPort), gossip.WithStaticFanoutSize(fanoutSize))

	err := node.RegisterFunc(Receive, hn.receive)
	if err != nil {
		return nil, err
	}

	sp, err := protocols.NewGServer(node)
	if err != nil {
		return nil, errors.New("could not initialize new GServer")
	}

	node.SetProtocol(sp)

	go func() {
		if err := node.Serve(listener); err != nil {
			logger.Err(err).Msg("node shutdown")
		}
	}()

	return node, nil
}
