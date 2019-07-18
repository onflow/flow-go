package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/dapperlabs/603-Making-nodes-talk-via-gossip/bamboo-node/pkg/network/gossip"
	proto "github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

type naiveGossip struct{}

// Take in the protobuff messages and call the grpc of recipients
func (n naiveGossip) Gossip(ctx context.Context, request *gossip.Message, reply proto.Message) (err error) {
	// Loop through recipients
	for _, addr := range request.Recipients {
		// Set up a connection to the other node
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			log.Fatalf("did not connect to %s: %v", addr, err)
			return err
		}
		defer conn.Close()
		// Call the grpc manually so that the method can easily be switched out
		err = conn.Invoke(ctx, request.Method, request.Payload, reply)
		if err != nil {
			log.Fatalf("could not greet: %v", err)
			return err
		}
	}
	return err
}

type server struct{}

func (s *server) SendMessage(ctx context.Context, in *MessageRequest) (*MessageReply, error) {
	log.Printf("Received: %v", in.Text)
	fmt.Println("Enter your text:")
	return &MessageReply{TextResponse: "Hello " + in.Text}, nil
}

func startServer(addrOfSelf string) {
	// Listen on the designated port and wait for rpc methods to be called
	lis, err := net.Listen("tcp", addrOfSelf)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	RegisterMessagesServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	// Get info from the cmd line
	var port int
	flag.IntVar(&port, "port", 10001, "port to run this node on")
	flag.Parse()

	fmt.Printf("Starting node, port = %d\n", port)
	portStr := strconv.Itoa(port)
	addrOfSelf := "localhost:" + portStr

	// Start a concurrent process listening on the designated port
	go startServer(addrOfSelf)

	// Filter our port from the list so we don't send messges to ourself
	nodeAddrs := []string{"localhost:50000", "localhost:50001", "localhost:50002"}
	peers := []string{}
	for _, node := range nodeAddrs {
		if node != addrOfSelf {
			peers = append(peers, node)
		}
	}

	// Take in input from the cmd line to use as messages
	scanner := bufio.NewScanner(os.Stdin)
	var text string
	// Quit/break the loop if text == "q"
	for text != "q" {
		fmt.Println("Enter your text:")
		scanner.Scan()
		text = scanner.Text()
		if text != "q" {
			// Generate the protobuff messages needed to send through gossip
			msgeRequest := MessageRequest{Text: text}
			gossipMessage := gossip.Message{
				Payload:    &msgeRequest,
				Method:     "/main.Messages/SendMessage",
				Recipients: peers,
			}
			gossipInstance := naiveGossip{}
			ctx := context.Background()
			// Nothing is currently done with result
			result := new(MessageReply)
			// Send the message
			gossipInstance.Gossip(ctx, &gossipMessage, result)
		}
	}
}
