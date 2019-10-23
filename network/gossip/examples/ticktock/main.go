package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/network/gossip/protocols"
)

// Demo of for the gossip node implementation
// How to run: just start three instances of this program. The nodes will
// communicate with each other and place gossip messages.

func main() {
	portPool := []string{"127.0.0.1:50000", "127.0.0.1:50001", "127.0.0.1:50002"}

	// step 1: establishing a tcp listener on an available port
	listener, err := pickPort(portPool)
	if err != nil {
		log.Fatal(err)
	}
	servePort := listener.Addr().String()

	fmt.Println(servePort)
	if err != nil {
		log.Fatal(err)
	}

	peers := make([]string, 0)
	for _, port := range portPool {
		if port != servePort {
			peers = append(peers, port)
		}
	}

	// step 2: registering the grpc services if any
	// Note: this example is not built upon any grpc service, hence we pass nil

	config := gossip.NewNodeConfig(nil, servePort, peers, 0, 10)
	node := gossip.NewNode(config)
	protocol := protocols.NewGServer(node)
	node.SetProtocol(protocol)

	// step 3: passing the listener to the instance of node
	go node.Serve(listener)

	// Defining and adding a time function to the registry of node
	Time := func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
		newMsg := &Message{}
		if err := proto.Unmarshal(payloadBytes, newMsg); err != nil {
			return nil, fmt.Errorf("could not unmarshal payload: %v", err)
		}

		log.Printf("Payload: %v", string(newMsg.Text))
		time.Sleep(2 * time.Second)
		fmt.Printf("The time is: %v\n", time.Now().Unix())
		return []byte("Pong"), nil
	}

	// add the Time function to the node's registry
	node.RegisterFunc("Time", Time)

	t := time.Tick(5 * time.Second)

	for {
		select {
		case <-t:
			go func() {
				log.Println("Gossiping")
				payload := &Message{Text: []byte(time.Now().String())}
				bytes, err := proto.Marshal(payload)
				if err != nil {
					log.Fatalf("could not marshal message: %v", err)
				}
				// You can try to change the line bellow to AsyncGossip(...), when you do
				// so you will notice that the responses returned to you will be empty
				// (that is because AsyncGossip does not wait for the sent messages to be
				// processed)
				rep, err := node.AsyncGossip(context.Background(), bytes, peers, "Time")
				if err != nil {
					log.Println(err)
				}
				for _, resp := range rep {
					if resp == nil {
						continue
					}
					log.Printf("Response: %v\n", string(resp.ResponseByte))
				}
			}()
		}
	}
}

// pickPort chooses a port from the port pool which is available
func pickPort(portPool []string) (net.Listener, error) {
	for _, port := range portPool {
		ln, err := net.Listen("tcp4", port)
		if err == nil {
			return ln, nil
		}
	}

	return nil, fmt.Errorf("could not find an empty port in the given pool")
}
