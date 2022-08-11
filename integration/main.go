package main

import (
	"context"
	"fmt"
	"time"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("0.0.0.0:3573", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	client := accessproto.NewAccessAPIClient(conn)

	ticker := time.NewTicker(time.Millisecond * 200)
	for range ticker.C {
		response, _ := client.GetLatestBlock(ctx, &accessproto.GetLatestBlockRequest{})
		block := response.Block
		fmt.Println("------------------------\n", block)
		fmt.Printf("%#v\n", block)

		response, _ = client.GetBlockByID(ctx, &accessproto.GetBlockByIDRequest{Id: block.Id})
		block = response.Block

		fmt.Println("------------------------\n", block)
		fmt.Printf("%#v\n", block)
	}
}
