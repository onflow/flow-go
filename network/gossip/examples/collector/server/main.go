package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/network/gossip/examples/collector"
	protocols "github.com/dapperlabs/flow-go/network/gossip/protocols/grpc"
	"github.com/dapperlabs/flow-go/protobuf/services/collection"
)

//A step by step on how to use gossip
func main() {
	// step 1: establishing a tcp listener on an available port
	portPool := []string{"127.0.0.1:50000", "127.0.0.1:50001", "127.0.0.1:50002"}
	listener, err := pickPort(portPool)
	if err != nil {
		log.Fatal(err)
	}

	myPort := listener.Addr().String()

	othersPort := make([]string, 0)
	for _, port := range portPool {
		if port != myPort {
			othersPort = append(othersPort, port)
		}
	}

	// step 2: registering the grpc services if any
	// Note: the gisp script should execute prior to the compile,
	// as this step to proceed requires a _registry.gen.go version of .proto files
	colReg := collection.NewCollectServiceServerRegistry(collector.NewCollector())

	collector := gossip.NewNode(gossip.WithLogger(zerolog.New(ioutil.Discard)), gossip.WithRegistry(colReg), gossip.WithAddress(myPort), gossip.WithPeers(othersPort), gossip.WithStaticFanoutSize(2))

	sp, err := protocols.NewGServer(collector)
	if err != nil {
		log.Fatalf("could not start network server: %v", err)
	}
	collector.SetProtocol(sp)

	// step 3: passing the listener to the instance of gnode
	collector.Serve(listener)
}

// pickPort chooses an available port from the portPool
func pickPort(portPool []string) (net.Listener, error) {
	for _, port := range portPool {
		ln, err := net.Listen("tcp4", port)
		if err == nil {
			return ln, nil
		}
	}

	return nil, fmt.Errorf("could not find an empty port in the given pool")
}
