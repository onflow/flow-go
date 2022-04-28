import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import ListenNFT from 0xf448b8731913195c
import ListenUSD from 0xf448b8731913195c

// ListenMarketplace
//
// Based on https://github.com/onflow/nft-storefront
//
// Added support for listing multiple NFTs of same type.... 
// Currently hardcoded to use ListenNFT (requires createEmptyCollection)
// Can store empty collection created in transaction in Listing
// This would be more flexible and allow use of other tokens in the future...
// Still they would be required to be of same Type
//
// Each account that wants to list NFTs for sale installs a ListenStorefront,
// and lists individual sales within that ListenStorefront as Listings.
// There is one ListenStorefront per account
//
// Each NFT may be listed in one or more Listings, the validity of each
// Listing can easily be checked.
// 
// Purchasers can watch for Listing events and check the NFT 
// IDs to see if they wish to buy the listed items.
// Marketplaces and other aggregators can watch for Listing events
// and list items of interest.
//
pub contract ListenMarketplace {
    // ListenMarketplaceInitialized
    // This contract has been deployed.
    // Event consumers can now expect events from this contract.
    //
    pub event ListenMarketplaceInitialized()

    // Set to account.self.address on deployment..... consider using different address and/or making updatable by admin 
    pub let listenRoyaltyPaymentAddress: Address

    // ListingAvailable
    // A listing has been created and added to a ListenStorefront resource.
    // The Address values here are valid when the event is emitted, but
    // the state of the accounts they refer to may be changed outside of the
    // ListenStorefront workflow, so be careful to check when using them.
    //
    pub event ListingAvailable(
        listenStorefrontAddress: Address,
        listingResourceID: UInt64,
        nftType: Type,
        nftIDs: [UInt64],
        ftVaultType: Type,
        price: UFix64
    )



    // ListingCompleted
    // The listing has been resolved. It has either been purchased, or removed and destroyed.
    //
    pub event ListingCompleted(listingResourceID: UInt64, listenStorefrontResourceID: UInt64, purchased: Bool)

    // ListenStorefrontStoragePath
    // The location in storage that a ListenStorefront resource should be located.
    pub let ListenStorefrontStoragePath: StoragePath

    // ListenStorefrontPublicPath
    // The public location for a ListenStorefront link.
    pub let ListenStorefrontPublicPath: PublicPath


    // SaleCut
    // A struct representing a recipient that must be sent a certain amount
    // of the payment when a token is sold.
    //
    pub struct SaleCut {
        // The receiver for the payment.
        // Note that we do not store an address to find the Vault that this represents,
        // as the link or resource that we fetch in this way may be manipulated,
        // so to find the address that a cut goes to you must get this struct and then
        // call receiver.borrow()!.owner.address on it.
        // This can be done efficiently in a script.
        pub let receiver: Capability<&{FungibleToken.Receiver}>

        // The amount of the payment FungibleToken that will be paid to the receiver.
        pub let amount: UFix64
        
        // initializer
        //
        init(receiver: Capability<&{FungibleToken.Receiver}>, amount: UFix64) {
            self.receiver = receiver
            self.amount = amount
        }
    }


    // ListingDetails
    // A struct containing a Listing's data.
    //
    pub struct ListingDetails {
        // The ListenStorefront that the Listing is stored in.
        // Note that this resource cannot be moved to a different ListenStorefront,
        // so this is OK. If we ever make it so that it *can* be moved,
        // this should be revisited.
        pub var ListenStorefrontID: UInt64
        // Whether this listing has been purchased or not.
        pub var purchased: Bool
        // The Type of the NonFungibleToken.NFT that is being listed.
        pub let nftType: Type
        // The ID of the NFT within that type.
        pub let nftIDs: [UInt64]
        // The Type of the FungibleToken that payments must be made in.
        pub let salePaymentVaultType: Type
        // The amount that must be paid in the specified FungibleToken.
        pub let salePrice: UFix64
        // This specifies the division of payment between recipients.
        pub let saleCuts: [SaleCut]

        // setToPurchased
        // Irreversibly set this listing as purchased.
        //
        access(contract) fun setToPurchased() {
            self.purchased = true
        }

        // initializer
        //
        init (
            nftType: Type,
            nftIDs: [UInt64],
            salePaymentVaultType: Type,
            saleCuts: [SaleCut],
            listenStorefrontID: UInt64
        ) {
            self.ListenStorefrontID = listenStorefrontID
            self.purchased = false
            self.nftType = nftType
            self.nftIDs = nftIDs
            self.salePaymentVaultType = salePaymentVaultType

            // Store the cuts
            assert(saleCuts.length > 0, message: "Listing must have at least one payment cut recipient")
            self.saleCuts = saleCuts

           
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


    // ListingPublic
    // An interface providing a useful public interface to a Listing.
    //
    pub resource interface ListingPublic {
        // borrowNFT
        // This will assert in the same way as the NFT standard borrowNFT()
        // if the NFT is absent, for example if it has been sold via another listing.
        //
        pub fun borrowNFTs(): [&NonFungibleToken.NFT]

        // purchase
        // Purchase the listing, buying the token.
        // This pays the beneficiaries and returns the token to the buyer.
        //
        pub fun purchase(payment: @FungibleToken.Vault): @NonFungibleToken.Collection

        // getDetails
        //
        pub fun getDetails(): ListingDetails
    }


    // Listing
    // A resource that allows an NFT to be sold for an amount of a given FungibleToken,
    // and for the proceeds of that sale to be split between several recipients.
    // 
    pub resource Listing: ListingPublic {
        // The simple (non-Capability, non-complex) details of the sale
        access(self) let details: ListingDetails

        // A capability allowing this resource to withdraw the NFT with the given ID from its collection.
        // This capability allows the resource to withdraw *any* NFT, so you should be careful when giving
        // such a capability to a resource and always check its code to make sure it will use it in the
        // way that it claims.
        access(contract) let nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>

        // borrowNFT
        // This will assert in the same way as the NFT standard borrowNFT()
        // if the NFT is absent, for example if it has been sold via another listing.
        //
        pub fun borrowNFTs(): [&NonFungibleToken.NFT] {
            let refs: [&NonFungibleToken.NFT] = []
            for id in self.getDetails().nftIDs {
                let ref = self.nftProviderCapability.borrow()!.borrowNFT(id: id)
                assert(ref.isInstance(self.getDetails().nftType), message: "token has wrong type")
                assert(ref.id == id, message: "token has wrong ID")
            }
            return refs // as &NonFungibleToken.NFT
        }

        // getDetails
        // Get the details of the current state of the Listing as a struct.
        // This avoids having more public variables and getter methods for them, and plays
        // nicely with scripts (which cannot return resources).
        //
        pub fun getDetails(): ListingDetails {
            return self.details
        }

        // purchase
        // Purchase the listing, buying the token.
        // This pays the beneficiaries and returns the token to the buyer.
        //
        pub fun purchase(payment: @FungibleToken.Vault): @NonFungibleToken.Collection {
            pre {
                self.details.purchased == false: "listing has already been purchased"
                payment.isInstance(self.details.salePaymentVaultType): "payment vault is not requested fungible token"
                payment.balance == self.details.salePrice: "payment vault does not contain requested price"
            }

            // Make sure the listing cannot be purchased again.
            self.details.setToPurchased()

            // Fetch the token to return to the purchaser.
            let collection <- ListenNFT.createEmptyCollection()
            for id in self.details.nftIDs {
                let nft <-self.nftProviderCapability.borrow()!.withdraw(withdrawID: id)

                // Neither receivers nor providers are trustworthy, they must implement the correct
                // interface but beyond complying with its pre/post conditions they are not gauranteed
                // to implement the functionality behind the interface in any given way.
                // Therefore we cannot trust the Collection resource behind the interface,
                // and we must check the NFT resource it gives us to make sure that it is the correct one.
                assert(nft.isInstance(self.details.nftType), message: "withdrawn NFT is not of specified type")
                assert(nft.id == id, message: "withdrawn NFT does not have specified ID")
                collection.deposit( token: <- nft)
            }
           
            // Rather than aborting the transaction if any receiver is absent when we try to pay it,
            // we send the cut to the first valid receiver.
            // The first receiver should therefore either be the seller, or an agreed recipient for
            // any unpaid cuts.
            var residualReceiver: &{FungibleToken.Receiver}? = nil

            // Pay each beneficiary their amount of the payment.
            for cut in self.details.saleCuts {
                if let receiver = cut.receiver.borrow() {
                   let paymentCut <- payment.withdraw(amount: cut.amount)
                    receiver.deposit(from: <-paymentCut)
                    if (residualReceiver == nil) {
                        residualReceiver = receiver
                    }
                }
            }

            assert(residualReceiver != nil, message: "No valid payment receivers")

            // At this point, if all recievers were active and availabile, then the payment Vault will have
            // zero tokens left, and this will functionally be a no-op that consumes the empty vault
            residualReceiver!.deposit(from: <-payment)

            // If the listing is purchased, we regard it as completed here.
            // Otherwise we regard it as completed in the destructor.
            emit ListingCompleted(
                listingResourceID: self.uuid,
                listenStorefrontResourceID: self.details.ListenStorefrontID,
                purchased: self.details.purchased
            )

            return <-collection
        }

        // destructor
        //
        destroy () {
            // If the listing has not been purchased, we regard it as completed here.
            // Otherwise we regard it as completed in purchase().
            // This is because we destroy the listing in ListenStorefront.removeListing()
            // or ListenStorefront.cleanup() .
            // If we change this destructor, revisit those functions.
            if !self.details.purchased {
                emit ListingCompleted(
                    listingResourceID: self.uuid,
                    listenStorefrontResourceID: self.details.ListenStorefrontID,
                    purchased: self.details.purchased
                )
            }
        }

        // initializer
        //
        init (
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftIDs: [UInt64],
            salePaymentVaultType: Type,
            saleCuts: [SaleCut],
            listenStorefrontID: UInt64
        ) {
            // Store the sale information
            self.details = ListingDetails(
                nftType: nftType,
                nftIDs: nftIDs,
                salePaymentVaultType: salePaymentVaultType,
                saleCuts: saleCuts,
                listenStorefrontID: listenStorefrontID
            )

            // Store the NFT provider
            self.nftProviderCapability = nftProviderCapability

            // Check that the provider contains the NFT.
            // We will check it again when the token is sold.
            // We cannot move this into a function because initializers cannot call member functions.
            let provider = self.nftProviderCapability.borrow()
            assert(provider != nil, message: "cannot borrow nftProviderCapability")
            // This will precondition assert if the token is not available.
            for id in self.details.nftIDs {
                let nft = provider!.borrowNFT(id: id)
                assert(nftProviderCapability.address == nft.owner!.address, message: "NFT Provider address must match NFT owner's address.")
                //assert(nft.isInstance(Type<@ListenNFT.NFT>()), message: "token is not a ListenNFT!")
                assert(nft.isInstance(self.details.nftType), message: "token is not of specified type")
                assert(nft.id == id, message: "token does not have specified ID")
            }
        }
    }

    // ListenStorefrontManager
    // An interface for adding and removing Listings within a ListenStorefront,
    // intended for use by the ListenStorefront's own
    //
    pub resource interface ListenStorefrontManager {
        // createListing
        // Allows the ListenStorefront owner to create and insert Listings.
        //
        pub fun createListing(
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftIDs: [UInt64],
            salePaymentVaultType: Type,
            ftReceiverCap: Capability<&{FungibleToken.Receiver}>,
            salePrice: UFix64
            // saleCuts: [SaleCut]
        ): UInt64
        // removeListing
        // Allows the ListenStorefront owner to remove any sale listing, acepted or not.
        //
        pub fun removeListing(listingResourceID: UInt64)
    }

    // ListenStorefrontPublic
    // An interface to allow listing and borrowing Listings, and purchasing items via Listings
    // in a ListenStorefront.
    //
    pub resource interface ListenStorefrontPublic {
        pub fun getListingIDs(): [UInt64]
        pub fun borrowListing(listingResourceID: UInt64): &Listing{ListingPublic}?
        pub fun cleanup(listingResourceID: UInt64)
   }

    // ListenStorefront
    // A resource that allows its owner to manage a list of Listings, and anyone to interact with them
    // in order to query their details and purchase the NFTs that they represent.
    //
    pub resource ListenStorefront : ListenStorefrontManager, ListenStorefrontPublic {
        // The dictionary of Listing uuids to Listing resources.
        access(self) var listings: @{UInt64: Listing}

        // insert
        // Create and publish a Listing for an NFT.
        //
        pub fun createListing(
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftIDs: [UInt64],
            salePaymentVaultType: Type,
            ftReceiverCap: Capability<&{FungibleToken.Receiver}>,
            salePrice: UFix64
            // saleCuts: [SaleCut]
         ): UInt64 {
            let royaltyReceiverCap = getAccount(ListenMarketplace.listenRoyaltyPaymentAddress)
                                    .getCapability<&{FungibleToken.Receiver}>(ListenUSD.ReceiverPublicPath)
            assert(royaltyReceiverCap.borrow() != nil, message: "Missing or mis-typed ListenUSD receiver (royalty account)")
            
            // 5% royalty fee 
            let royalty = ListenMarketplace.SaleCut(
                receiver: royaltyReceiverCap, // as Capability<&{FungibleToken.Receiver}>,
                amount: salePrice * 0.05
            )
            // With a 5% seller fee, the buyer would pay $100 for the NFT and $5 would go to Listen, meaning the seller would receive $95. 
            // With a 5% buyer fee,  the buyer would pay $105 for the NFT, $5 would go to Listen, and $100 would go to the seller.
            let saleCut = ListenMarketplace.SaleCut(
                receiver: ftReceiverCap, //  as Capability<&{FungibleToken.Receiver}>,
                amount: salePrice - royalty.amount // -royalty.amount = levy on seller  omitted equivalent to levy on buyer
            )

            let listing <- create Listing(
                nftProviderCapability: nftProviderCapability,
                nftType: nftType,
                nftIDs: nftIDs,
                salePaymentVaultType: salePaymentVaultType,
                saleCuts: [royalty, saleCut],
                listenStorefrontID: self.uuid
            )

            let listingResourceID = listing.uuid
            let listingPrice = listing.getDetails().salePrice

            // Add the new listing to the dictionary.
            let oldListing <- self.listings[listingResourceID] <- listing
            // Note that oldListing will always be nil, but we have to handle it.
            destroy oldListing

            emit ListingAvailable(
                listenStorefrontAddress: self.owner?.address!,
                listingResourceID: listingResourceID,
                nftType: nftType,
                nftIDs: nftIDs,
                ftVaultType: salePaymentVaultType,
                price: listingPrice
            )

            return listingResourceID
        }

        // removeListing
        // Remove a Listing that has not yet been purchased from the collection and destroy it.
        //
        pub fun removeListing(listingResourceID: UInt64) {
            let listing <- self.listings.remove(key: listingResourceID)
                ?? panic("missing Listing")
    
            // This will emit a ListingCompleted event.
            destroy listing
        }

        // getListingIDs
        // Returns an array of the Listing resource IDs that are in the collection
        //
        pub fun getListingIDs(): [UInt64] {
            return self.listings.keys
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

        // cleanup
        // Remove an listing *if* it has been purchased.
        // Anyone can call, but at present it only benefits the account owner to do so.
        // Kind purchasers can however call it if they like.
        //
        pub fun cleanup(listingResourceID: UInt64) {
            pre {
                self.listings[listingResourceID] != nil: "could not find listing with given id"
            }

            let listing <- self.listings.remove(key: listingResourceID)!
            assert(listing.getDetails().purchased == true, message: "listing is not purchased, only admin can remove")
            destroy listing
        }

        // destructor
        //
        destroy () {
            destroy self.listings
        }

        // constructor
        //
        init () {
            self.listings <- {}
        }
    }

    // createListenStorefront
    // Make creating a ListenStorefront publicly accessible.
    //
    pub fun createListenStorefront(): @ListenStorefront {
        return <-create ListenStorefront()
    }

    init () {
        self.ListenStorefrontStoragePath = /storage/ListenStorefront
        self.ListenStorefrontPublicPath = /public/ListenStorefront

        self.listenRoyaltyPaymentAddress = self.account.address

        // Added setup here to ensure Royalty vault exists. 
        // Setup ListenUSD Vault for receiving royalties

        let listenUSD_Vault = self.account.borrow<&ListenUSD.Vault>(from: ListenUSD.VaultStoragePath)

        if listenUSD_Vault ==  nil {
            self.account.save(<-ListenUSD.createEmptyVault(), to: ListenUSD.VaultStoragePath)
        }

        self.account.link<&{FungibleToken.Receiver}>(
            ListenUSD.ReceiverPublicPath,
            target: ListenUSD.VaultStoragePath
        )
        self.account.link<&{FungibleToken.Balance}>(
            ListenUSD.BalancePublicPath,
            target: ListenUSD.VaultStoragePath
        )

        emit ListenMarketplaceInitialized()
    }
}
 