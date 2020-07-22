package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"strings"
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/integration/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func main() {

	sleep := flag.Int64("sleep", 0, "seconds to sleep before benchmarking starts")
	time.Sleep(time.Duration(*sleep) * time.Second)

	verbose := flag.Bool("verbose", false, "print verbose information")
	chainIDStr := flag.String("chain", string(flowsdk.Testnet), "chain ID")
	chainID := flowsdk.ChainID([]byte(*chainIDStr))
	access := flag.String("access", "localhost:3569", "access node address")
	serviceAccountPrivateKeyHex := flag.String("servPrivHex", unittest.ServiceAccountPrivateKeyHex, "service account private key hex")
	accessNodeAddrs := strings.Split(*access, ",")

	addressGen := flowsdk.NewAddressGenerator(chainID)
	serviceAccountAddress := addressGen.NextAddress()
	fmt.Println("Root Service Address:", serviceAccountAddress)
	fungibleTokenAddress := addressGen.NextAddress()
	fmt.Println("Fungible Address:", fungibleTokenAddress)
	flowTokenAddress := addressGen.NextAddress()
	fmt.Println("Flow Address:", flowTokenAddress)

	serviceAccountPrivateKeyBytes, err := hex.DecodeString(*serviceAccountPrivateKeyHex)
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

	flowClient, err := client.New(accessNodeAddrs[0], grpc.WithInsecure())
	lg, err := utils.NewLoadGenerator(flowClient, priv, &serviceAccountAddress, &fungibleTokenAddress,
		&flowTokenAddress, 2000, *verbose)
	if err != nil {
		panic(err)
	}

	rounds := 5
	// extra 3 is for setup
	for i := 0; i < rounds+3; i++ {
		lg.Next()
	}

	// this prints all transactions
	// fmt.Println(lg.Stats())
	fmt.Println(lg.Stats().Digest())
	lg.Close()
}
