package main

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/grpc/services/collect"
	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
	"github.com/rs/zerolog"
	"log"
	"net"
)

var (
	collectorPort = ":50000"
)

//A step by step on how to use gossip
func main() {
	// step 1: establishing a tcp listener on an available port
	portPool := []string{"127.0.0.1:50000", "127.0.0.1:50001", "127.0.0.1:50002"}
	listener, err := pickPort(portPool)
	if err != nil {
		log.Fatal(err)
	}

	// step 2: registering the grpc services if any
	// Note: the gisp script should execute prior to the compile,
	// as this step to proceed requires a _registry.gen.go version of .proto files
	collector := gnode.NewNode(zerolog.Logger{}, collect.NewCollectServiceServerRegistry(NewCollector()))

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
