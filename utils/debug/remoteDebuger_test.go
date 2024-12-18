package debug_test

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog/log"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
)

func TestRemoteDebug(t *testing.T) {

	executionAddress := "execution-001.devnet52.nodes.onflow.org:9000"

	computeLimit := uint64(100_000_000)
	flagAtLatestBlock := true

	chain := flow.Testnet.Chain()

	txCode := `
// Thirdparty imports
import FungibleToken from 0x9a0766d93b6608b7
import FlowToken from 0x7e60df042a9c0868
import ScopedFTProviders from 0x31ad40c07a2a9788
import FlowEVMBridgeConfig from 0xdfc20aee650fcbdf
// Fixes Imports
import Fixes from 0xb7248baa24a95c3f
import FGameLottery from 0xb7248baa24a95c3f
import FGameLotteryFactory from 0xb7248baa24a95c3f

transaction(
    ticketAmt: UInt64,
    powerupLv: UInt8,
    forFlow: Bool,
    withMinting: Bool,
) {
    let address: Address
    let store: auth(Fixes.Manage) &Fixes.InscriptionsStore
    let scopedProvider: @ScopedFTProviders.ScopedFTProvider

    prepare(acct: auth(Storage, Capabilities) &Account) {
        self.address = acct.address

        /** ------------- Prepare the Inscription Store - Start ---------------- */
        let storePath = Fixes.getFixesStoreStoragePath()
        if acct.storage
            .borrow<auth(Fixes.Manage) &Fixes.InscriptionsStore>(from: storePath) == nil {
            acct.storage.save(<- Fixes.createInscriptionsStore(), to: storePath)
        }

        self.store = acct.storage
            .borrow<auth(Fixes.Manage) &Fixes.InscriptionsStore>(from: storePath)
            ?? panic("Could not borrow a reference to the Inscriptions Store!")
        /** ------------- End -------------------------------------------------- */

        /** ------------- Initialize TicketCollection - Start ---------------- */
        // If the user doesn't have a TicketCollection yet, create one
        if acct.storage.borrow<&FGameLottery.TicketCollection>(from: FGameLottery.userCollectionStoragePath) == nil {
            acct.storage.save(<- FGameLottery.createTicketCollection(), to: FGameLottery.userCollectionStoragePath)
        }
        // Link public capability to the account
        if acct
            .capabilities.get<&FGameLottery.TicketCollection>(FGameLottery.userCollectionPublicPath)
            .borrow() == nil {
            acct.capabilities.unpublish(FGameLottery.userCollectionPublicPath)
            acct.capabilities.publish(
                acct.capabilities.storage.issue<&FGameLottery.TicketCollection>(FGameLottery.userCollectionStoragePath),
                at: FGameLottery.userCollectionPublicPath
            )
        }
        /** ------------- End ------------------------------------------------ */

        let powerupType = FGameLotteryFactory.PowerUpType(rawValue: powerupLv) ?? panic("Invalid powerup level")

        let estimateFlowCost = forFlow
            ? FGameLotteryFactory.getFIXESMintingLotteryFlowCost(ticketAmt, powerupType, withMinting)
            : FGameLotteryFactory.getFIXESLotteryFlowCost(ticketAmt, powerupType, acct.address)

        /* --- Configure a ScopedFTProvider --- */
        //
        // Issue and store bridge-dedicated Provider Capability in storage if necessary
        if acct.storage.type(at: FlowEVMBridgeConfig.providerCapabilityStoragePath) == nil {
            let providerCap = acct.capabilities.storage
                .issue<auth(FungibleToken.Withdraw) &{FungibleToken.Provider}>(/storage/flowTokenVault)
            acct.storage.save(providerCap, to: FlowEVMBridgeConfig.providerCapabilityStoragePath)
        }
        let providerCapCopy = acct.storage
            .copy<Capability<auth(FungibleToken.Withdraw) &{FungibleToken.Provider}>>(
                from: FlowEVMBridgeConfig.providerCapabilityStoragePath
            ) ?? panic("Invalid FungibleToken Provider Capability found in storage at path "
                .concat(FlowEVMBridgeConfig.providerCapabilityStoragePath.toString()))
        let providerFilter = ScopedFTProviders.AllowanceFilter(estimateFlowCost)
        self.scopedProvider <- ScopedFTProviders.createScopedFTProvider(
            provider: providerCapCopy,
            filters: [ providerFilter ],
            expiration: getCurrentBlock().timestamp + 1.0
        )
    }

    execute {
        // Get the user's TicketCollection capability
        let ticketCollectionCap = FGameLottery.getUserTicketCollection(self.address)
        assert(
            ticketCollectionCap.borrow() != nil,
            message: "Could not borrow a reference to the user's TicketCollection!"
        )

        // Purchase the lottery
        if forFlow {
            FGameLotteryFactory.buyFIXESMintingLottery(
                flowProvider:  &self.scopedProvider as auth(FungibleToken.Withdraw) &{FungibleToken.Provider},
                ticketAmount: ticketAmt,
                powerup: FGameLotteryFactory.PowerUpType(rawValue: powerupLv) ?? panic("Invalid powerup level"),
                withMinting: withMinting,
                recipient: ticketCollectionCap,
                inscriptionStore: self.store
            )
        } else {
            FGameLotteryFactory.buyFIXESLottery(
                flowProvider: &self.scopedProvider as auth(FungibleToken.Withdraw) &{FungibleToken.Provider},
                ticketAmount: ticketAmt,
                powerup: FGameLotteryFactory.PowerUpType(rawValue: powerupLv) ?? panic("Invalid powerup level"),
                recipient: ticketCollectionCap,
                inscriptionStore: self.store
            )
        }
        destroy self.scopedProvider
        log("FIXES Lottery purchased!")
    }
}
`

	fmt.Println("Debugging transaction ...")

	debugger := debug.NewRemoteDebugger(
		executionAddress,
		chain,
		log.Logger,
	)

	payer, err := common.HexToAddress("0xa51d7fe9e0080662")
	require.NoError(t, err)

	txBody := flow.NewTransactionBody().
		SetScript([]byte(txCode)).
		SetComputeLimit(computeLimit).
		SetPayer(flow.Address(payer))

	ticketAmt, err := jsoncdc.Encode(cadence.NewUInt64(1))
	require.NoError(t, err)

	powerupLv, err := jsoncdc.Encode(cadence.NewUInt8(2))
	require.NoError(t, err)

	forFlow, err := jsoncdc.Encode(cadence.NewBool(true))
	require.NoError(t, err)

	withMinting, err := jsoncdc.Encode(cadence.NewBool(false))
	require.NoError(t, err)

	txBody.AddArgument(ticketAmt)
	txBody.AddArgument(powerupLv)
	txBody.AddArgument(forFlow)
	txBody.AddArgument(withMinting)

	txBody.AddAuthorizer(flow.Address(payer))

	proposalKeySequenceNumber := uint64(812)
	keyIndex := uint32(0)

	txBody.SetProposalKey(
		flow.Address(payer),
		keyIndex,
		proposalKeySequenceNumber,
	)

	var txErr, processErr error
	if flagAtLatestBlock {
		txErr, processErr = debugger.RunTransaction(txBody)
	} else {
		txID, err := flow.HexStringToIdentifier("8db47c0c5a5d952a7e62e00447f32dd0717842a5cbeedb19c88a4233a1746406")
		require.NoError(t, err)

		//txID = flow.Identifier{}

		txErr, processErr = debugger.RunTransactionAtBlockID(
			txBody,
			txID,
			"test.cache",
		)
	}

	if txErr != nil {
		//log.Fatal().Err(txErr).Msg("transaction error")
	}
	if processErr != nil {
		//log.Fatal().Err(processErr).Msg("process error")
	}
}
