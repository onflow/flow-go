/*:
  # Evaluate.Market Marketplace Flow Smart Contract

  - Author: Evaluate.Market
  - Copyright: 2021 Evaluate.Market
*/

import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448

pub contract EMMarket {

    priv let tokenCollectionPlatforms: {String: TokenCollectionPlatform}
    priv var marketSaleCut: MarketSaleCut
    priv var marketStatus: MarketStatus

    // The location in storage that a Storefront resource should be located.
    //
    pub let EMStorefrontStoragePath: StoragePath
    // The public location for a Storefront link.
    //
    pub let EMStorefrontPublicPath: PublicPath
    // The location in storage that a the Market admin resource should be located.
    //
    pub let EMMarketAdminStoragePath: StoragePath

    // This contract has been deployed.
    // Event consumers can now expect events from this contract.
    //
    pub event EMMarketInitialized()

    // A token platform is added by an admin
    //
    pub event TokenCollectionPlatformAdded(nftType: Type)
    // A token platform is removed by an admin
    //
    pub event TokenCollectionPlatformRemoved(nftType: Type)
    // A token platform is updated by an admin
    //
    pub event TokenCollectionPlatformUpdated(nftType: Type)

    // Market sale fee is updated by an admin
    //
    pub event MarketFeeChanged()

    // Market fee receiver was added by an admin
    //
    pub event MarketFeeReceiverAdded(type: Type)

    // Market fee receiver was removed by an admin
    //
    pub event MarketFeeReceiverRemoved(type: Type)

    // Market status is updated by an admin
    //
    pub event MarketStatusChanged(marketStatus: UInt8)

    // A listing has been created and added to a Storefront resource.
    // The Address values here are valid when the event is emitted, but
    // the state of the accounts they refer to may be changed outside of the
    // Storefront workflow, so be careful to check when using them.
    //
    pub event ListingAvailable(
        seller: Address,
        listingResourceID: UInt64,
        nftType: Type,
        nftID: UInt64,
        ftVaultType: Type,
        price: UFix64
    )

    // The listing has been been purchased.
    //
    pub event ListingCompleted(
        seller: Address,
        listingResourceID: UInt64,
        nftType: Type,
        nftID: UInt64,
        price: UFix64
    )

    // The listing has been removed.
    //
    pub event ListingRemoved(
        seller: Address,
        listingResourceID: UInt64,
        nftType: Type,
        nftID: UInt64
    )

    // emitted when the price of a listed token has changed
    //
    pub event ListingSaleCutsChanged(
        seller: Address,
        listingResourceID: UInt64,
        nftType: Type,
        nftID: UInt64,
        saleCuts: [UFix64],
        salePrice: UFix64
    )

    // An interface to allow listing and borrowing Listings, and purchasing items via Listings
    // in a Storefront.
    //
    pub resource interface StorefrontPublic {
        pub fun getListingIDs(): [UInt64]
        pub fun getListings(): {UInt64: ListingDetails}
        pub fun purchaseListing(listingResourceID: UInt64, paymentVault: @FungibleToken.Vault): @NonFungibleToken.NFT
        pub fun borrowListing(listingResourceID: UInt64): &Listing{ListingPublic}?
    }

    // An interface for adding and removing Listings within a Storefront,
    // intended for use by the Storefront's own
    //
    pub resource interface StorefrontManager {
        // createListing
        // Allows the Storefront owner to create and insert Listings.
        //
        pub fun createListing(
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
            saleCuts: [ListingSaleCut]
        ): UInt64
        // removeListing
        // Allows the Storefront owner to remove any sale listing, acepted or not.
        //
        pub fun removeListing(listingResourceID: UInt64)

        pub fun borrowManagerListing(listingResourceID: UInt64): &Listing?
    }

    pub resource EMStorefront: StorefrontManager, StorefrontPublic {

        priv var listings: @{UInt64: Listing}

        init () {
            self.listings <- {}
        }

        pub fun createListing(
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
            saleCuts: [ListingSaleCut]
        ): UInt64 {

            pre {
                EMMarket.marketStatus != MarketStatus.disabled : "Marketplace has been disabled.  Please try again later."
                EMMarket.marketStatus != MarketStatus.restrictListing : "Marketplace is currently not allowing any changes to listings.  Please try again later."     
            }

            self.cleanup()

            let listingDetails = self.getListings()

            for resourceListingId in listingDetails.keys {
                let hasNFT = self.borrowListing(listingResourceID: resourceListingId)!.hasNFT()
                let details = listingDetails[resourceListingId]!
                if details.nftType == nftType && details.nftID == nftID && hasNFT {
                    panic("This NFT has already been listed in the storefront.")
                }
            }

            let listing <- create Listing(
                nftProviderCapability: nftProviderCapability,
                nftType: nftType,
                nftID: nftID,
                salePaymentVaultType: salePaymentVaultType,
                saleCuts: saleCuts
            )

            let listingResourceID = listing.uuid
            let listingPrice = listing.getDetails().salePrice

            // Add the new listing to the dictionary.
            let oldListing <- self.listings[listingResourceID] <- listing
            // Note that oldListing will always be nil, but we have to handle it.
            destroy oldListing

            emit ListingAvailable(
                seller: self.owner?.address!,
                listingResourceID: listingResourceID,
                nftType: nftType,
                nftID: nftID,
                ftVaultType: salePaymentVaultType,
                price: listingPrice
            )

            return listingResourceID
        }

        // removeListing
        // Remove a Listing that has not yet been purchased from the collection and destroy it.
        //
        pub fun removeListing(listingResourceID: UInt64) {
            pre {
                EMMarket.marketStatus != MarketStatus.disabled : "Marketplace has been disabled.  Please try again later."
                EMMarket.marketStatus != MarketStatus.restrictDelisting : "Marketplace is currently not allowing any changes to listings.  Please try again later."
                self.listings[listingResourceID] != nil: "could not find listing with given id"
            }

            let listing <- self.listings.remove(key: listingResourceID)
                ?? panic("missing Listing")
    
            // This will emit a ListingCompleted event.
            destroy listing
        }

        // Remove an listing *if* it is no longer owned.
        // Anyone can call, but at present it only benefits the account owner to do so.
        // Kind purchasers can however call it if they like.
        //
        priv fun cleanup() {

            let listingDetails = self.getListings()
            for resourceListingId in listingDetails.keys {
                let hasNFT = self.borrowListing(listingResourceID: resourceListingId)!.hasNFT()
                let details = listingDetails[resourceListingId]!
                let wasPurchased = details.purchased
                if !hasNFT || wasPurchased {
                    let listing <- self.listings.remove(key: resourceListingId)!
                    destroy listing
                }
            }

        }

        // getListingIDs
        // Returns an array of the Listing resource IDs that are in the collection
        //
        pub fun getListingIDs(): [UInt64] {
            return self.listings.keys
        }

        pub fun getListings(): {UInt64: ListingDetails} {
            let listings: {UInt64: ListingDetails} = {}
            for listingResourceID in self.getListingIDs() {
                listings[listingResourceID] = self.borrowListing(listingResourceID: listingResourceID)!.getDetails()
            }
            return listings
        }

        // purchaseListing
        // Returns the purchased NFT from the listing given the listingID if it is contained by this collection.
        //
        pub fun purchaseListing(listingResourceID: UInt64, paymentVault: @FungibleToken.Vault): @NonFungibleToken.NFT {

            let listing = self.borrowListing(listingResourceID: listingResourceID)
                ?? panic("No listing with that ID in Storefront")

            let hasNFT = listing.hasNFT()

            if !hasNFT {
                // Listing won't be removed as the panic causes a rollback.
                self.removeListing(listingResourceID: listingResourceID)
                panic("NFT has already been sold.")
            }

            let item <- listing.purchase(
                paymentVault: <-paymentVault
            )

            self.cleanup()

            return <- item
        }

        // borrowSaleItem
        // Returns a read-only view of the SaleItem for the given listingID if it is contained by this collection.
        //
        pub fun borrowListing(listingResourceID: UInt64): &Listing{ListingPublic}? {
            if self.listings[listingResourceID] != nil {
                return &self.listings[listingResourceID] as! &Listing{ListingPublic}
            } else {
                return nil
            }
        }

        pub fun borrowManagerListing(listingResourceID: UInt64): &Listing? {
            if self.listings[listingResourceID] != nil {
                return &self.listings[listingResourceID] as! &Listing
            } else {
                return nil
            }
        }

        destroy() {
            pre {
                EMMarket.marketStatus != MarketStatus.disabled : "Marketplace has been disabled.  Please try again later."
            }

            destroy self.listings
        }
    }

    pub resource EMMarketAdmin {
        pub fun addTokenCollectionPlatform(
            _ platform: TokenCollectionPlatform
        ): Type {
            let nftType = platform.nftType
            EMMarket.tokenCollectionPlatforms[nftType.identifier] = platform

            emit TokenCollectionPlatformAdded(nftType: nftType)

            return nftType
        }

        pub fun addMarketFeeReceiver(
            type: Type,
            receiver: Capability<&{FungibleToken.Receiver}>
        ): Type {

            pre {
                receiver.borrow() != nil:
                    "Could not add market fee payment token receiver: receiver capability is invalid."
            }
            EMMarket.marketSaleCut.receivers[type.identifier] = receiver

            emit MarketFeeReceiverAdded(type: type)

            return type
        }

        
        pub fun listMarketFeeReceivers(): {String: Capability<&{FungibleToken.Receiver}>} {
            return EMMarket.marketSaleCut.receivers
        }

        pub fun removeTokenCollectionPlatform(nftType: Type): Type {
            pre {
                EMMarket.tokenCollectionPlatforms[nftType.identifier] != nil : "Could not remove token platform: collection type does not exist."
            }

            
            let platform = EMMarket.tokenCollectionPlatforms.remove(key: nftType.identifier)!

            emit TokenCollectionPlatformRemoved(nftType: nftType)

            return nftType
        }

        pub fun removeMarketFeeReceiver(type: Type): Type {
            pre {
                EMMarket.marketSaleCut.receivers[type.identifier] != nil : "Could not remove payment token: payment token type does not exist."
            }

            let platform = EMMarket.marketSaleCut.receivers.remove(key: type.identifier)!

            emit MarketFeeReceiverRemoved(type: type)

            return type
        }

        pub fun updateTokenCollectionPlatform(_ platform: TokenCollectionPlatform): Type {
            pre {
                EMMarket.tokenCollectionPlatforms[platform.nftType.identifier] != nil: "Could not update token platform: platform id does not exist."
            }

            let nftType = platform.nftType
            EMMarket.tokenCollectionPlatforms[nftType.identifier] = platform

            emit TokenCollectionPlatformUpdated(nftType: nftType)

            return nftType
        }

        pub fun updateMarketFee(_ marketSaleCut: MarketSaleCut) {
            for receiverKey in EMMarket.marketSaleCut.receivers.keys {
                marketSaleCut.receivers[receiverKey] = EMMarket.marketSaleCut.receivers[receiverKey]
            }
            EMMarket.marketSaleCut = marketSaleCut

            emit MarketFeeChanged()
        }

        pub fun updateMarketStatus(_ marketStatus: UInt8) {
            EMMarket.marketStatus = MarketStatus(rawValue: marketStatus)!

            emit MarketStatusChanged(marketStatus: marketStatus)
        }

        pub fun createNewAdmin(): @EMMarketAdmin {
            return <-create EMMarketAdmin()
        }
    }


    // A struct containing a Listing's data.
    //
    pub struct ListingDetails {
        // Whether this listing has been purchased or not.
        pub var purchased: Bool
        // The Type of the NonFungibleToken.NFT that is being listed.
        pub let nftType: Type
        // The ID of the NFT within that type.
        pub let nftID: UInt64
        // The Type of the FungibleToken that payments must be made in.
        pub let salePaymentVaultType: Type
        // The amount that must be paid in the specified FungibleToken.
        pub var salePrice: UFix64
        // This specifies the division of payment between recipients.
        pub var saleCuts: [ListingSaleCut]

        // Irreversibly set this listing as purchased.
        //
        access(contract) fun setToPurchased() {
            self.purchased = true
        }

        // Update the sale cuts for this listing
        //
        access(contract) fun updateSaleCuts(_ saleCuts: [ListingSaleCut]) {
            // Store the cuts
            assert(saleCuts.length > 0, message: "Listing must have at least one payment cut recipient")
            self.saleCuts = saleCuts

            // Calculate the total price from the cuts
            var salePrice = 0.0
            // Perform initial check on capabilities, and calculate sale price from cut amounts.
            for cut in self.saleCuts {
                // Make sure we can borrow the receiver.
                // We will check this again when the token is sold.
                cut.receiver.borrow()
                    ?? panic("Cannot borrow receiver")
                // Add the cut amount to the total price
                salePrice = salePrice + cut.amount
            }
            assert(salePrice > 0.0, message: "Listing must have non-zero price")

            // Store the calculated sale price
            self.salePrice = salePrice
        }

        init (
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
            saleCuts: [ListingSaleCut],
        ) {
            self.purchased = false
            self.nftType = nftType
            self.nftID = nftID
            self.salePaymentVaultType = salePaymentVaultType

            // Store the cuts
            assert(saleCuts.length > 0, message: "Listing must have at least one payment cut recipient")
            self.saleCuts = saleCuts

            // Calculate the total price from the cuts
            var salePrice = 0.0
            // Perform initial check on capabilities, and calculate sale price from cut amounts.
            for cut in self.saleCuts {
                // Make sure we can borrow the receiver.
                // We will check this again when the token is sold.
                cut.receiver.borrow()
                    ?? panic("Cannot borrow receiver")
                // Add the cut amount to the total price
                salePrice = salePrice + cut.amount
            }
            assert(salePrice > 0.0, message: "Listing must have non-zero price")

            // Store the calculated sale price
            self.salePrice = salePrice
        }
    }


    // An interface providing a useful public interface to a Listing.
    //
    pub resource interface ListingPublic {
        // borrowNFT
        // This will assert in the same way as the NFT standard borrowNFT()
        // if the NFT is absent, for example if it has been sold via another listing.
        //
        pub fun borrowNFT(): &NonFungibleToken.NFT

        pub fun hasNFT(): Bool

        // purchase
        // Purchase the listing, buying the token.
        // This pays the beneficiaries and returns the token to the buyer.
        //
        pub fun purchase(paymentVault: @FungibleToken.Vault): @NonFungibleToken.NFT

        // getDetails
        //
        pub fun getDetails(): ListingDetails
    }


    // A resource that allows an NFT to be sold for an amount of a given FungibleToken,
    // and for the proceeds of that sale to be split between several recipients.
    // 
    pub resource Listing: ListingPublic {
        priv let details: ListingDetails

        // A capability allowing this resource to withdraw the NFT with the given ID from its collection.
        // This capability allows the resource to withdraw *any* NFT, so you should be careful when giving
        // such a capability to a resource and always check its code to make sure it will use it in the
        // way that it claims.
        access(contract) let nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>


        init (
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
            saleCuts: [ListingSaleCut],
        ) {
            pre {
                EMMarket.tokenCollectionPlatforms[nftType.identifier] != nil : "Platform with the provided collection type identifier is not registered."
            }
            
            self.details = ListingDetails(
                nftType: nftType,
                nftID: nftID,
                salePaymentVaultType: salePaymentVaultType,
                saleCuts: saleCuts
            )

            self.nftProviderCapability = nftProviderCapability

            // Check that the provider contains the NFT.
            // We will check it again when the token is sold.
            // We cannot move this into a function because initializers cannot call member functions.
            let provider = self.nftProviderCapability.borrow()
            assert(provider != nil, message: "cannot borrow nftProviderCapability")

            // This will precondition assert if the token is not available.
            let nft = provider!.borrowNFT(id: self.details.nftID)
            assert(nft.isInstance(self.details.nftType), message: "token is not of specified type")
            assert(nft.id == self.details.nftID, message: "token does not have specified ID")
        }

        // borrowNFT
        // This will assert in the same way as the NFT standard borrowNFT()
        // if the NFT is absent, for example if it has been sold via another listing.
        //
        pub fun borrowNFT(): &NonFungibleToken.NFT {
            let ref = self.nftProviderCapability.borrow()!.borrowNFT(id: self.getDetails().nftID)
            assert(ref.isInstance(self.getDetails().nftType), message: "token has wrong type")
            assert(ref.id == self.getDetails().nftID, message: "token has wrong ID")
            return ref as &NonFungibleToken.NFT
        }

        pub fun hasNFT(): Bool {
            let ids = self.nftProviderCapability.borrow()!.getIDs()
            return ids.contains(self.getDetails().nftID);
        }

        // getDetails
        // Get the details of the current state of the Listing as a struct.
        // This avoids having more public variables and getter methods for them, and plays
        // nicely with scripts (which cannot return resources).
        //
        pub fun getDetails(): ListingDetails {
            return self.details
        }

        pub fun updateSaleCuts(_ saleCuts: [ListingSaleCut]) {
            self.details.updateSaleCuts(saleCuts)

            let saleCutAmounts: [UFix64] = []
            for saleCut in self.details.saleCuts {
                saleCutAmounts.append(saleCut.amount)
            }
            emit ListingSaleCutsChanged(
                seller: self.owner?.address!,
                listingResourceID: self.uuid,
                nftType: self.details.nftType,
                nftID: self.details.nftID,
                saleCuts: saleCutAmounts,
                salePrice: self.details.salePrice
            )
        }

        // purchase
        // Purchase the listing, buying the token.
        // This pays the beneficiaries and returns the token to the buyer.
        //
        pub fun purchase(paymentVault: @FungibleToken.Vault): @NonFungibleToken.NFT {
            pre {
                EMMarket.marketStatus != MarketStatus.disabled : "Marketplace has been disabled.  Please try again later."
                EMMarket.marketStatus != MarketStatus.restrictPurchasing : "Marketplace is currently not allowing purchase transactions.  Please try again later."
                EMMarket.tokenCollectionPlatforms[self.details.nftType.identifier] != nil : "Platform with the provided collection type identifier is not registered."
                self.details.purchased == false: "listing has already been purchased"
                paymentVault.isInstance(self.details.salePaymentVaultType): "payment vault is not requested fungible token"
                paymentVault.balance == self.details.salePrice: "payment vault does not contain requested price"
            }

            // Make sure the listing cannot be purchased again.
            self.details.setToPurchased()

            // Fetch the token to return to the purchaser.
            let nft <-self.nftProviderCapability.borrow()!.withdraw(withdrawID: self.details.nftID)
            // Neither receivers nor providers are trustworthy, they must implement the correct
            // interface but beyond complying with its pre/post conditions they are not gauranteed
            // to implement the functionality behind the interface in any given way.
            // Therefore we cannot trust the Collection resource behind the interface,
            // and we must check the NFT resource it gives us to make sure that it is the correct one.
            assert(nft.isInstance(self.details.nftType), message: "withdrawn NFT is not of specified type")
            assert(nft.id == self.details.nftID, message: "withdrawn NFT does not have specified ID")

            var tokenCollectionPlatform = EMMarket.tokenCollectionPlatforms[self.details.nftType.identifier]!

            let marketSaleCutReceiver = EMMarket.marketSaleCut.receivers[self.details.salePaymentVaultType.identifier]
            let marketSaleCuts = marketSaleCutReceiver != nil ? [EMMarket.getMarketSaleCut(paymentType: self.details.salePaymentVaultType)!] : []
            let saleCuts = marketSaleCuts.concat(tokenCollectionPlatform.saleCuts)

            // Pay each platform beneficiary their amount of the payment.
            for cut in saleCuts {
                if let receiver = cut.receiver.borrow() {
                    let cutAmount = cut.calculateCut(self.details.salePrice)
                    let paymentCut <- paymentVault.withdraw(amount: cutAmount)
                    receiver.deposit(from: <-paymentCut)
                }
            }


            // Rather than aborting the transaction if any receiver is absent when we try to pay it,
            // we send the cut to the first valid receiver.
            // The first receiver should therefore either be the seller, or an agreed recipient for
            // any unpaid cuts.
            var residualReceiver: &{FungibleToken.Receiver}? = nil

            // Pay each listing beneficiary their amount of the payment.
            for cut in self.details.saleCuts {
                if let receiver = cut.receiver.borrow() {
                    var cutAmount = cut.amount
                    if cutAmount > paymentVault.balance {
                        cutAmount = paymentVault.balance
                    }
                    let paymentCut <- paymentVault.withdraw(amount: cutAmount)
                    receiver.deposit(from: <-paymentCut)

                    if (residualReceiver == nil) {
                        residualReceiver = receiver
                    }
                }
            }

            assert(residualReceiver != nil, message: "No valid payment receivers")

            // At this point, if all recievers were active and availabile, then the payment Vault will have
            // zero tokens left, and this will functionally be a no-op that consumes the empty vault
            residualReceiver!.deposit(from: <-paymentVault)

            

            // If the listing is purchased, we regard it as completed here.
            // Otherwise we regard it as removed in the destructor.
            emit ListingCompleted(
                seller: self.owner?.address!,
                listingResourceID: self.uuid,
                nftType: self.details.nftType,
                nftID: self.details.nftID,
                price: self.details.salePrice
            )

            return <-nft
        }

        // destructor
        //
        destroy () {
            // If the listing has not been purchased, we regard it as completed here.
            // Otherwise we regard it as completed in purchase().
            // This is because we destroy the listing in Storefront.removeListing()
            // or Storefront.cleanup() .
            // If we change this destructor, revisit those functions.
            if !self.details.purchased {
                emit ListingRemoved(
                    seller: self.owner?.address!,
                    listingResourceID: self.uuid,
                    nftType: self.details.nftType,
                    nftID: self.details.nftID
                )
            }
        }

    }

    // A struct representing a recipient that must be sent a certain amount
    // of the payment when a token is sold.
    pub struct ListingSaleCut {
        // The display name of the receiver
        pub let receiverDisplayName: String
        // The description of the intended purpose of this cut.
        pub let description: String

        // The receiver for the payment.
        pub let receiver: Capability<&{FungibleToken.Receiver}>
        pub let amount: UFix64

        init(
            receiverDisplayName: String, 
            description: String, 
            receiver: Capability<&{FungibleToken.Receiver}>, 
            amount: UFix64
        ) {
            pre {
                receiverDisplayName.length > 0 : "Could not initialize listing sale cut: receiverDisplayName is required."
                description.length > 0 : "Could not initialize listing sale cut: description is required."
                receiver.borrow() != nil:
                    "Could not initialize listing sale cut: receiver capability is invalid."
                amount > 0.0 : "Could not initialize listing sale cut: amount must be more than 0."
            }
            self.receiverDisplayName = receiverDisplayName
            self.description = description
            self.receiver = receiver
            self.amount = amount
        }
    }

    // A struct representing a recipient that must be sent a certain amount
    // of the payment when a token is sold.
    pub struct MarketSaleCut {
        // The receiver for the payment.
        pub let receivers: {String: Capability<&{FungibleToken.Receiver}>}

        // The amount of the payment FungibleToken that will be paid to the receiver. Refer to the amountType for how this is calculated
        pub let amountValue: UFix64
        // The way the amount value is used in order to determine the final sale cut amount.
        // In the case of fixed, an exact currency amount is used
        pub let amountType: SaleCutType

        // The minimum amount limit
        pub let minAmount: UFix64?
        // The maximum amount limit
        pub let maxAmount: UFix64?

        init(
            amountValue: UFix64, 
            amountType: SaleCutType, 
            minAmount: UFix64?, 
            maxAmount: UFix64?
        ) {
            pre {
                amountValue > 0.0 && (amountType != SaleCutType.percent || amountValue < 1.0) : "Could not initialize sale cut: valid amount value and type is required."
                minAmount == nil || maxAmount == nil || minAmount! < maxAmount! : "Could not initialize sale cut: minAmount should be less than the maxAmount."
            }
            self.receivers = {}
            self.amountValue = amountValue
            self.amountType = amountType
            self.minAmount = minAmount
            self.maxAmount = maxAmount
        }
    }

    // A struct representing a recipient that must be sent a certain amount
    // of the payment when a token is sold.
    pub struct SaleCut {
        // The display name of the receiver
        pub let receiverDisplayName: String
        // The description of the intended purpose of this cut.
        pub let description: String

        // The receiver for the payment.
        pub let receiver: Capability<&{FungibleToken.Receiver}>

        // The amount of the payment FungibleToken that will be paid to the receiver. Refer to the amountType for how this is calculated
        pub let amountValue: UFix64
        // The way the amount value is used in order to determine the final sale cut amount.
        // In the case of fixed, an exact currency amount is used
        pub let amountType: SaleCutType

        // The minimum amount limit
        pub let minAmount: UFix64?
        // The maximum amount limit
        pub let maxAmount: UFix64?

        init(
            receiverDisplayName: String, 
            description: String, 
            receiver: Capability<&{FungibleToken.Receiver}>, 
            amountValue: UFix64, 
            amountType: SaleCutType, 
            minAmount: UFix64?, 
            maxAmount: UFix64?
        ) {
            pre {
                receiverDisplayName.length > 0 : "Could not initialize sale cut: receiverDisplayName is required."
                description.length > 0 : "Could not initialize sale cut: description is required."
                receiver.borrow() != nil:
                    "Could not initialize sale cut: receiver capability is invalid."
                amountValue > 0.0 && (amountType != SaleCutType.percent || amountValue < 1.0) : "Could not initialize sale cut: valid amount value and type is required."
                minAmount == nil || maxAmount == nil || minAmount! < maxAmount! : "Could not initialize sale cut: minAmount should be less than the maxAmount."
            }
            self.receiverDisplayName = receiverDisplayName
            self.description = description
            self.receiver = receiver
            self.amountValue = amountValue
            self.amountType = amountType
            self.minAmount = minAmount
            self.maxAmount = maxAmount
        }

        pub fun calculateCut(_ amount: UFix64): UFix64 {
            var cut = 0.0
            if self.amountType == SaleCutType.fixed {
                cut = self.amountValue 
            } else {
                cut = amount * self.amountValue
            }
            if self.minAmount != nil && cut < self.minAmount! {
                cut = self.minAmount!
            }
            if self.maxAmount != nil && cut > self.maxAmount! {
                cut = self.maxAmount!
            }
            return cut
        }
    }

    pub enum SaleCutType: UInt8 {
        pub case fixed
        pub case percent
    }

    pub enum MarketStatus: UInt8 {
        pub case enabled
        pub case restrictListing
        pub case restrictPurchasing
        pub case restrictDelisting
        pub case disabled
    }

    pub struct TokenCollectionPlatform {
        pub let name: String
        pub let nftType: Type
        pub let storagePath: StoragePath
        pub let publicPath: PublicPath
        pub let contractAddress: Address
        pub let saleCuts: [SaleCut]

        init(
            name: String, 
            nftType: Type, 
            storagePath: StoragePath, 
            publicPath: PublicPath, 
            contractAddress: Address, 
            saleCuts: [SaleCut]
        ) {
            pre {
                name.length > 0 : "Could not initialize token platform: name is required."
            }
            self.name = name
            self.nftType = nftType
            self.storagePath = storagePath
            self.publicPath = publicPath
            self.contractAddress = contractAddress
            self.saleCuts = saleCuts
        }

        pub fun calculateCuts(_ price: UFix64): {String: UFix64} {
            let cutMap: {String: UFix64} = {}

            for saleCut in self.saleCuts {
                cutMap[saleCut.receiverDisplayName] = saleCut.calculateCut(price)
            }
            return cutMap
        }
    }

    pub fun listTokenCollectionPlatforms(): {String: TokenCollectionPlatform} {
        return self.tokenCollectionPlatforms
    }

    pub fun getTokenCollectionPlatformByNftType(nftType: Type): TokenCollectionPlatform? {
        return self.tokenCollectionPlatforms[nftType.identifier]
    }

    pub fun getMarketSaleCut(paymentType: Type): SaleCut? {
        let marketSaleCut = EMMarket.marketSaleCut
        let marketSaleCutReceiver = marketSaleCut.receivers[paymentType.identifier]
        if  marketSaleCutReceiver != nil {
            return SaleCut(
                receiverDisplayName: "Evaluate.Market", 
                description: (marketSaleCut.amountValue * 100.0).toString().concat("% fee charged by Evaluate.Market for providing the marketplace services."),
                receiver: marketSaleCutReceiver!,
                amountValue: marketSaleCut.amountValue, 
                amountType: marketSaleCut.amountType,
                minAmount: marketSaleCut.minAmount,
                maxAmount: marketSaleCut.maxAmount
            )
        }
        return nil
    }

    pub fun createTokenStorefront(): @EMStorefront {
        return <- create EMStorefront()
    }

    init() {
        let adminAccount = self.account
        self.tokenCollectionPlatforms = {}
        self.marketStatus = MarketStatus.enabled
        self.marketSaleCut = MarketSaleCut(
            amountValue: 0.025, 
            amountType: SaleCutType.percent,
            minAmount: nil,
            maxAmount: nil
        )
        self.EMStorefrontStoragePath = /storage/EMStorefront
        self.EMStorefrontPublicPath = /public/EMStorefrontPublic
        self.EMMarketAdminStoragePath = /storage/EMMarketAdmin
        adminAccount.save<@EMMarketAdmin>(<- create EMMarketAdmin(), to: self.EMMarketAdminStoragePath)
        emit EMMarketInitialized()
    }
}