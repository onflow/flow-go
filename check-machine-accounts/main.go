package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	sdk "github.com/onflow/flow-go-sdk"
	sdkclient "github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/io"
)

var (
	bootstrapDir  = "/Users/dapperlabs/canary6"
	nodeID        = flow.MustHexStringToIdentifier("14d6c408106c5a46c6039f9d14b4601acc63187781c21265141a119de288985b")
	accessAddress = "access.canary.nodes.onflow.org:9000"
)

func handle(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	log := zerolog.New(os.Stderr)

	// path to the root protocol snapshot json file
	snapshotPath := filepath.Join(bootstrapDir, bootstrap.PathRootProtocolStateSnapshot)

	// read root protocol-snapshot.json
	bz, err := io.ReadFile(snapshotPath)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not read root snapshot file")
	}
	log.Info().Str("snapshot_path", snapshotPath).Msg("read in root-protocol-snapshot.json")

	// unmarshal bytes to inmem protocol snapshot
	snapshot, err := convert.BytesToInmemSnapshot(bz)
	if err != nil {
		log.Fatal().Err(err).Msg("could not convert array of bytes to snapshot")
	}

	// retrieve participants
	identities, err := snapshot.Identities(filter.Any)
	if err != nil {
		log.Fatal().Err(err).Msg("could not retrieve identities")
	}

	wg := sync.WaitGroup{}
	for _, identity := range identities.Filter(filter.HasRole(flow.RoleConsensus, flow.RoleCollection)) {
		wg.Add(1)
		go func(nodeID flow.Identifier) {
			sendTx(nodeID)
			wg.Done()
		}(identity.NodeID)
	}
	wg.Wait()
}

func sendTx(nodeID flow.Identifier) {
	// attempt to read NodeMachineAccountInfo
	info, err := cmd.LoadNodeMachineAccountInfoFile(bootstrapDir, nodeID)
	if err != nil {
		panic(err)
	}

	// construct signer from private key
	sk, err := crypto.DecodePrivateKey(info.SigningAlgorithm, info.EncodedPrivateKey)
	if err != nil {
		panic(err)
	}
	txSigner := crypto.NewInMemorySigner(sk, info.HashAlgorithm)

	// create flow client
	flowClient, err := sdkclient.New(accessAddress, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	account, err := flowClient.GetAccount(ctx, sdk.HexToAddress(info.Address))
	handle(err)

	latest, err := flowClient.GetLatestBlock(ctx, true)
	handle(err)

	tx := sdk.NewTransaction().
		SetScript([]byte(`transaction() {prepare(acct: AuthAccount) {}}`)).
		SetGasLimit(9999).
		SetReferenceBlockID(latest.ID).
		SetProposalKey(account.Address, 0, account.Keys[0].SequenceNumber).
		SetPayer(account.Address).
		AddAuthorizer(account.Address)

	err = tx.SignEnvelope(account.Address, 0, txSigner)
	handle(err)

	err = flowClient.SendTransaction(ctx, *tx)
	handle(err)

	fmt.Println(tx.ID())

	for i := 0; i < 100; i++ {
		result, err := flowClient.GetTransactionResult(ctx, tx.ID())
		if err != nil {
			time.Sleep(time.Millisecond * 100)
			fmt.Println(tx.ID(), err)
			continue
		}

		if result.Status == sdk.TransactionStatusSealed {
			fmt.Println(tx.ID(), "sealed", result.Error)
			return
		}
	}
}
