package debug

import (
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_RemoteDebugMigrationMainnet(t *testing.T) {
	// use the following command to forward port 9000 from the EN to localhost:9001
	// `gcloud compute ssh '--ssh-flag=-A' --no-user-output-enabled --tunnel-through-iap migrationmainnet1-execution-001 --project flow-multi-region -- -NL 9001:localhost:9000`

	grpcAddress := "0.0.0.0:9001"
	chain := flow.Mainnet.Chain()
	log := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.DebugLevel)

	debugger := NewRemoteDebugger(
		grpcAddress,
		chain,
		log,
	)

	tx := getTx()

	txErr, processErr := debugger.RunTransaction(tx)
	require.NoError(t, txErr)
	require.NoError(t, processErr)

}

func getTx() *flow.TransactionBody {
	script := `
		import FlowStakingCollection from 0x8d0e87b65159ae63
		
		/// Changes the networking address for the specified node
		
		transaction(nodeID: String, newAddress: String) {
		
			let stakingCollectionRef: auth(FlowStakingCollection.CollectionOwner) &FlowStakingCollection.StakingCollection
		
			prepare(account: auth(BorrowValue) &Account) {
				self.stakingCollectionRef = account.storage.borrow<auth(FlowStakingCollection.CollectionOwner) &FlowStakingCollection.StakingCollection>(from: FlowStakingCollection.StakingCollectionStoragePath)
					?? panic("Could not borrow a reference to a StakingCollection in the primary user's account")
			}
		
			execute {
				self.stakingCollectionRef.updateNetworkingAddress(nodeID: nodeID, newAddress: newAddress)
			}
		}`

	payerAddress := flow.HexToAddress("0xe467b9dd11fa00df")
	authorizerAddress := flow.HexToAddress("0x88cc9deda57e8478")

	argument1 := cadence.String("26c1cd3254ec259b4faea0f53e3a446539256d81f0c06fff430690433d69731f")
	argument1cdc := jsoncdc.MustEncode(argument1)
	argument2 := cadence.String("verification-001.migrationmainnet1.nodes.onflow.org:3569")
	argument2cdc := jsoncdc.MustEncode(argument2)

	tx := flow.NewTransactionBody().
		SetScript([]byte(script)).
		SetProposalKey(authorizerAddress, 1, 1).
		AddAuthorizer(authorizerAddress).
		SetComputeLimit(9999).
		SetPayer(payerAddress).
		AddArgument(argument1cdc).
		AddArgument(argument2cdc)

	return tx
}

// you can use the following flow.json and port forward command to interact with the network:
// `gcloud compute ssh '--ssh-flag=-A' --no-user-output-enabled --tunnel-through-iap migrationmainnet1-access-001 --project flow-multi-region -- -NL 9000:localhost:9000`
// ```
// {
//  "emulators": {},
//  "networks": {
//    "migmain": "localhost:9000"
//  },
//  "accounts": {
//    "service_account": {
//      "address": "e467b9dd11fa00df",
//      "key": {
//        "type": "hex",
//        "index": 19,
//        "signatureAlgorithm": "ECDSA_P256",
//        "hashAlgorithm": "SHA3_256",
//        "privateKey": "c411a2c97f2355774d8e9925bea0309a01d9bf8fa07a209a60b279e239db806d"
//      }
//    },
//    "test_root": {
//      "address": "0xb77d0ead3c7afca1",
//      "key": "c411a2c97f2355774d8e9925bea0309a01d9bf8fa07a209a60b279e239db806d"
//    }
//  },
//  "deployments": {},
//  "contracts": {
//    "FlowFees": {
//      "aliases": {
//        "migmain": "f919ee77447b7497"
//      }
//    },
//    "FlowEpoch": {
//      "aliases": {
//        "migmain": "8624b52f9ddcd04a"
//      }
//    },
//    "FlowIDTableStaking": {
//      "aliases": {
//        "migmain": "8624b52f9ddcd04a"
//      }
//    }
//  }
// }
// ```
//
//
