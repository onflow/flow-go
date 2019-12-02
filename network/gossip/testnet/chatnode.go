package testnet

import (
	"context"
	"errors"
	"fmt"
	"github.com/dapperlabs/flow-go/network/gossip"
	protocols "github.com/dapperlabs/flow-go/network/gossip/protocols/grpc"
	"github.com/rs/zerolog"
	"net"
	"sync"

	"github.com/golang/protobuf/proto"
)

type chatNode struct {
	messages []string
	mu       sync.RWMutex
	wg       *sync.WaitGroup
}

// chatnode implements a node that sends out and receives messages as defined in message.proto

func (cn *chatNode) DisplayMessage(ctx context.Context, msg *Message) (*Void, error) {
	// disabled to not clutter up space in console
	// fmt.Printf("\n%v: %v", msg.Sender, string(msg.Content))
	cn.mu.Lock()
	cn.messages = append(cn.messages, string(msg.Content))
	cn.wg.Done()
	cn.mu.Unlock()
	return &Void{}, nil
}

// Initializes a chatNode
func newChatNode(wg *sync.WaitGroup) (*chatNode, error) {
	return &chatNode{
		wg:       wg,
		messages: make([]string, 0),
	}, nil
}

// createMsg constructs a Message and marshals it
func createMsg(content, sender string) ([]byte, error) {
	msg := &Message{
		Sender:  sender,
		Content: []byte(content),
	}

	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("could not marshal proto message: %v", err)
	}
	return msgBytes, nil
}

// startNode initializes a gnode with the given chatNode's functions as its registry
// and makes it serve a listener from the pool. If different functions using the function at the same time
// provide different startPorts to avoid conflicts
func (cn *chatNode) startNode(logger zerolog.Logger, fanoutSize int, totalNumNodes int, index int, listeners []net.Listener, addresses []string) (*gossip.Node, error) {
	listener := listeners[index]
	myPort := addresses[index]
	othersPort := make([]string, 0)
	for _, port := range addresses {
		if port != myPort {
			othersPort = append(othersPort, port)
		}
	}

	node := gossip.NewNode(gossip.WithLogger(defaultLogger), gossip.WithAddress(myPort), gossip.WithPeers(othersPort), gossip.WithStaticFanoutSize(fanoutSize), gossip.WithRegistry(NewReceiverServerRegistry(cn)))

	sp, err := protocols.NewGServer(node)
	if err != nil {
		return nil, errors.New("could not initialize new GServer")
	}

	node.SetProtocol(sp)

	go node.Serve(listener)

	return node, nil
}
