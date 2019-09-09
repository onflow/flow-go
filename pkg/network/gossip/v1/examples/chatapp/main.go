package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"

	gnode "github.com/dapperlabs/bamboo-node/pkg/network/gossip/v1"
	"github.com/golang/protobuf/proto"
)

func main() {
	portPool := []string{"127.0.0.1:50000", "127.0.0.1:50001", "127.0.0.1:50002"}

	ln, err := pickPort(portPool)
	if err != nil {
		log.Fatal(err)
	}

	DisplayMessage := func(payloadBytes []byte) ([]byte, error) {
		msg := &Message{}

		if err := proto.Unmarshal(payloadBytes, msg); err != nil {
			return nil, fmt.Errorf("could not unmarshal payload: %v", err)
		}

		fmt.Printf("\n%v: %v", msg.Sender, string(msg.Content))
		fmt.Printf("Enter Message: ")
		return nil, nil
	}

	node := gnode.NewNodeWithRegistry(&chatRegistry{
		methods: map[string]gnode.HandleFunc{
			"DisplayMessage": DisplayMessage,
		},
	})

	servePort := ln.Addr().String()

	fmt.Println(servePort)

	peers := make([]string, 0)
	for _, port := range portPool {
		if port != servePort {
			peers = append(peers, port)
		}
	}

	go node.Serve(ln)

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("Enter Message: ")
		input, _ := reader.ReadString('\n')
		payloadBytes, err := createMsg(input, servePort)
		if err != nil {
			log.Fatalf("could not create message payload: %v", err)
		}

		_, err = node.AsyncGossip(context.Background(), payloadBytes, peers, "DisplayMessage")
		if err != nil {
			log.Println(err)
		}

	}
}

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

func pickPort(portPool []string) (net.Listener, error) {
	for _, port := range portPool {
		ln, err := net.Listen("tcp4", port)
		if err == nil {
			return ln, nil
		}
	}

	return nil, fmt.Errorf("could not find an empty port in the given pool")
}

type chatRegistry struct {
	methods map[string]gnode.HandleFunc
}

func (cr *chatRegistry) Methods() map[string]gnode.HandleFunc {
	return cr.methods
}
