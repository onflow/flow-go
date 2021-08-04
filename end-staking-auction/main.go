package main

import (
	"context"
	"encoding/hex"
	"fmt"

	"google.golang.org/grpc"

	sdk "github.com/onflow/flow-go-sdk"
	sdkclient "github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
)

var endStakingAuction = `
import FlowEpoch from 0x9eca2b38b18b5dfe
import FlowIDTableStaking from 0x9eca2b38b18b5dfe

transaction() {

    prepare(signer: AuthAccount) {
        let heartbeat = signer.borrow<&FlowEpoch.Heartbeat>(from: FlowEpoch.heartbeatStoragePath)
            ?? panic("Could not borrow heartbeat from storage path")

	let ids = FlowIDTableStaking.getProposedNodeIDs()

	let approvedIDs: {String: Bool} = {}
	for id in ids {
	    // Here is where we would make sure that each node's 
	    // keys and addresses are correct, they haven't committed any violations,
	    // and are operating properly
	    // for now we just set approved to true for all
	    approvedIDs[id] = true
	}
	heartbeat.endStakingAuction(approvedIDs: approvedIDs)

    }
}
`

var (
	// secp/sha2
	serviceAddr  = sdk.HexToAddress("8c5303eaa26202d6")
	serviceSkHex = "d1aa781aaf47d6b8cc8e2510e8e18ab12b7356bee944b84e6dfa578b1bcaac64"

	// ecdsa/sha3
	stakingAddr  = sdk.HexToAddress("9eca2b38b18b5dfe")
	stakingSkHex = "fe2b431f43b6ebcf79d2e4b2cd1d8da115356704e06868cca4c25bfd68e06c78"

	accessAddress = "access.canary.nodes.onflow.org:9000"
)

func MustHexToBytes(str string) []byte {
	bz, err := hex.DecodeString(str)
	if err != nil {
		panic(err)
	}
	return bz
}

func handle(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	serviceSK, err := crypto.DecodePrivateKey(crypto.ECDSA_secp256k1, MustHexToBytes(serviceSkHex))
	handle(err)
	serviceSigner := crypto.NewInMemorySigner(serviceSK, crypto.SHA2_256)

	stakingSK, err := crypto.DecodePrivateKey(crypto.ECDSA_P256, MustHexToBytes(stakingSkHex))
	handle(err)
	stakingSigner := crypto.NewInMemorySigner(stakingSK, crypto.SHA3_256)

	// create flow client
	flowClient, err := sdkclient.New(accessAddress, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	serviceAccount, err := flowClient.GetAccount(ctx, serviceAddr)
	handle(err)
	stakingAccount, err := flowClient.GetAccount(ctx, stakingAddr)
	handle(err)

	latest, err := flowClient.GetLatestBlock(ctx, true)
	handle(err)

	tx := sdk.NewTransaction().
		SetScript([]byte(endStakingAuction)).
		SetGasLimit(100000).
		SetReferenceBlockID(latest.ID).
		SetProposalKey(serviceAccount.Address, 0, serviceAccount.Keys[0].SequenceNumber).
		SetPayer(serviceAccount.Address).
		AddAuthorizer(stakingAccount.Address)

	err = tx.SignPayload(stakingAccount.Address, 0, stakingSigner)
	handle(err)

	err = tx.SignEnvelope(serviceAccount.Address, 0, serviceSigner)
	handle(err)

	err = flowClient.SendTransaction(ctx, *tx)
	handle(err)

	fmt.Println(tx.ID())
}
