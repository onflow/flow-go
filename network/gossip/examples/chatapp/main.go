package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/network/gossip"
	protocols "github.com/dapperlabs/flow-go/network/gossip/protocols/grpc"
	"github.com/dapperlabs/flow-go/network/gossip/testnet"
)

// Demo of a simple chat application based on the gossip node implementation
// How to run: just start three instances of this program.
// No IP/port configuration is required
// then you can send messages from and to multiple nodes (chat room)

// messageReceiver is our implementation of a ReceiverServer
type messageReceiver struct{}

// DisplayMessage displays received messages to the screen
func (mr *messageReceiver) DisplayMessage(ctx context.Context, msg *Message) (*Void, error) {
	fmt.Printf("\n%v: %v", msg.Sender, string(msg.Content))
	fmt.Printf("Enter Message: ")
	return &Void{}, nil
}

func main() {
	portPool := []string{"127.0.0.1:50000", "127.0.0.1:50001", "127.0.0.1:50002"}

	// step 1: establishing a tcp listener on an available port
	// pick a port from the port pool provided and listen on it.
	listener, err := testnet.PickPort(portPool)
	if err != nil {
		log.Fatal(err)
	}

	// step 2: registering the grpc services if any
	// Note: the gisp script should execute prior to the compile,
	// as this step to proceed requires a _registry.gen.go version of .proto files
	// Registering the gRPC services provided by the messageReceiver to the gossip registry
	myPort := listener.Addr().String()

	// finding the port number of other nodes
	othersPort := make([]string, 0)
	for _, port := range portPool {
		if port != myPort {
			othersPort = append(othersPort, port)
		}
	}

	node := gossip.NewNode(gossip.WithLogger(zerolog.New(ioutil.Discard)), gossip.WithRegistry(NewReceiverServerRegistry(&messageReceiver{})), gossip.WithAddress(myPort), gossip.WithPeers(othersPort), gossip.WithStaticFanoutSize(2))

	sp, err := protocols.NewGServer(node)
	if err != nil {
		log.Fatalf("could not start network server: %v", err)
	}

	node.SetProtocol(sp)

	fmt.Println("Chat app serves at port: ", myPort)

	// step 3: passing the listener to the instance of gnode
	go node.Serve(listener)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Enter Message: ")
		input, _ := reader.ReadString('\n')
		payloadBytes, err := createMsg(input, myPort)
		if err != nil {
			log.Fatalf("could not create message payload: %v", err)
		}

		// This example is using oneToMany, in order to test OneToAll, replace
		// othersPort with nil
		_, err = node.Gossip(context.Background(), payloadBytes, othersPort, DisplayMessage)
		if err != nil {
			log.Println(err)
		}

	}
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
