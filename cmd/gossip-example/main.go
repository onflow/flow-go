package main

import (
	"context"
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/grpc/services/collect"
	"google.golang.org/grpc"
)

func main() {
	addr := "localhost:5000"

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		fmt.Println("Failed to dial", err)
	}
	defer conn.Close()

	client := collect.NewCollectServiceClient(conn)

	ctx := context.Background()
	res, err := client.Ping(ctx, &collect.PingRequest{})
	if err != nil {
		fmt.Println("Failed to call ping", err)
	}

	fmt.Printf("response: %s\n", res.GetAddress())

	// var g gossip.GossipService

	// res, err := g.Gossip(
	// 	ctx,
	// 	gossip.Message{
	// 		Payload:    &collect.PingRequest{},
	// 		Method:     "/bamboo.services.collect.CollectService/Ping",
	// 		Recipients: []string{"localhost:5000"},
	// 	},
	// )
}
