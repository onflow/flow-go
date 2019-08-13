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

type GossipError []error

func (g *GossipError) Collect(e error) {
	if e != nil {
		*g = append(*g, e)
	}
}

func (g *GossipError) Error() string {
	err := "gossip errors:\n"
	for i, e := range *g {
		err += fmt.Sprintf("\tgerror %d: %s\n", i, e.Error())
	}

	return err
}

type naiveGossip struct{}

// Take in the protobuff messages and call the grpc of recipients
func (n naiveGossip) Gossip(ctx context.Context, gossipMsg *shared.GossipMessage) ([]proto.Message, error) {
	var (
		gerrc    = make(chan error)
		repliesc = make(chan proto.Message)

		gerr    = &GossipError{}
		replies = make([]proto.Message, len(gossipMsg.Recipients))
	)

	// Loop through recipients
	for _, addr := range gossipMsg.Recipients {
		go func(addr string) {
			reply, err := n.gossipHandler(ctx, addr, gossipMsg)
			gerrc <- err
			repliesc <- reply
		}(addr)
	}

	for i, _ := range gossipMsg.Recipients {
		gerr.Collect(<-gerrc)
		replies[i] = <-repliesc
	}

	if len(*gerr) != 0 {
		return replies, gerr
	}

	return replies, nil
}

// gossipHandler is used to send one gossip message to one recipient
func (n naiveGossip) gossipHandler(ctx context.Context, addr string, gossipMsg *shared.GossipMessage) (proto.Message, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("did not connect to %s: %v", addr, err)
	}

	defer conn.Close()

	// Call the grpc manually so that the method can easily be switched out
	reply := new(shared.MessageReply)
	err = conn.Invoke(ctx, gossipMsg.Method, gossipMsg, reply)
	if err != nil {
		return nil, fmt.Errorf("could not greet: %v", err)
	}

	return reply, nil
}

type server struct{}

func (s *server) SendMessage(ctx context.Context, in *shared.GossipMessage) (*shared.MessageReply, error) {
	txt := in.GetMessageRequest().GetText()
	log.Printf("\nReceived: %v", txt)
	fmt.Print("Enter your text: ")
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

	fmt.Print("Enter your text: ")
	for {
		scanner.Scan()
		text := scanner.Text()

		// exit condition
		if text == "q" {
			os.Exit(0)
		}
		// Generate the protobuff messages needed to send through gossip

		go func(text string) {
			msgRequest := shared.MessageRequest{Text: text}

			gossipMsg := shared.GossipMessage{
				Payload:    &shared.GossipMessage_MessageRequest{&msgRequest},
				Method:     "/bamboo.shared.Messages/SendMessage",
				Recipients: peers,
			}
			gossipInstance := naiveGossip{}
			ctx := context.Background()

			// Send the message
			msgReply, err := gossipInstance.Gossip(ctx, &gossipMsg)

			// Just to show the generality of receiving the return value,
			// have to use the var somewhere, kind of annoying that it prints twice
			log.Printf("Responses: %v\n", msgReply)
			log.Printf("Errors: %v\n", err)
			fmt.Print("Enter your text: ")
		}(text)

	}
}
