import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448;

// GeniaceMarketplace
//
// A general purpose sale support contract for Flow NonFungibleTokens.
// 
// Each account that wants to offer NFTs for sale installs a Storefront,
// and lists individual sales within that Storefront as SaleOffers.
// There is one Storefront per account, it handles sales of all NFT types
// for that account.
//
// Each SaleOffer can have one or more "cut"s of the sale price that
// goes to one or more addresses. Cuts can be used to pay listing fees
// or other considerations.
// Each NFT may be listed in one or more SaleOffers, the validity of each
// SaleOffer can easily be checked.
// 
// Purchasers can watch for SaleOffer events and check the NFT type and
// ID to see if they wish to buy the offered item.
// Marketplaces and other aggregators can watch for SaleOffer events
// and list items of interest.
//
pub contract GeniaceMarketplace {
    // GeniaceMarketplaceInitialized
    // This contract has been deployed.
    // Event consumers can now expect events from this contract.
    //
    pub event GeniaceMarketplaceInitialized()

    // StorefrontInitialized
    // A Storefront resource has been created.
    // Event consumers can now expect events from this Storefront.
    // Note that we do not specify an address: we cannot and should not.
    // Created resources do not have an owner address, and may be moved
    // after creation in ways we cannot check.
    // SaleOfferAvailable events can be used to determine the address
    // of the owner of the Storefront (...its location) at the time of
    // the offer but only at that precise moment in that precise transaction.
    // If the offerer moves the Storefront while the offer is valid, that
    // is on them.
    //
    pub event StorefrontInitialized(storefrontResourceID: UInt64)

    // StorefrontDestroyed
    // A Storefront has been destroyed.
    // Event consumers can now stop processing events from this Storefront.
    // Note that we do not specify an address.
    //
    pub event StorefrontDestroyed(storefrontResourceID: UInt64)

    // SaleOfferAvailable
    // A sale offer has been created and added to a Storefront resource.
    // The Address values here are valid when the event is emitted, but
    // the state of the accounts they refer to may be changed outside of the
    // GeniaceMarketplace workflow, so be careful to check when using them.
    //
    pub event SaleOfferAvailable(
        storefrontAddress: Address,
        saleOfferResourceID: UInt64,
        nftType: Type,
        nftID: UInt64,
        ftVaultType: Type,
        price: UFix64
    )

    // SaleOfferCompleted
    // The sale offer has been resolved. It has either been accepted, or removed and destroyed.
    //
    pub event SaleOfferCompleted(saleOfferResourceID: UInt64, storefrontResourceID: UInt64, accepted: Bool)

    // StorefrontStoragePath
    // The location in storage that a Storefront resource should be located.
    pub let StorefrontStoragePath: StoragePath

    // StorefrontPublicPath
    // The public location for a Storefront link.
    pub let StorefrontPublicPath: PublicPath


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


    // SaleOfferDetails
    // A struct containing a SaleOffer's data.
    //
    pub struct SaleOfferDetails {
        // The Storefront that the SaleOffer is stored in.
        // Note that this resource cannot be moved to a different Storefront,
        // so this is OK. If we ever make it so that it *can* be moved,
        // this should be revisited.
        pub var storefrontID: UInt64
        // Whether this offer has been accepted or not.
        pub var accepted: Bool
        // The Type of the NonFungibleToken.NFT that is being offered.
        pub let nftType: Type
        // The ID of the NFT within that type.
        pub let nftID: UInt64
        // The Type of the FungibleToken that payments must be made in.
        pub let salePaymentVaultType: Type
        // The amount that must be paid in the specified FungibleToken.
        pub let salePrice: UFix64
        // This specifies the division of payment between recipients.
        pub let saleCuts: [SaleCut]

        // setToAccepted
        // Irreversibly set this offer as accepted.
        //
        access(contract) fun setToAccepted() {
            self.accepted = true
        }

        // initializer
        //
        init (
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
            saleCuts: [SaleCut],
            storefrontID: UInt64
        ) {
            self.storefrontID = storefrontID
            self.accepted = false
            self.nftType = nftType
            self.nftID = nftID
            self.salePaymentVaultType = salePaymentVaultType

            // Store the cuts
            assert(saleCuts.length > 0, message: "SaleOffer must have at least one payment cut recipient")
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
            assert(salePrice > 0.0, message: "SaleOffer must have non-zero price")

            // Store the calculated sale price
            self.salePrice = salePrice
        }
    }


    // SaleOfferPublic
    // An interface providing a useful public interface to a SaleOffer.
    //
    pub resource interface SaleOfferPublic {
        // borrowNFT
        // This will assert in the same way as the NFT standard borrowNFT()
        // if the NFT is absent, for example if it has been sold via another offer.
        //
        pub fun borrowNFT(): &NonFungibleToken.NFT

        // accept
        // Accept the offer, buying the token.
        // This pays the beneficiaries and returns the token to the buyer.
        //
        pub fun accept(payment: @FungibleToken.Vault): @NonFungibleToken.NFT

        // getDetails
        //
        pub fun getDetails(): SaleOfferDetails
    }


    // SaleOffer
    // A resource that allows an NFT to be sold for an amount of a given FungibleToken,
    // and for the proceeds of that sale to be split between several recipients.
    // 
    pub resource SaleOffer: SaleOfferPublic {
        // The simple (non-Capability, non-complex) details of the sale
        access(self) let details: SaleOfferDetails

        // A capability allowing this resource to withdraw the NFT with the given ID from its collection.
        // This capability allows the resource to withdraw *any* NFT, so you should be careful when giving
        // such a capability to a resource and always check its code to make sure it will use it in the
        // way that it claims.
        access(contract) let nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>

        // borrowNFT
        // This will assert in the same way as the NFT standard borrowNFT()
        // if the NFT is absent, for example if it has been sold via another offer.
        //
        pub fun borrowNFT(): &NonFungibleToken.NFT {
            let ref = self.nftProviderCapability.borrow()!.borrowNFT(id: self.getDetails().nftID)
            //- CANNOT DO THIS IN PRECONDITION: "member of restricted type is not accessible: isInstance"
            //  result.isInstance(self.getDetails().nftType): "token has wrong type"
            assert(ref.isInstance(self.getDetails().nftType), message: "token has wrong type")
            assert(ref.id == self.getDetails().nftID, message: "token has wrong ID")
            return ref as &NonFungibleToken.NFT
        }

        // getDetails
        // Get the details of the current state of the SaleOffer as a struct.
        // This avoids having more public variables and getter methods for them, and plays
        // nicely with scripts (which cannot return resources).
        //
        pub fun getDetails(): SaleOfferDetails {
            return self.details
        }

        // accept
        // Accept the offer, buying the token.
        // This pays the beneficiaries and returns the token to the buyer.
        //
        pub fun accept(payment: @FungibleToken.Vault): @NonFungibleToken.NFT {
            pre {
                self.details.accepted == false: "offer has already been accepted"
                payment.isInstance(self.details.salePaymentVaultType): "payment vault is not requested fungible token"
                payment.balance == self.details.salePrice: "payment vault does not contain requested price"
            }

            // Make sure the offer cannot be accepted again.
            self.details.setToAccepted()

            // Fetch the token to return to the purchaser.
            let nft <-self.nftProviderCapability.borrow()!.withdraw(withdrawID: self.details.nftID)
            // Neither receivers nor providers are trustworthy, they must implement the correct
            // interface but beyond complying with its pre/post conditions they are not gauranteed
            // to implement the functionality behind the interface in any given way.
            // Therefore we cannot trust the Collection resource behind the interface,
            // and we must check the NFT resource it gives us to make sure that it is the correct one.
            assert(nft.isInstance(self.details.nftType), message: "withdrawn NFT is not of specified type")
            assert(nft.id == self.details.nftID, message: "withdrawn NFT does not have specified ID")

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

            // If the offer is accepted, we regard it as completed here.
            // Otherwise we regard it as completed in the destructor.
            emit SaleOfferCompleted(
                saleOfferResourceID: self.uuid,
                storefrontResourceID: self.details.storefrontID,
                accepted: self.details.accepted
            )

            return <-nft
        }

        // destructor
        //
        destroy () {
            // If the offer has not been accepted, we regard it as completed here.
            // Otherwise we regard it as completed in accept().
            // This is because we destroy the offer in Storefront.removeSaleOffer()
            // or Storefront.cleanup() .
            // If we change this destructor, revisit those functions.
            
            if !self.details.accepted {
              log("Destroying sale offer")
                emit SaleOfferCompleted(
                    saleOfferResourceID: self.uuid,
                    storefrontResourceID: self.details.storefrontID,
                    accepted: self.details.accepted
                )
            }
        }

        // initializer
        //
        init (
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
            saleCuts: [SaleCut],
            storefrontID: UInt64
        ) {
            // Store the sale information
            self.details = SaleOfferDetails(
                nftType: nftType,
                nftID: nftID,
                salePaymentVaultType: salePaymentVaultType,
                saleCuts: saleCuts,
                storefrontID: storefrontID
            )

            // Store the NFT provider
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
    }

    // StorefrontManager
    // An interface for adding and removing SaleOffers within a Storefront,
    // intended for use by the Storefront's own
    //
    pub resource interface StorefrontManager {
        // createSaleOffer
        // Allows the Storefront owner to create and insert SaleOffers.
        //
        pub fun createSaleOffer(
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
            saleCuts: [SaleCut]
        ): UInt64
        // removeSaleOffer
        // Allows the Storefront owner to remove any sale offer, acepted or not.
        //
        pub fun removeSaleOffer(saleOfferResourceID: UInt64)
    }

    // StorefrontPublic
    // An interface to allow listing and borrowing SaleOffers, and purchasing items via SaleOffers
    // in a Storefront.
    //
    pub resource interface StorefrontPublic {
        pub fun getSaleOfferIDs(): [UInt64]
        pub fun borrowSaleOffer(saleOfferResourceID: UInt64): &SaleOffer{SaleOfferPublic}?
        pub fun cleanup(saleOfferResourceID: UInt64)
   }

    // Storefront
    // A resource that allows its owner to manage a list of SaleOffers, and anyone to interact with them
    // in order to query their details and purchase the NFTs that they represent.
    //
    pub resource Storefront : StorefrontManager, StorefrontPublic {
        // The dictionary of SaleOffer uuids to SaleOffer resources.
        access(self) var saleOffers: @{UInt64: SaleOffer}

        // insert
        // Create and publish a SaleOffer for an NFT.
        //
         pub fun createSaleOffer(
            nftProviderCapability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>,
            nftType: Type,
            nftID: UInt64,
            salePaymentVaultType: Type,
            saleCuts: [SaleCut]
         ): UInt64 {
            let saleOffer <- create SaleOffer(
                nftProviderCapability: nftProviderCapability,
                nftType: nftType,
                nftID: nftID,
                salePaymentVaultType: salePaymentVaultType,
                saleCuts: saleCuts,
                storefrontID: self.uuid
            )

            let saleOfferResourceID = saleOffer.uuid
            let saleOfferPrice = saleOffer.getDetails().salePrice

            // Add the new offer to the dictionary.
            let oldOffer <- self.saleOffers[saleOfferResourceID] <- saleOffer
            // Note that oldOffer will always be nil, but we have to handle it.
            destroy oldOffer

            emit SaleOfferAvailable(
                storefrontAddress: self.owner?.address!,
                saleOfferResourceID: saleOfferResourceID,
                nftType: nftType,
                nftID: nftID,
                ftVaultType: salePaymentVaultType,
                price: saleOfferPrice
            )

            return saleOfferResourceID
        }

        // removeSaleOffer
        // Remove a SaleOffer that has not yet been accepted from the collection and destroy it.
        //
        pub fun removeSaleOffer(saleOfferResourceID: UInt64) {

            let offer <- self.saleOffers.remove(key: saleOfferResourceID)
                ?? panic("missing SaleOffer")
    
            // This will emit a SaleOfferCompleted event.
            destroy offer
        }

        // getSaleOfferIDs
        // Returns an array of the SaleOffer resource IDs that are in the collection
        //
        pub fun getSaleOfferIDs(): [UInt64] {
            return self.saleOffers.keys
        }

        // borrowSaleItem
        // Returns a read-only view of the SaleItem for the given saleOfferID if it is contained by this collection.
        //
        pub fun borrowSaleOffer(saleOfferResourceID: UInt64): &SaleOffer{SaleOfferPublic}? {
            if self.saleOffers[saleOfferResourceID] != nil {
                return &self.saleOffers[saleOfferResourceID] as! &SaleOffer{SaleOfferPublic}
            } else {
                return nil
            }
        }

        // cleanup
        // Remove an offer *if* it has been accepted.
        // Anyone can call, but at present it only benefits the account owner to do so.
        // Kind purchasers can however call it if they like.
        //
        pub fun cleanup(saleOfferResourceID: UInt64) {
            pre {
                self.saleOffers[saleOfferResourceID] != nil: "could not find offer with given id"
            }

            let offer <- self.saleOffers.remove(key: saleOfferResourceID)!
            assert(offer.getDetails().accepted == true, message: "offer is not accepted, only admin can remove")
            destroy offer
        }

        // destructor
        //
        destroy () {
            destroy self.saleOffers

            // Let event consumers know that this storefront will no longer exist
            emit StorefrontDestroyed(storefrontResourceID: self.uuid)
        }

        // constructor
        //
        init () {
            self.saleOffers <- {}

            // Let event consumers know that this storefront exists
            emit StorefrontInitialized(storefrontResourceID: self.uuid)
        }
    }

    // createStorefront
    // Make creating a Storefront publicly accessible.
    //
    pub fun createStorefront(): @Storefront {
        return <-create Storefront()
    }

    init () {
        self.StorefrontStoragePath = /storage/GeniaceMarketplace
        self.StorefrontPublicPath = /public/GeniaceMarketplace

        emit GeniaceMarketplaceInitialized()
    }
}
