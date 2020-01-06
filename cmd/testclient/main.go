package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/dapperlabs/flow-go/sdk/client"
	"github.com/dapperlabs/flow-go/sdk/keys"
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/model/flow"
)

var (
	targetAddr      string
	numTransactions int
)

/*
0000000000000000000000000000000000000000000000000000000000000001
0000000000000000000000000000000000000000000000000000000000000002
0000000000000000000000000000000000000000000000000000000000000003

collection-0000000000000000000000000000000000000000000000000000000000000001@localhost:8001=1000,collection-0000000000000000000000000000000000000000000000000000000000000002@localhost:8002=1000,collection-0000000000000000000000000000000000000000000000000000000000000003@localhost:8003=1000

./collection \
	--entries collection-0000000000000000000000000000000000000000000000000000000000000001@localhost:8001=1000,collection-0000000000000000000000000000000000000000000000000000000000000002@localhost:8002=1000,collection-0000000000000000000000000000000000000000000000000000000000000003@localhost:8003=1000 \
	--loglevel debug \
	--connections 2 \
	--nodeid 0000000000000000000000000000000000000000000000000000000000000001 \
	--datadir ./data1 \
	--ingress-addr localhost:9001

./collection \
	--entries collection-0000000000000000000000000000000000000000000000000000000000000001@localhost:8001=1000,collection-0000000000000000000000000000000000000000000000000000000000000002@localhost:8002=1000,collection-0000000000000000000000000000000000000000000000000000000000000003@localhost:8003=1000 \
	--loglevel debug \
	--connections 2 \
	--nodeid 0000000000000000000000000000000000000000000000000000000000000002 \
	--datadir ./data2 \
	--ingress-addr localhost:9002

./collection \
	--entries collection-0000000000000000000000000000000000000000000000000000000000000001@localhost:8001=1000,collection-0000000000000000000000000000000000000000000000000000000000000002@localhost:8002=1000,collection-0000000000000000000000000000000000000000000000000000000000000003@localhost:8003=1000 \
	--loglevel debug \
	--connections 2 \
	--nodeid 0000000000000000000000000000000000000000000000000000000000000003 \
	--datadir ./data3 \
	--ingress-addr localhost:9003
*/

func main() {
	pflag.StringVarP(&targetAddr, "target-addr", "t", "localhost:9001", "address of the collection node to connect to")
	pflag.IntVarP(&numTransactions, "num-transactions", "n", 5, "number of transactions to send")
	pflag.Parse()

	fmt.Println("server addr: ", targetAddr)
	fmt.Println("num transactions: ", numTransactions)

	c, err := client.New(targetAddr)
	if err != nil {
		panic(err)
	}

	// Generate key
	seed := make([]byte, 40)
	rand.Read(seed)
	key, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA2_256, seed)
	if err != nil {
		panic(err)
	}

	for i := 0; i < numTransactions; i++ {
		time.Sleep(time.Second)

		tx := flow.Transaction{
			Script:             []byte("fun main() {}"),
			ReferenceBlockHash: []byte{1, 2, 3, 4},
			Nonce:              uint64(i + 1),
			ComputeLimit:       10,
			PayerAccount:       flow.RootAddress,
		}

		sig, err := keys.SignTransaction(tx, key)
		if err != nil {
			fmt.Println("failed to sign transaction: ", err)
			continue
		}
		tx.AddSignature(flow.RootAddress, sig)

		err = c.SendTransaction(context.Background(), tx)
		if err != nil {
			fmt.Println("failed to send transaction: ", err)
			continue
		}

	}
}
