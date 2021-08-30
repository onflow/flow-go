package main

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
)

func main()  {
	SecurePing()
}

func SecurePing() {
	ctx := context.Background()

	secureGRPCServerAddress := "0.0.0.0:3570"
	accessNodePublicKey := "7d18b2e351147d7c656b16279a5936e7ffeaf5f17f97c560d0a9c24d6294533fb1fda149ce8981583cd1bc25b489ea973cdf16eb224b59a476a9009f039043a4"

	flowClient, err := common.SecureFlowClient(secureGRPCServerAddress, accessNodePublicKey)
	if err != nil {
		panic(err)
	}

	err = flowClient.Ping(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("ping successful")
}
