//  ____   ____ _   _ __  __  ___  _____ ____  
// / ___| / ___| | | |  \/  |/ _ \| ____/ ___| 
// \___ \| |   | |_| | |\/| | | | |  _| \___ \ 
//  ___) | |___|  _  | |  | | |_| | |___ ___) |
// |____/ \____|_| |_|_|  |_|\___/|_____|____/ 
// 
// Made by amit @ zay.codes
// Forked from NFTStorefront as a starter with many changes
//

import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import FlowToken from 0x1654653399040a61
import SchmoesNFT from 0x6c4fe48768523577
import ZayVerifier from 0x99e68982576033cf

pub contract ZayTrader {
    // NFTStorefrontInitialized
    // This contract has been deployed.
    // Event consumers can expect future events from this contract.
    //
    pub event Initialized()

    // TradeCollectionInitialized
    // A trade resource has been created.
    // Event consumers can now expect events from this TradeCollection.
    //
    pub event TradeCollectionInitialized(tradeCollectionResourceID: UInt64)

    // TradeCollectionDestroyed
    // A TradeCollection has been destroyed.
    // Event consumers can now stop processing events from this TradeCollection.
    // Note that we do not specify an address.
    //
    pub event TradeCollectionDestroyed(tradeCollectionResourceID: UInt64)

    // TradeOfferAvailable
    // A TradeOffer has been created and added to a TradeCollection resource.
    // The Address values here are valid when the event is emitted, but
    // the state of the accounts they refer to may be changed outside of the
    // TradeCollection workflow, so be careful to check when using them.
    //
    pub event TradeOfferAvailable(
        tradeCollectionAddress: Address,
        tradeOfferResourceID: UInt64,
        tradeCollectionResourceID: UInt64,
        offeredFtAmounts: [UFix64],
        offeredFtTypes: [Type],
        offeredNftIDs: [UInt64],
        offeredNftTypes: [Type],
        requestedFtAmounts: [UFix64],
        requestedFtTypes: [Type],
        requestedNftIDs: [UInt64],
        requestedNftTypes: [Type],
        requestedAddress: Address?,
        expiration: UFix64
    )

    // TradeExecuted
    // The TradeOffer has been resolved. It has either been purchased, or removed and destroyed.
    //
    pub event TradeExecuted(
        tradeOfferResourceID: UInt64,
        tradeCollectionResourceID: UInt64,
        offeredFtAmounts: [UFix64],
        offeredFtTypes: [Type],
        offeredNftIDs: [UInt64],
        offeredNftTypes: [Type],
        requestedFtAmounts: [UFix64],
        requestedFtTypes: [Type],
        requestedNftIDs: [UInt64],
        requestedNftTypes: [Type]
    )

    pub event TradeCancelled(
        tradeOfferResourceID: UInt64,
        tradeCollectionResourceID: UInt64
    )

    // CollectionStoragePath
    // The location in storage that a TradeCollection resource should be located.
    //
    pub let CollectionStoragePath: StoragePath

    // CollectionPublicPath
    // The public location for a TradeCollection link.
    //
    pub let CollectionPublicPath: PublicPath

    // CollectionPrivatePath
    // The private location for a TradeCollection manager link
    //
    pub let CollectionPrivatePath: PrivatePath

    // AdminStoragePath
    // The location in storage for the Admin resource
    //
    pub let AdminStoragePath: StoragePath

    // feePerTrader
    // The amount of fee required for each trader in order to fulfill this trade
    //
    pub var feePerTrader: UFix64

    // feeReceiver
    // The admin Flow account that receives the fee
    //
    pub var feeReceiver: Capability<&{FungibleToken.Receiver}>

    // TradeAsset
    // The piece of a trade that is required in all trades.
    // The type is needed to validate that types match when
    // confirming a trade
    //
    pub struct interface TradeAsset {
      pub let type: Type
    }

    // NftTradeAsset
    // The field to identify a valid NFT within a trade
    // The nftID + the type provided by a `TradeAsset` combined
    // allow for verification of a valid NFT being transferred
    //
    pub struct interface NftTradeAsset {
      pub let nftID: UInt64
    }

    // FungibleTradeAsset
    // The field to identify a valid FungibleToken within a trade
    // The amount + the type provided by a `TradeAsset` combined
    // allow for verification of a valid amount of a token
    // is being transferred as part of this trade
    //
    pub struct interface FungibleTradeAsset {
      pub let amount: UFix64
    }

    // Offered NFT structs include a capability that allows for direct access to user's collections for their assets
    // These are needed in order to complete a trade without needing to transfer all assets to a new temporary
    // collection. The offered NFT and FT trade asset structs are also used by the requested trader in order to
    // provide the capabilities to complete the trade on both sides.

    // OfferedNftTradeAsset
    // Offered NFT as part of a trade
    //
    pub struct OfferedNftTradeAsset: TradeAsset, NftTradeAsset {
      pub let type: Type
      pub let nftID: UInt64
      access(contract) let nftCollectionAccess: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>

      pub fun getReadable(): {String: AnyStruct} {
          return {
              "nftID": self.nftID,
              "type": self.type
          }
      }

      init(
        type: Type,
        nftID: UInt64,
        nftCollectionAccess: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>
      ) {
          self.type = type
          self.nftID = nftID
          self.nftCollectionAccess = nftCollectionAccess
      }
    }
    
    // OfferedFungibleTradeAsset
    // Offered fungible token as part of a trade
    //
    pub struct OfferedFungibleTradeAsset: TradeAsset, FungibleTradeAsset {
      pub let type: Type
      pub let amount: UFix64
      access(contract) let fungibleTokenAccess: Capability<&{FungibleToken.Provider, FungibleToken.Balance}>

      pub fun getReadable(): {String: AnyStruct} {
          return {
              "amount": self.amount,
              "type": self.type
          }
      }

      init(
        type: Type,
        amount: UFix64,
        fungibleTokenAccess: Capability<&{FungibleToken.Provider, FungibleToken.Balance}>
      ) {
          self.type = type
          self.amount = amount
          self.fungibleTokenAccess = fungibleTokenAccess
      }
    }

    // RequestedNftTradeAsset
    // Requested assets - access to the capability that can view the assets
    // And a place to deposit the asset
    //
    pub struct RequestedNftTradeAsset: TradeAsset, NftTradeAsset {
      pub let type: Type
      pub let nftID: UInt64
      pub let offererReceiverCapability: Capability<&{NonFungibleToken.CollectionPublic}>

      pub fun getReadable(): {String: AnyStruct} {
          return {
              "nftID": self.nftID,
              "type": self.type
          }
      }

      init(
        type: Type,
        nftID: UInt64,
        offererReceiverCapability: Capability<&{NonFungibleToken.CollectionPublic}>
      ) {
          self.type = type
          self.nftID = nftID
          self.offererReceiverCapability = offererReceiverCapability
      }
    }

    // RequestedFungibleTradeAsset
    // Requested Fungible token amount and where it should be deposited
    //
    pub struct RequestedFungibleTradeAsset: TradeAsset, FungibleTradeAsset {
      pub let amount: UFix64
      pub let type: Type
      pub let offererReceiverCapability: Capability<&{FungibleToken.Receiver}>

      pub fun getReadable(): {String: AnyStruct} {
          return {
              "amount": self.amount,
              "type": self.type
          }
      }

      init(
        type: Type,
        amount: UFix64,
        offererReceiverCapability: Capability<&{FungibleToken.Receiver}>
      ) {
          self.type = type
          self.amount = amount
          self.offererReceiverCapability = offererReceiverCapability
      }

    }

    // TradeOfferDetails
    // A struct containing a TradeOffer's data.
    //
    pub struct TradeOfferDetails {
        // The Resource ID that this TradeOffer refers to.
        pub let tradeOfferResourceID: UInt64

        // The TradeCollection that the TradeOffer is stored in.
        pub let tradeCollectionID: UInt64

        // Whether this tradeoffer TradeOffer been executed or not.
        pub var executed: Bool

        // When this trade offer should no longer be eligible to be accepted
        pub let expiration: UFix64

        // The requested items the TradeOffer creator is expected to get back from the assets
        // they have offered
        pub let requestedNfts: [RequestedNftTradeAsset]
        pub let requestedFts: [RequestedFungibleTradeAsset]

        // The requested address is what address is expected to complete this trade.
        // This is verified by way of a time-bound signature that is expected to be provided
        // by an account, if this requestedAddress exists
        pub let requestedAddress: Address?

        // The items that the TradeOffer creator is expected to give up in this trade
        access(contract) let offeredNfts: [OfferedNftTradeAsset]
        access(contract) let offeredFts: [OfferedFungibleTradeAsset]

        // Fees are taken from this provided capability when the trade is being executed
        access(contract) let offerFeePayer: Capability<&FlowToken.Vault{FungibleToken.Provider}>

        access(contract) let offerFee: UFix64

        access(contract) fun setToExecuted() {
            self.executed = true
        }

        // -----------------

        // getReadableTradeOfferDetails
        // It is difficult to read the TradeOfferDetails as a struct, and faces
        // JS conversion problems. This converts it to an easier readable format
        //
        pub fun getReadableTradeOfferDetails(): {String: AnyStruct} {
            let map: {String: AnyStruct} = {}
            map["tradeCollectionID"] = self.tradeCollectionID
            map["tradeOfferResourceID"] = self.tradeOfferResourceID
            map["executed"] = self.executed
            map["expiration"] = self.expiration
            map["requestedAddress"] = self.requestedAddress

            let offeredNftArr: [{String: AnyStruct}] = []
            let offeredFtArr: [{String: AnyStruct}] = []
            let requestedNftArr: [{String: AnyStruct}] = []
            let requestedFtArr: [{String: AnyStruct}] = []

            var index = 0
            while (index < self.offeredNfts.length) {
                offeredNftArr.append(self.offeredNfts[index].getReadable())
                index = index + 1
            }

            index = 0
            while (index < self.offeredFts.length) {
                offeredFtArr.append(self.offeredFts[index].getReadable())
                index = index + 1
            }

            index = 0
            while (index < self.requestedNfts.length) {
                requestedNftArr.append(self.requestedNfts[index].getReadable())
                index = index + 1
            }

            index = 0
            while (index < self.requestedFts.length) {
                requestedFtArr.append(self.requestedFts[index].getReadable())
                index = index + 1
            }

            map["offeredNfts"] = offeredNftArr
            map["offeredFts"] = offeredFtArr
            map["requestedNfts"] = requestedNftArr
            map["requestedFts"] = requestedFtArr
            return map
        }

        // initializer
        //
        init (
          tradeOfferResourceID: UInt64,
          tradeCollectionID: UInt64,
          offeredNfts: [OfferedNftTradeAsset],
          offeredFts: [OfferedFungibleTradeAsset],
          offerFeePayer: Capability<&FlowToken.Vault{FungibleToken.Provider}>,
          offerFee: UFix64,
          requestedNfts: [RequestedNftTradeAsset],
          requestedFts: [RequestedFungibleTradeAsset],
          requestedAddress: Address?,
          expiration: UFix64
        ) {
            self.tradeOfferResourceID = tradeOfferResourceID
            self.tradeCollectionID = tradeCollectionID
            self.offeredNfts = offeredNfts
            self.offeredFts = offeredFts
            self.offerFeePayer  = offerFeePayer
            self.offerFee = offerFee
            self.requestedNfts = requestedNfts
            self.requestedFts = requestedFts
            self.requestedAddress = requestedAddress
            self.expiration = expiration
            self.executed = false
        }
    }

    // TradeBundle
    // Bundles all assets from the result of executing a trade to a single resource
    // in order to return the resource back to the caller, where the resources
    // can then be moved to appropriate locations as desired.
    pub resource TradeBundle {
        pub var fungibleAssets: @[FungibleToken.Vault]
        pub var nftAssets: @[NonFungibleToken.NFT]

        pub fun extractAllFungibleAssets(): @[FungibleToken.Vault] {
            var assets: @[FungibleToken.Vault] <- []
            self.fungibleAssets <-> assets
            return <-assets
        }

        pub fun extractAllNftAssets(): @[NonFungibleToken.NFT] {
            var assets: @[NonFungibleToken.NFT] <- []
            self.nftAssets <-> assets
            return <-assets
        }

        init(fungibleAssets: @[FungibleToken.Vault], nftAssets: @[NonFungibleToken.NFT]) {
            self.fungibleAssets <- fungibleAssets
            self.nftAssets <- nftAssets
        }

        destroy() {
            assert(self.fungibleAssets.length == 0, message: "Can not destroy trade bundle with ft resources within")
            assert(self.nftAssets.length == 0, message: "Can not destroy trade bundle with nft resources within")
            destroy self.fungibleAssets
            destroy self.nftAssets
        }
    }

    // TradeOfferPublic
    // An interface providing a useful public interface to a TradeOffer.
    //
    pub resource interface TradeOfferPublic {
        // acceptTrade
        // Accept the TradeOffer, buying the offered items and providing
        // the requested assets in return.
        // Optionally, params for a signature may be provided. This is required
        // if the referenced trade has a specific address that is allowed to
        // fulfill the trade. Also required if a Schmoe is wanted to be used
        // to void the otherwise required trading fee
        //
        pub fun acceptTrade(
            tradeBundle: @TradeBundle,
            fee: @FungibleToken.Vault,
            signingAddress: Address?,
            signedMessage: String?,
            keyIds: [Int],
            signatures: [String],
            signatureBlock: UInt64?
        ): @TradeBundle

        // getDetails
        // Receive details of this trade offer
        //
        pub fun getDetails(): TradeOfferDetails
    }


    // TradeOffer
    // A resource that allows a group of NFTs and FungibleTokens to be offered
    // in an exchange for another group of NFTs and FungibleTokens
    // 
    pub resource TradeOffer: TradeOfferPublic {
        // The details of the trade
        access(self) let details: TradeOfferDetails

        // getDetails
        // Get the details of the current state of the TradeOffer as a struct.
        //
        pub fun getDetails(): TradeOfferDetails {
            return self.details
        }

        // executeTrade
        // Execute the trade, providing access to the requested resources
        // and receiving back all of the initially offered resources
        // emptyFtVaults is needed in order to fill in the resulting tradebundle.
        //
        access(self) fun executeTrade(
            tradeBundle: @TradeBundle
        ): @TradeBundle {
            pre {
                self.details.executed == false: "Trade has already been executed"
                getCurrentBlock().timestamp < self.details.expiration : "Trade has expired, unable to execute trade"
            }
            // As the assets are pulled out of the TradeBundle to execute the trade, the following is also done:
            //  - Assert that all of the offered items are accessible to fit the trade parameters.
            //  - Assert that all of the requested items are present to fit the trade parameters

            // ------------------------------------------------------

            // Fields needed for emitting TradeExecuted event
            let offeredFtAmounts: [UFix64] = []
            let offeredFtTypes: [Type] = []
            let offeredNftIDs: [UInt64] = []
            let offeredNftTypes: [Type] = []
            let requestedFtAmounts: [UFix64] = []
            let requestedFtTypes: [Type] = []
            let requestedNftIDs: [UInt64] = []
            let requestedNftTypes: [Type] = []

            // ------------------------------------------------------

            // Create a TradeBundle by withdrawing from the `offerers` collections and vaults
            let offererNfts: @[NonFungibleToken.NFT] <- []
            let offererFts: @[FungibleToken.Vault] <- []

            // Loop through all previously offered NFT assets and withdraw -> deposit them to the newly provided receivers
            var index = 0
            while index < self.details.offeredNfts.length {
                let offeredNftAsset = self.details.offeredNfts[index]
                let provider = offeredNftAsset.nftCollectionAccess.borrow()

                assert(provider != nil, message: "Unable to borrow collection to withdraw offered NFT.")
                let nft <- provider!.withdraw(withdrawID: offeredNftAsset.nftID)

                // Verify the originally offered NFT matches the currently withdrawn NFT
                assert(nft.isInstance(offeredNftAsset.type), message: "Wrong type received from offered collection")
                assert(nft.id == offeredNftAsset.nftID, message: "Wrong nft ID received from offered collection")
                
                offeredNftIDs.append(nft.id)
                offeredNftTypes.append(nft.getType())

                offererNfts.append(<-nft)

                // Continue to the next NFT
                index = index + 1
            }
            assert(self.details.offeredNfts.length == offererNfts.length, message: "Invalid amount of NFTs in resulting bundle.")

            // Loop through all previously offered Fungible assets and withdraw -> deposit them to the newly provided receivers
            index = 0
            while index < self.details.offeredFts.length {
                let offeredFtAsset = self.details.offeredFts[index]
                let provider = offeredFtAsset.fungibleTokenAccess.borrow()
                
                assert(provider != nil, message: "Unable to borrow collection to withdraw offered FT")
                assert(provider!.isInstance(offeredFtAsset.type), message: "Invalid FT type provided by offered capability")
                
                // If we attempt to withdraw more than is available, the following will fail.
                // We aren't validating the balance is high enough before withdrawing because
                // it is not needed (would fail anyways because of FungibleToken defaults), and
                // with flow tokens that assert can be inaccurate due to storage costs
                let withdrawnVault <- provider!.withdraw(amount: offeredFtAsset.amount)
                offererFts.append(<-withdrawnVault)

                offeredFtAmounts.append(offeredFtAsset.amount)
                offeredFtTypes.append(provider!.getType())

                // Continue to the next FT vault
                index = index + 1
            }
            
            let finalTradeBundle <- create TradeBundle(
                fungibleAssets: <-offererFts,
                nftAssets: <-offererNfts
            )

            // ------------------------------------------------------   

            // Deposit the given trade assets to the `offerer` the provided nft assets and fungible assets
            // Provided offered NFTs are expected to map 1-1 to the requestedNftTradeAssets from the trade details
            
            let providedNfts:@[NonFungibleToken.NFT] <- tradeBundle.extractAllNftAssets()
            let providedFts:@[FungibleToken.Vault] <- tradeBundle.extractAllFungibleAssets()
            destroy tradeBundle
            
            // Loop through all provided NFT assets, verify their validity
            // and withdraw -> deposit them to the original offering party
            index = 0
            assert(self.details.requestedNfts.length == providedNfts.length, message: "Mismatch in number of nfts provided and requested.")
            while index < self.details.requestedNfts.length {
                let requestedNft = self.details.requestedNfts[index]
                let nft <- providedNfts.remove(at:0)
                
                // Verify the provided NFT matches the requested one
                assert(nft.isInstance(requestedNft.type), message: "Provided NFT type does not match requested NFT type")
                assert(nft.id == requestedNft.nftID, message: "Provided NFT ID does not match requested NFT")
                let nftReceiver = requestedNft.offererReceiverCapability.borrow()
                assert(nftReceiver != nil, message: "Unable to borrow offer creator NFT receiver")

                requestedNftTypes.append(nft.getType())
                requestedNftIDs.append(nft.id)

                // deposit the NFT
                nftReceiver!.deposit(token: <-nft)

                // Continue to the next NFT
                index = index + 1
            }
            assert(providedNfts.length == 0, message: "Failed to use/transfer all provided nfts.")
            destroy providedNfts

            // Loop through all provided FT assets, and withdraw -> deposit them to the original offering party
            index = 0
            while index < self.details.requestedFts.length {
                let requestedFt = self.details.requestedFts[index]
                let ftVault <- providedFts.remove(at:0)

                // Verify the provided fungible token matches the requested one
                assert(ftVault.isInstance(requestedFt.type), message: "Provided FT type does not match requested ft type")
                assert(ftVault.balance == requestedFt.amount, message: "Provided FT amount does not match requested amount")
                let ftReceiver = requestedFt.offererReceiverCapability.borrow()
                assert(ftReceiver != nil, message: "Unable to borrow offer creator FT receiver")

                requestedFtAmounts.append(ftVault.balance)
                requestedFtTypes.append(ftVault.getType())

                // deposit the tokens
                ftReceiver!.deposit(from: <- ftVault.withdraw(amount: ftVault.balance))
                
                // destroy the empty vault
                destroy ftVault

                // Continue to the next FT Vault
                index = index + 1
            }
            assert(providedFts.length == 0, message: "Failed to use/transfer all provided nfts.")
            destroy providedFts

            // ------------------------------------------------------

            self.details.setToExecuted()

            emit TradeExecuted(
                tradeOfferResourceID: self.uuid,
                tradeCollectionResourceID: self.details.tradeCollectionID,
                offeredFtAmounts: offeredFtAmounts,
                offeredFtTypes: offeredFtTypes,
                offeredNftIDs: offeredNftIDs,
                offeredNftTypes: offeredNftTypes,
                requestedFtAmounts: requestedFtAmounts,
                requestedFtTypes: requestedFtTypes,
                requestedNftIDs: requestedNftIDs,
                requestedNftTypes: requestedNftTypes
            )
            return <-finalTradeBundle
        }

        pub fun acceptTrade(
            tradeBundle: @TradeBundle,
            fee: @FungibleToken.Vault,
            signingAddress: Address?,
            signedMessage: String?,
            keyIds: [Int],
            signatures: [String],
            signatureBlock: UInt64?
        ): @TradeBundle {
            
            let signatureRequired = self.details.requestedAddress != nil
            assert(
                self.details.requestedAddress == nil || (signingAddress! == self.details.requestedAddress!),
                message: "Requested trade account does not match provided account or expected signature not provided"
            )

            // Check provided signature is valid for the 
            // signing account if it was provided
            var validSignature = false
            if (signingAddress != nil && signedMessage != nil) {
                let signatureTimestamp = ZayVerifier.verifySignature(
                    acctAddress: signingAddress!,
                    message: signedMessage!,
                    keyIds: keyIds,
                    signatures: signatures,
                    signatureBlock: signatureBlock!
                )
                assert(signatureTimestamp != nil, message: "Invalid signature provided.")

                let curTime = getCurrentBlock().timestamp
                let timePassedSinceSignature = curTime - signatureTimestamp!

                // If the signature happened less than 10 minutes ago, consider the signature valid
                assert(timePassedSinceSignature < UFix64(10 * 60), message: "Invalid signature provided")
                validSignature = true
            }
            
            assert(validSignature || !signatureRequired, message: "Proof of requested account ownership failed")
            
            var accountHasSchmoe = false
            // If the user provided a valid signature, also check if they have a schmoe
            // in a standard public schmoe collection
            if (validSignature) {
                accountHasSchmoe = ZayVerifier.checkOwnership(
                    address: signingAddress!,
                    collectionPath: SchmoesNFT.CollectionPublicPath,
                    nftType: Type<@SchmoesNFT.NFT>()
                )
            }

            if (accountHasSchmoe) {
                // If the user has a schmoe, and provided a fee with balance, fail the call
                // because we do not want to take their fees
                assert(fee.balance == 0.0, message: "Fee provided without the need for fee due to schmoe ownership")
                destroy fee
            } else {
                // If the user doesn't have a schmoe, then ensure a fee was provided equal to
                // the current expected flat fee
                assert(fee.balance == ZayTrader.feePerTrader, message: "Invalid balance provided for free.")
                ZayTrader.feeReceiver.borrow()!.deposit(from: <-fee.withdraw(amount: fee.balance))
                destroy fee
            }

            return <-self.executeTrade(tradeBundle: <-tradeBundle)
        }

        // destructor
        //
        destroy () {
            // If the trade has not been executed and we are destroying it
            // we must notify that this trade is no longer valid. If the trade
            // has already been executed, the executed flag would be true
            // and we do not need to emit this twice
            if !self.details.executed {
                emit TradeCancelled(
                    tradeOfferResourceID: self.uuid,
                    tradeCollectionResourceID: self.details.tradeCollectionID
                )
            }
        }

        // initializer
        //
        init (
            tradeCollectionID: UInt64,
            offeredNfts: [OfferedNftTradeAsset],
            offeredFts: [OfferedFungibleTradeAsset],
            offerFeePayer: Capability<&FlowToken.Vault{FungibleToken.Provider}>,
            offerFee: UFix64,
            requestedNfts: [RequestedNftTradeAsset],
            requestedFts: [RequestedFungibleTradeAsset],
            creatorAddress: Address,
            requestedAddress: Address?,
            expiration: UFix64,
        ) {
            // Store the trade information and capabilities to access
            // the needed trade assets at execution time
            self.details = TradeOfferDetails(
                tradeOfferResourceID: self.uuid,
                tradeCollectionID: tradeCollectionID,
                offeredNfts: offeredNfts,
                offeredFts: offeredFts,
                offerFeePayer: offerFeePayer,
                offerFee: offerFee,
                requestedNfts: requestedNfts,
                requestedFts: requestedFts,
                requestedAddress: requestedAddress,
                expiration: expiration
            )

            // Verify that all of the provided capabilities and nfts/fts are formed validly
            // We cannot move the following verification steps into a function because
            // initializers cannot call member functions.
            // The verification that is done here is just to validate this is a valid trade
            // to create - the execution of the trade will repeat this verification to ensure
            // everything remains accessible and valid true at the time of execution

            let offeredFtAmounts: [UFix64] = []
            let offeredFtTypes: [Type] = []
            let offeredNftIDs: [UInt64] = []
            let offeredNftTypes: [Type] = []
            let requestedFtAmounts: [UFix64] = []
            let requestedFtTypes: [Type] = []
            let requestedNftIDs: [UInt64] = []
            let requestedNftTypes: [Type] = []

            // Must verify all offered and requested NFT collections exist as stated
            // and have the valid ID as specified in the TradeAsset
            // nftCollectionAccess fungibleTokenAccess
            for tradeAsset in self.details.offeredNfts {
                let provider = tradeAsset.nftCollectionAccess.borrow()
                assert(provider != nil, message: "Cannot borrow one of the provided offered NFTs")
                let nft = provider!.borrowNFT(id: tradeAsset.nftID)
                assert(nft.isInstance(tradeAsset.type), message: "Token is not of specified type")
                assert(nft.id == tradeAsset.nftID, message: "Token does not have expected ID")
                offeredNftTypes.append(nft.getType())
                offeredNftIDs.append(nft.id)
            }
            
            // Verify all offered and requested FTs capabilities
            // exist and have valid balances attached to them
            for tradeAsset in self.details.offeredFts {
                let provider = tradeAsset.fungibleTokenAccess.borrow()
                assert(provider != nil, message: "Cannot borrow provided FT capability")
                let ft = provider!
                assert(ft.isInstance(tradeAsset.type), message: "Token is not of specified type")
                assert(ft.balance > tradeAsset.amount, message: "Not a large enough balance present for trade")
                offeredFtAmounts.append(ft.balance)
                offeredFtTypes.append(ft.getType())
            }

            for tradeAsset in self.details.requestedNfts {
                requestedNftTypes.append(tradeAsset.type)
                requestedNftIDs.append(tradeAsset.nftID)
            }
            for tradeAsset in self.details.requestedFts {
                requestedFtTypes.append(tradeAsset.type)
                requestedFtAmounts.append(tradeAsset.amount)
            }

            emit TradeOfferAvailable(
                tradeCollectionAddress: creatorAddress,
                tradeOfferResourceID: self.uuid,
                tradeCollectionResourceID: tradeCollectionID,
                offeredFtAmounts: offeredFtAmounts,
                offeredFtTypes: offeredFtTypes,
                offeredNftIDs: offeredNftIDs,
                offeredNftTypes: offeredNftTypes,
                requestedFtAmounts: requestedFtAmounts,
                requestedFtTypes: requestedFtTypes,
                requestedNftIDs: requestedNftIDs,
                requestedNftTypes: requestedNftTypes,
                requestedAddress: requestedAddress,
                expiration: expiration
            )
        }
    }

    // TradeCollectionManager
    // An interface for adding and removing TradeOffers within a TradeCollection,
    // intended for use by the TradeCollections's owner
    //
    pub resource interface TradeCollectionManager {
        // createListing
        // Allows the TradeCollection owner to create and insert TradeOffers.
        //
        pub fun createTradeOffer(
                offeredNfts: [OfferedNftTradeAsset],
                offeredFts: [OfferedFungibleTradeAsset],
                offerFeePayer: Capability<&FlowToken.Vault{FungibleToken.Provider}>,
                requestedNfts: [RequestedNftTradeAsset],
                requestedFts: [RequestedFungibleTradeAsset],
                expiration: UFix64,
                requestedAddress: Address?,
                signingAddress: Address?,
                signedMessage: String?,
                keyIds: [Int],
                signatures: [String],
                signatureBlock: UInt64?
        ): UInt64

        // removeListing
        // Allows the TradeCollection owner to remove any open trade listing, accepted or not.
        //
        pub fun removeTradeOffer(tradeOfferResourceID: UInt64)
    }

    // TradeCollectionPublic
    // An interface to allow viewing and borrowing of open trade offers, and
    // and execute trades given that the proper assets are provided in return
    //
    pub resource interface TradeCollectionPublic {
        pub fun getTradeOfferIDs(): [UInt64]
        pub fun borrowTradeOffer(tradeOfferResourceID: UInt64): &TradeOffer{TradeOfferPublic}?
        pub fun cleanup(tradeOfferResourceID: UInt64)
   }

    // TradeCollection
    // A resource that allows its owner to manage a list of open trade offers, and anyone to interact with them
    // in order to query their details and execute a trade if they have the requested assets
    //
    pub resource TradeCollection : TradeCollectionManager, TradeCollectionPublic {
        // The dictionary of Listing uuids to Listing resources.
        access(self) var tradeOffers: @{UInt64: TradeOffer}

        // insert
        // Create and publish a listing for a new trade offer.
        //
        pub fun createTradeOffer(
                offeredNfts: [OfferedNftTradeAsset],
                offeredFts: [OfferedFungibleTradeAsset],
                offerFeePayer: Capability<&FlowToken.Vault{FungibleToken.Provider}>,
                requestedNfts: [RequestedNftTradeAsset],
                requestedFts: [RequestedFungibleTradeAsset],
                expiration: UFix64,
                requestedAddress: Address?,
                signingAddress: Address?,
                signedMessage: String?,
                keyIds: [Int],
                signatures: [String],
                signatureBlock: UInt64?
        ): UInt64 {
            var accountHasSchmoe = false
            // If a signature was provided, check if the account has a schmoe
            // And if the account does not have a schmoe, apply the standard fee
            // as part of the trade offer.
            if (signingAddress != nil && signedMessage != nil) {
                let signatureTimestamp = ZayVerifier.verifySignature(
                    acctAddress: signingAddress!,
                    message: signedMessage!,
                    keyIds: keyIds,
                    signatures: signatures,
                    signatureBlock: signatureBlock!
                )
                assert(signatureTimestamp != nil, message: "Invalid signature provided.")
                let curTime = getCurrentBlock().timestamp
                let timePassedSinceSignature = curTime - signatureTimestamp! 
                // If the signature happened less than 10 minutes ago, allow the code to continue
                assert(timePassedSinceSignature < UFix64(10 * 60), message: "Invalid signature provided")
                accountHasSchmoe = ZayVerifier.checkOwnership(
                    address: signingAddress!,
                    collectionPath: SchmoesNFT.CollectionPublicPath,
                    nftType: Type<@SchmoesNFT.NFT>()
                )
            }

            var requiredFeeAmount = 0.0
            if (!accountHasSchmoe) {
                requiredFeeAmount = ZayTrader.feePerTrader
            }

            let tradeOffer <- create TradeOffer(
                tradeCollectionID: self.uuid,
                offeredNfts: offeredNfts,
                offeredFts: offeredFts,
                offerFeePayer: offerFeePayer,
                offerFee: requiredFeeAmount,
                requestedNfts: requestedNfts,
                requestedFts: requestedFts,
                creatorAddress: self.owner?.address!,
                requestedAddress: requestedAddress,
                expiration: expiration
            )
            let tradeOfferResourceID = tradeOffer.uuid

            // Add the new offer to the dictionary.
            let oldOffer <- self.tradeOffers[tradeOfferResourceID] <- tradeOffer

            // Destroy the old offer, which won't exist because this is a new resource ID
            // being placed into the map
            destroy oldOffer

            return tradeOfferResourceID
        }

        // removeTradeOffer
        // Remove a TradeOffer that has not yet been executed from the collection and destroy it.
        //
        pub fun removeTradeOffer(tradeOfferResourceID: UInt64) {
            let tradeOfferResource <- self.tradeOffers.remove(key: tradeOfferResourceID)
                ?? panic("missing Listing")
    
            // This will emit a TradeCancelled event.
            destroy tradeOfferResource
        }

        // getTradeOfferIDs
        // Returns an array of the TradeOffer resource IDs that are in the collection
        //
        pub fun getTradeOfferIDs(): [UInt64] {
            return self.tradeOffers.keys
        }

        // borrowTradeOffer
        // Returns a read-only view of the TradeOffer for the given tradeOfferResourceID if it is contained by this collection.
        //
        pub fun borrowTradeOffer(tradeOfferResourceID: UInt64): &TradeOffer{TradeOfferPublic}? {
            if self.tradeOffers[tradeOfferResourceID] != nil {
                return &self.tradeOffers[tradeOfferResourceID] as! &TradeOffer{TradeOfferPublic}
            } else {
                return nil
            }
        }

        // cleanup
        // Remove an listing *if* it has been purchased.
        // Anyone can call, but at present it only benefits the account owner to do so.
        // Kind purchasers can however call it if they like.
        //
        pub fun cleanup(tradeOfferResourceID: UInt64) {
            pre {
                self.tradeOffers[tradeOfferResourceID] != nil: "could not find listing with given id"
            }
            let tradeOffer <- self.tradeOffers.remove(key: tradeOfferResourceID)!
            assert(tradeOffer.getDetails().executed == true, message: "trade offer has not been executed, only owner can cancel it")
            destroy tradeOffer
        }

        // destructor
        //
        destroy () {
            destroy self.tradeOffers

            // Let event consumers know that this storefront will no longer exist
            emit TradeCollectionDestroyed(tradeCollectionResourceID: self.uuid)
        }

        // constructor
        //
        init () {
            self.tradeOffers <- {}

            // Let event consumers know that this storefront exists
            emit TradeCollectionInitialized(tradeCollectionResourceID: self.uuid)
        }
    }

    // Admin resource allowing for the contract administrator to make specific changes
    // The only intended control over the contract from an administrator is over the
    // fees
    pub resource Admin {

        // Update the capability we use to send fees to
        pub fun updateFeeReceiver(cap: Capability<&FlowToken.Vault{FungibleToken.Receiver}>) {
            ZayTrader.feeReceiver = cap
        }

        // Update the amount received from fees
        pub fun updateFeeAmount(amount: UFix64) {
            ZayTrader.feePerTrader = amount
        }

    }

    // Public functions

    // createTradeCollection
    // Make creating a TradeCollection publicly accessible.
    //
    pub fun createTradeCollection(): @TradeCollection {
        return <-create TradeCollection()
    }

    // createTradeBundle
    // Make a TradeBundle for transferring many trade assets
    //
    pub fun createTradeBundle(fungibleAssets: @[FungibleToken.Vault], nftAssets: @[NonFungibleToken.NFT]): @TradeBundle {
        return <-create TradeBundle(
            fungibleAssets: <-fungibleAssets,
            nftAssets: <-nftAssets
        )
    }

    init () {
        self.CollectionStoragePath = /storage/ZayTraderCollection
        self.CollectionPublicPath = /public/ZayTraderCollection
        self.CollectionPrivatePath = /private/ZayTraderCollection

        // Fees set to 0 to start - will be updated by the admin resource
        self.feePerTrader = 0.0

        // Default to receiving fees on this central contract - this is meant
        // to be updated by an admin so that this contract can go keyless
        // when it is mature/ready to
        self.feeReceiver = self.account.getCapability<&FlowToken.Vault{FungibleToken.Receiver}>(/public/flowTokenReceiver)

        self.AdminStoragePath = /storage/ZayTraderAdmin
        self.account.save(<- create Admin(), to: self.AdminStoragePath)

        emit Initialized()
    }
}