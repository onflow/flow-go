package main

import (
	"fmt"
	"log"
	"net"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/network/gossip/examples/collector"
	"github.com/dapperlabs/flow-go/network/gossip/protocols"
	"github.com/dapperlabs/flow-go/proto/services/collection"
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
	config := gossip.NewNodeConfig(colReg, myPort, othersPort, 0, 10)
	collector := gossip.NewNode(config)

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
