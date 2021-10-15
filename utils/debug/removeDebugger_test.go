package debug_test

import (
	"fmt"
	"os"
	"testing"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
)

func TestMintingTestNetIssue(t *testing.T) {

	const script = `
		import NonFungibleToken from 0x631e88ae7f1d7c20
		import CricketMoments from 0xb45e7992680a0f7f


		// This transction uses the NFTMinter resource to mint a new NFT.
		//
		// It must be run with the account that has the minter resource
		// stored at path /storage/NFTMinter.

		transaction(recipient: Address, momentId:UInt64, serialQuantity: UInt64, metadata:{String:String}) {
			
			// local variable for storing the minter reference
			let minter: &CricketMoments.NFTMinter

			prepare(signer: AuthAccount) {

				// borrow a reference to the NFTMinter resource in storage
				self.minter = signer.borrow<&CricketMoments.NFTMinter>(from: CricketMoments.MinterStoragePath)
					?? panic("Could not borrow a reference to the NFT minter")
			}

			execute {
				// get the public account object for the recipient
				let recipientAccount = getAccount(recipient)

				// borrow the recipient's public NFT collection reference
				let receiver = recipientAccount
					.getCapability(CricketMoments.CollectionPublicPath)
					.borrow<&{NonFungibleToken.CollectionPublic}>()
					?? panic("Could not get receiver reference to the NFT Collection")

				// mint the NFT and deposit it to the recipient's collection
				self.minter.mintOldNFTs(recipient: receiver, momentId: momentId, serialQuantity: serialQuantity, metadata: metadata)
			}
		}
	`

	add := flow.HexToAddress("b45e7992680a0f7f")
	pairs := make([]cadence.KeyValuePair, 0)
	pairs = append(pairs, cadence.KeyValuePair{Key: cadence.String("description"), Value: cadence.String("test moment")})
	pairs = append(pairs, cadence.KeyValuePair{Key: cadence.String("ipfs"), Value: cadence.String("QmYQKyWUxCDJwrRStXkrxhswFanjRrVyyJFFGtFHZMe36J6")})
	dataArg := cadence.NewDictionary(pairs)
	txScript := []byte(script)
	txBody := flow.NewTransactionBody().
		SetGasLimit(9999).
		SetScript([]byte(txScript)).
		SetPayer(flow.HexToAddress("fd0383dc0eafaacb")).
		SetProposalKey(add, 88, 3).
		AddAuthorizer(add).
		AddArgument(jsoncdc.MustEncode(cadence.BytesToAddress(add[:]))).
		AddArgument(jsoncdc.MustEncode(cadence.UInt64(17))).
		AddArgument(jsoncdc.MustEncode(cadence.UInt64(250))).
		AddArgument(jsoncdc.MustEncode(dataArg))

	grpcAddress := "35.208.109.249:9000"
	chain := flow.Testnet.Chain()
	debugger := debug.NewRemoteDebugger(grpcAddress, chain, zerolog.New(os.Stdout).With().Logger())

	// TODO update the blockID

	blockId, err := flow.HexStringToIdentifier("8c2d264d12a64c17664126e55d42c605d1a0b787211b5d872f62ad15fe7beadd")
	require.NoError(t, err)
	txErr, err := debugger.RunTransactionAtBlockID(txBody, blockId, "MintingTestNetCache")
	require.NoError(t, txErr)
	require.NoError(t, err)
}

func TestDebugger_RunTransaction(t *testing.T) {

	// this code is mostly a sample code so we skip by default
	t.Skip()

	grpcAddress := "localhost:3600"
	chain := flow.Emulator.Chain()
	debugger := debug.NewRemoteDebugger(grpcAddress, chain, zerolog.New(os.Stdout).With().Logger())

	const scriptTemplate = `
	import FlowServiceAccount from 0x%s

	transaction() {
		prepare(signer: AuthAccount) {
			log(signer.balance)
		}
	  }
	`

	script := []byte(fmt.Sprintf(scriptTemplate, chain.ServiceAddress()))
	txBody := flow.NewTransactionBody().
		SetGasLimit(9999).
		SetScript([]byte(script)).
		SetPayer(chain.ServiceAddress()).
		SetProposalKey(chain.ServiceAddress(), 0, 0)
	txBody.Authorizers = []flow.Address{chain.ServiceAddress()}

	// Run at the latest blockID
	txErr, err := debugger.RunTransaction(txBody)
	require.NoError(t, txErr)
	require.NoError(t, err)

	// Run with blockID (use the file cache)
	blockId, err := flow.HexStringToIdentifier("3a8281395e2c1aaa3b8643d148594b19e2acb477611a8e0cab8a55c46c40b563")
	require.NoError(t, err)
	txErr, err = debugger.RunTransactionAtBlockID(txBody, blockId, "")
	require.NoError(t, txErr)
	require.NoError(t, err)

	testCacheFile := "test.cache"
	defer os.Remove(testCacheFile)
	// the first run would cache the results
	txErr, err = debugger.RunTransactionAtBlockID(txBody, blockId, testCacheFile)
	require.NoError(t, txErr)
	require.NoError(t, err)

	// second one should only use the cache
	// make blockId invalid so if it endsup looking up by id it should fail
	blockId = flow.Identifier{}
	txErr, err = debugger.RunTransactionAtBlockID(txBody, blockId, testCacheFile)
	require.NoError(t, txErr)
	require.NoError(t, err)
}
