package testingdock_test

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"testing"
	"time"

	sdk "github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/dapperlabs/flow-go-sdk/keys"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/m4ksio/testingdock"
)

func TestContainer_Start(t *testing.T) {

	//network := flow.IdentityList{
	//	flow.Identity{
	//		NodeID:  unittest.IdentifierFixture(),
	//		Address: "collection",
	//		Role:    flow.RoleCollection,
	//		Stake:   1000,
	//	},
	//	flow.Identity{
	//		NodeID:  unittest.IdentifierFixture(),
	//		Address: "consensus",
	//		Role:    flow.RoleConsensus,
	//		Stake:   1000,
	//	},
	//	flow.Identity{
	//		NodeID:  unittest.IdentifierFixture(),
	//		Address: "execution",
	//		Role:    flow.RoleExecution,
	//		Stake:   1234,
	//	}
	//}

	// create suite

	testingdock.Verbose = true

	name := "mvp"
	suite, ok := testingdock.GetOrCreateSuite(t, name, testingdock.SuiteOpts{})
	if ok {
		t.Fatal("this suite should not exists yet")
	}

	// create network
	n := suite.Network(testingdock.NetworkOpts{
		Name: name,
	})

	collectionNodeApiPort := testingdock.RandomPort(t)

	collectionNode := suite.Container(testingdock.ContainerOpts{
		Name:      "collection",
		ForcePull: false,
		Config: &container.Config{
			Image: "gcr.io/dl-flow/collection:latest",
			ExposedPorts: nat.PortSet{
				"9000/tcp": struct{}{},
			},
			Cmd:   []string{
				"--entries=collection-c3bb99b34fdbd57eae8378a317af29b4b2da07610fd3a866c3a5ce9a19173295@collection:2137=10000,consensus-8330adb950a72ea2d1ca919d9bc4d9a784544d2614fcd13709606e0ece204d2b@consensus:2138=10000,execution-7276bf64cecb9e556dc961f2aa9b2cc8db2c5a1d8d2021fa75da3acd8b367e0b@execution:2139=10000,verification-831c36beb7fa82a05fb0114c03a4c167897941d0268212b6ba66bd2648a6ff53@verification:2140=1000",
				"--nodeid=c3bb99b34fdbd57eae8378a317af29b4b2da07610fd3a866c3a5ce9a19173295",
				"--connections=3",
				"--loglevel=debug",
				"--ingress-addr=collection:9000",
			},
		},
		HostConfig: &container.HostConfig{
			PortBindings: nat.PortMap{
				nat.Port("9000/tcp"): []nat.PortBinding{
					{
						HostIP: "0.0.0.0",
						HostPort: collectionNodeApiPort,
					},
				},
			},
		},
		HealthCheck: testingdock.HealthCheckCustom(func() error {
			return healthcheckGRPC(collectionNodeApiPort)
		}),
	})

	consensusNode := suite.Container(testingdock.ContainerOpts{
		Name:      "consensus",
		ForcePull: false,
		Config: &container.Config{
			Image: "gcr.io/dl-flow/consensus:latest",
			Cmd:   []string{
				"--entries=collection-c3bb99b34fdbd57eae8378a317af29b4b2da07610fd3a866c3a5ce9a19173295@collection:2137=10000,consensus-8330adb950a72ea2d1ca919d9bc4d9a784544d2614fcd13709606e0ece204d2b@consensus:2138=10000,execution-7276bf64cecb9e556dc961f2aa9b2cc8db2c5a1d8d2021fa75da3acd8b367e0b@execution:2139=10000,verification-831c36beb7fa82a05fb0114c03a4c167897941d0268212b6ba66bd2648a6ff53@verification:2140=1000",
				"--nodeid=8330adb950a72ea2d1ca919d9bc4d9a784544d2614fcd13709606e0ece204d2b",
				"--connections=3",
				"--loglevel=debug",
			},
		},
	})

	executionNode := suite.Container(testingdock.ContainerOpts{
		Name:      "execution",
		ForcePull: false,
		Config: &container.Config{
			Image: "gcr.io/dl-flow/execution:latest",
			Cmd:   []string{
				"--entries=collection-c3bb99b34fdbd57eae8378a317af29b4b2da07610fd3a866c3a5ce9a19173295@collection:2137=10000,consensus-8330adb950a72ea2d1ca919d9bc4d9a784544d2614fcd13709606e0ece204d2b@consensus:2138=10000,execution-7276bf64cecb9e556dc961f2aa9b2cc8db2c5a1d8d2021fa75da3acd8b367e0b@execution:2139=10000,verification-831c36beb7fa82a05fb0114c03a4c167897941d0268212b6ba66bd2648a6ff53@verification:2140=1000",
				"--nodeid=7276bf64cecb9e556dc961f2aa9b2cc8db2c5a1d8d2021fa75da3acd8b367e0b",
				"--connections=3",
				"--loglevel=debug",
			},
		},
	})

	verificationNode := suite.Container(testingdock.ContainerOpts{
		Name:      "verification",
		ForcePull: false,
		Config: &container.Config{
			Image: "gcr.io/dl-flow/verification:latest",
			Cmd:   []string{
				"--entries=collection-c3bb99b34fdbd57eae8378a317af29b4b2da07610fd3a866c3a5ce9a19173295@collection:2137=10000,consensus-8330adb950a72ea2d1ca919d9bc4d9a784544d2614fcd13709606e0ece204d2b@consensus:2138=10000,execution-7276bf64cecb9e556dc961f2aa9b2cc8db2c5a1d8d2021fa75da3acd8b367e0b@execution:2139=10000,verification-831c36beb7fa82a05fb0114c03a4c167897941d0268212b6ba66bd2648a6ff53@verification:2140=1000",
				"--nodeid=831c36beb7fa82a05fb0114c03a4c167897941d0268212b6ba66bd2648a6ff53",
				"--connections=3",
				"--loglevel=debug",
			},
		},
	})

	// add postgres to the test network
	n.After(collectionNode)
	n.After(consensusNode)
	n.After(executionNode)
	n.After(verificationNode)
	// start mnemosyned after postgres, this also adds it to the test network
	//postgres.After(mnemosyned)

	// start the network, this also starts the containers
	suite.Start(context.Background())
	defer suite.Close()

	//testQueries(t, db)

	sendTransaction(t, collectionNodeApiPort)

	time.Sleep(15*time.Second)

}

func healthcheckGRPC(apiPort string) error {
	fmt.Printf("healthchecking...\n")
	c, err := client.New("localhost:"+apiPort)
	if err != nil {
		return err
	}
	return c.Ping(context.Background())
}

func sendTransaction(t *testing.T, apiPort string) {
	fmt.Printf("Sending tx to %s\n", apiPort)
	c, err := client.New("localhost:"+apiPort)
	if err != nil {
		log.Fatal(err)
	}

	//_, err = c.GetLatestBlock(context.Background(), false)
	//if err != nil {
	//	log.Fatal(err)
	//}

	// Generate key
	seed := make([]byte, 40)
	_, _ = rand.Read(seed)
	key, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA2_256, seed)
	if err != nil {
		log.Fatal(err)
	}

	nonce := 2137

	tx := sdk.Transaction{
		Script:             []byte("fun main() {}"),
		ReferenceBlockHash: []byte{1, 2, 3, 4},
		Nonce:              uint64(nonce),
		ComputeLimit:       10,
		PayerAccount:       sdk.RootAddress,
	}

	sig, err := keys.SignTransaction(tx, key)
	if err != nil {
		log.Fatal(err)
	}

	tx.AddSignature(sdk.RootAddress, sig)

	err = c.SendTransaction(context.Background(), tx)
	if err != nil {
		log.Fatal(err)
	}
}
