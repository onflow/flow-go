package main

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
)

func main() {
	SecurePing()
}

func SecurePing() {
	ctx := context.Background()

	secureGRPCServerAddress := "0.0.0.0:3570"
	accessNodePublicKey := "dd5da993faebcb16e8b96a126a78f8845140b12c5b083cfa6b61701bc78b0039c78434acb010ca73c449f93acec6b81c3a71b8ca1f3c5b330258139f513e6f2a"

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
