package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/dapperlabs/bamboo-node/pkg/grpc/shared"
	async "github.com/dapperlabs/bamboo-node/pkg/network/gossip/v1"
)

// Demo of for the gossip async node implementation
// How to run: just start three instances of this program. The nodes will
// communicate with each other and place gossip messages.

func main() {
	portPool := []string{"50000", "50001", "50002"}

	servePort, err := pickPort(portPool)
	if err != nil {
		log.Fatal(err)
	}

	node := async.NewNode(fmt.Sprintf("%s:%s", "127.0.0.1", servePort))

	node.RegisterFunc("Time", func(msg *shared.GossipMessage) error {
		time.Sleep(2 * time.Second)
		fmt.Printf("The time is: %v\n", time.Now().Unix())
		return nil
	})

	go node.Serve()

	peers := make([]string, 0)
	for _, port := range portPool {
		if port != servePort {
			peers = append(peers, fmt.Sprintf("%s:%s", "127.0.0.1", port))
		}
	}

	t := time.Tick(5 * time.Second)

	for {
		select {
		case <-t:
			msgRequest := shared.MessageRequest{Text: "test"}
			msg := &shared.GossipMessage{
				Payload:    &shared.GossipMessage_MessageRequest{MessageRequest: &msgRequest},
				Method:     "Time",
				Recipients: peers,
			}
			log.Println("Gossiping")
			_, err := node.Gossip(context.Background(), msg)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func pickPort(portPool []string) (string, error) {
	for _, port := range portPool {
		ln, err := net.Listen("tcp", ":"+port)
		if err == nil {
			defer ln.Close()
			return port, nil
		}
	}

	return "", fmt.Errorf("could not find an empty port in the given pool")
}
