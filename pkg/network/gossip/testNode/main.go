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

	proto "github.com/golang/protobuf/proto"
	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-node/pkg/grpc/shared"
)

type naiveGossip struct{}

// Take in the protobuff messages and call the grpc of recipients
func (n naiveGossip) Gossip(ctx context.Context, gossipMsg *shared.GossipMessage) (reply proto.Message, err error) {
	reply = new(shared.MessageReply)
	// Loop through recipients
	for _, addr := range gossipMsg.Recipients {
		// Set up a connection to the other node
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			log.Fatalf("did not connect to %s: %v", addr, err)
			return reply, err
		}
		defer conn.Close()
		// Call the grpc manually so that the method can easily be switched out
		err = conn.Invoke(ctx, gossipMsg.Method, gossipMsg, reply)
		if err != nil {
			log.Fatalf("could not greet: %v", err)
			return reply, err
		}
	}
	return reply, err
}

type server struct{}

func (s *server) SendMessage(ctx context.Context, in *shared.GossipMessage) (*shared.MessageReply, error) {
	txt := in.GetMessageRequest().GetText()
	log.Printf("Received: %v", txt)
	fmt.Println("Enter your text:")
	return &shared.MessageReply{TextResponse: "Hello " + txt}, nil
}

func startServer(addrOfSelf string) {
	// Listen on the designated port and wait for rpc methods to be called
	lis, err := net.Listen("tcp", addrOfSelf)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	shared.RegisterMessagesServer(s, &server{})
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
			msgRequest := shared.MessageRequest{Text: text}

			gossipMsg := shared.GossipMessage{
				Payload:    &shared.GossipMessage_MessageRequest{&msgRequest},
				Method:     "/bamboo.shared.Messages/SendMessage",
				Recipients: peers,
			}

			gossipInstance := naiveGossip{}
			ctx := context.Background()
			// Nothing is currently done with result, but this is how to retrieve it
			result := new(shared.MessageReply)
			// Send the message
			msgReply, err := gossipInstance.Gossip(ctx, &gossipMsg)
			result = msgReply.(*shared.MessageReply)

			// Just to show the generality of receiving the return value,
			// have to use the var somewhere, kind of annoying that it prints twice
			log.Printf("Received: %v", result)
			log.Printf("Received: %v", msgReply)
			log.Printf("Received: %v", err)
		}
	}
}
