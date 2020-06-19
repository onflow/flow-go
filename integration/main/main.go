package main

import (
	"encoding/hex"
	"fmt"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/integration/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func main() {

	chainID := flowsdk.Testnet
	addressGen := flowsdk.NewAddressGenerator(chainID)
	serviceAccountAddress := addressGen.NextAddress()
	fmt.Println("Root Service Address:", serviceAccountAddress)
	fungibleTokenAddress := addressGen.NextAddress()
	fmt.Println("Fungible Address:", fungibleTokenAddress)
	flowTokenAddress := addressGen.NextAddress()
	fmt.Println("Flow Address:", flowTokenAddress)

	serviceAccountPrivateKeyBytes, err := hex.DecodeString(unittest.ServiceAccountPrivateKeyHex)
	if err != nil {
		panic("error while hex decoding hardcoded root key")
	}

	// RLP decode the key
	ServiceAccountPrivateKey, err := flow.DecodeAccountPrivateKey(serviceAccountPrivateKeyBytes)
	if err != nil {
		panic("error while decoding hardcoded root key bytes")
	}

	// get the private key string
	priv := hex.EncodeToString(ServiceAccountPrivateKey.PrivateKey.Encode())

	flowClient, err := client.New("localhost:3569", grpc.WithInsecure())
	lg, err := utils.NewLoadGenerator(flowClient, priv, &serviceAccountAddress, &fungibleTokenAddress, &flowTokenAddress, 100, false)
	if err != nil {
		panic(err)
	}

	rounds := 5
	// extra 3 is for setup
	for i := 0; i < rounds+3; i++ {
		lg.Next()
	}

	fmt.Println(lg.Stats())
	lg.Close()
}
