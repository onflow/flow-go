package main

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go-sdk/client"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/integration/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func main() {

	// TODO flowsdk.Testnet as chainID

	// addressGen := flowsdk.NewAddressGenerator(flowsdk.Testnet)
	// serviceAccountAddressHex = addressGen.NextAddress()
	// fmt.Println("Root Service Address:", serviceAccountAddressHex)
	// fungibleTokenAddress = addressGen.NextAddress()
	// fmt.Println("Fungible Address:", fungibleTokenAddress)
	// flowTokenAddress = addressGen.NextAddress()
	// fmt.Println("Flow Address:", flowTokenAddress)

	serviceAccountAddressHex := "8c5303eaa26202d6"
	fungibleTokenAddressHex := "9a0766d93b6608b7"
	flowTokenAddressHex := "7e60df042a9c0868"

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
	lg, err := utils.NewLoadGenerator(flowClient, priv, serviceAccountAddressHex, fungibleTokenAddressHex, flowTokenAddressHex, 100)
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
