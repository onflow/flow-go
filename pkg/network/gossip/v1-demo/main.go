package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/dapperlabs/bamboo-node/pkg/grpc/shared"
	async "github.com/dapperlabs/bamboo-node/pkg/network/gossip/v1"
)

// Demo of for the gossip async node implementation
// How to run: just start three instances of this program. The nodes will
// communicate with each other and place gossip messages.

func main() {
	portPool := []string{"127.0.0.1:50000", "127.0.0.1:50001", "127.0.0.1:50002"}

	ln, err := pickPort(portPool)
	if err != nil {
		log.Fatal(err)
	}

	servePort := ln.Addr().String()

	fmt.Println(servePort)
	if err != nil {
		log.Fatal(err)
	}

	node := async.NewNode()

	node.RegisterFunc("Time", func(msg *shared.GossipMessage) (*shared.MessageReply, error) {
		time.Sleep(2 * time.Second)
		fmt.Printf("The time is: %v\n", time.Now().Unix())
		return &shared.MessageReply{TextResponse: "text response"}, nil
	})

	go node.Serve(ln)

	peers := make([]string, 0)
	for _, port := range portPool {
		if port != servePort {
			peers = append(peers, port)
		}
	}

	t := time.Tick(5 * time.Second)

	r := rand.New(rand.NewSource(99))
	for {
		select {
		case <-t:
			msgRequest := shared.MessageRequest{Text: "test"}
			msg := &shared.GossipMessage{
				Uuid:       &shared.UUID{Value: fmt.Sprintf("%d", r.Int())},
				Payload:    &shared.GossipMessage_MessageRequest{MessageRequest: &msgRequest},
				Method:     "Time",
				Recipients: peers,
			}
			log.Println("Gossiping")
			rep, err := node.Gossip(context.Background(), msg)
			if err != nil {
				log.Println(err)
			}
			log.Println(rep[0])
		}
	}
}

func pickPort(portPool []string) (net.Listener, error) {
	for _, port := range portPool {
		ln, err := net.Listen("tcp4", port)
		if err == nil {
			return ln, nil
		}
	}

	return nil, fmt.Errorf("could not find an empty port in the given pool")
}
