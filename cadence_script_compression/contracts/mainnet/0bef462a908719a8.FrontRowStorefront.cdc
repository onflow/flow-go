/**

    FrontRowStorefront.cdc

    Description: Contract definitions for users to purchase FrontRow NFTs

    Storefront is where admin creates a sale collection and stores
    it in its account storage. In the sale collection, the admin can
    add/remove sale offers and publish a reference so that others can
    see the sale.

    Each sale offer respresents NFTs of a certain blueprint posted
    for sale. Admin sets the price for each sale offer and users
    can purchase any number of NFTs (up to the maximum quantity
    defined on a blueprint) for that price.

    If another user sees an NFT that they want to buy,
    they can send fungible tokens that equal the buy price
    to buy the NFT. The NFT is transferred to them when
    they make the purchase.

    Only the admin is allowed to sell NFTs. Users cannot resell NFTs
    after the purchase.

    When admin creates a sale offer, it supplies four arguments:
    - A FrontRow.Collection capability that allows their sale to withdraw
      a FrontRow NFT when it is purchased.
    - A blueprint ID as a type of NFTs to sell.
    - A sale price for each NFT of the specified blueprint.
    - A FungibleToken.Receiver capability specifying a beneficiary, where
      the funds get sent.
**/

import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393
import NonFungibleToken from 0x1d7e57aa55817448
import FrontRow from 0x0bef462a908719a8

pub contract FrontRowStorefront {

  // Fired once this contract is deployed.
  pub event ContractInitialized()

  // A Storefront resource has been created.
  // Event consumers can now expect events from this Storefront.
  //
  pub event StorefrontInitialized(storefrontResourceId: UInt64)

  // A Storefront has been destroyed.
  // Event consumers can now stop processing events from this Storefront.
  // Note that we do not specify an address.
  //
  pub event StorefrontDestroyed(storefrontResourceId: UInt64)

  // A sale offer has been created and added to a Storefront resource.
  // The Address values here are valid when the event is emitted, but
  // the state of the accounts they refer to may be changed outside of the
  // FrontRowStorefront workflow, so be careful to check when using them.
  pub event SaleOfferAvailable(
    storefrontAddress: Address,
    blueprintId: UInt32,
    price: UFix64
  )

  // A sale offer has been removed.
  //
  // Parameters:  blueprintId: ID of the blueprint that was removed from sale
  //              serialNumber: the place in the edition that this NFT was minted
  //                            It also represents the total number of NFTs sold for
  //                            the blueprint so far
  //              soldOut: a boolean flag showing if all NFTs were completely sold out
  //
  pub event SaleOfferRemoved(blueprintId: UInt32, serialNumber: UInt32, soldOut: Bool)

  // A purchase has been made.
  //
  // Parameters:  blueprintId: ID of the blueprint that was purchased
  //              serialNumber: the place in the edition that this NFT was minted
  //                            It also represents the total number of NFTs sold for
  //                            the blueprint so far
  //              soldOut: a boolean flag showing if all NFTs are completely sold out now
  //
  pub event Purchase(blueprintId: UInt32, serialNumber: UInt32, soldOut: Bool)

  pub let StorefrontStoragePath: StoragePath

  // The public location for a Storefront link.
  pub let StorefrontPublicPath: PublicPath

  // A struct containing a SaleOffer's data.
  pub struct SaleOfferDetails {

    // Number of NFTs this offer has sold.
    pub var sold: UInt32

    // Sale price
    pub var price: UFix64

    // The Blueprint ID
    pub let blueprintId: UInt32

    // The beneficiary of the sale
    pub let beneficiary: Capability<&{FungibleToken.Receiver}>

    // Increment the number of sold NFTs
    access(contract) fun recordPurchase() {
      self.sold = self.sold + 1
    }

    init (
      blueprintId: UInt32,
      price: UFix64,
      beneficiary: Capability<&{FungibleToken.Receiver}>,
    ) {
      // Check if blueprint exists
      let blueprint: FrontRow.Blueprint? = FrontRow.getBlueprint(id: blueprintId)
      assert(blueprint != nil, message: "Blueprint doesn't exist.")

      // Initialize struct fields
      self.sold = 0
      self.blueprintId = blueprintId
      self.price = price
      self.beneficiary = beneficiary
    }
  }

  // An interface providing a useful public interface to a SaleOffer.
  //
  pub resource interface SaleOfferPublic {

    // purchase allows users to buy NFTs
    // It pays the beneficiary and returns the token to the buyer.
    //
    // Parameters:  payment: the FUSD payment for the NFT purchase
    //
    // Returns: purchased @FrontRow.NFT token
    //
    pub fun purchase(payment: @FUSD.Vault): @FrontRow.NFT

    // getDetails returns detailed information about a sale offer
    //
    // Returns: sale offer details
    //
    pub fun getDetails(): SaleOfferDetails
  }

  // SaleOffer is a resource that allows an NFT to be sold for a specified
  // FungibleToken amount.
  pub resource SaleOffer: SaleOfferPublic {

    // The simple (non-Capability, non-complex) details of the sale
    access(self) let details: SaleOfferDetails

    // The capability to mint NFTs
    access(contract) let minterCapability: Capability<&{FrontRow.Minter}>

    // A capability allowing this resource to withdraw the NFT with the given ID
    // from its collection. This capability allows the resource to withdraw *any* NFT,
    // so be careful when giving such a capability to a resource and
    // always check its code to make sure it will use it in the way that it claims.
    access(contract) let nftProviderCapability:
      Capability<&{NonFungibleToken.Provider, FrontRow.CollectionPublic}>

    // Get the details of the current state of the SaleOffer as a struct.
    // This avoids having more public variables and getter methods for them, and plays
    // nicely with scripts (which cannot return resources).
    //
    // Returns: sale offer details such as blueprint id, price, etc
    //
    pub fun getDetails(): SaleOfferDetails {
        return self.details
    }

    // purchase allows users to buy NFTs
    // It pays the beneficiary and returns the token to the buyer.
    //
    // Parameters: payment: the FUSD payment for the NFT purchase
    //
    // Returns: purchased @FrontRow.NFT token
    //
    pub fun purchase(payment: @FUSD.Vault): @FrontRow.NFT {
      //
      let blueprint = FrontRow.getBlueprint(id: self.details.blueprintId)!

      // Check if the payment has sufficient balance to cover the purchase
      assert(
        payment.balance == self.details.price,
        message: "Payment vault doesn't have enough funds."
      )

      // Borrow minting capability for on demand minting
      let minter: &{FrontRow.Minter} = self.minterCapability.borrow()!
      assert(minter != nil, message: "Can't borrow minter capability.")

      // Get serial number of the NFT that's about to be purchased.
      // The serial number based on a particular blueprint.
      let nftSerialNumber : UInt32 = blueprint.getMintCount() + 1

      // Check if NFTs are sold out
      assert(nftSerialNumber <= blueprint.maxQuantity, message: "NFTs are sold out.")

      let nft: @FrontRow.NFT <- self.minterCapability
        .borrow()!
        .mintNFT(blueprintId: blueprint.id)

      // Neither receivers nor providers are trustworthy, they must implement the correct
      // interface but beyond complying with its pre/post conditions they are not
      // guaranteed to implement interface's functionality.
      // Therefore we cannot trust the Collection resource behind the interface,
      // and must check the NFT resource it gives us to make sure that it correct.
      assert(
        nft.isInstance(Type<@FrontRow.NFT>()),
        message: "Withdrawn NFT is not of FrontRow.NFT type."
      )

      // NFT minted: check the blueprint ID and the serial number
      assert(nft.blueprintId == blueprint.id && nft.serialNumber == nftSerialNumber,
        message: "Selling incorrect NFT: serial number or/and blueprint are incorrect."
      )

       // Send payment to the beneficiary
      let beneficiary = self.details.beneficiary.borrow()
      beneficiary!.deposit(from: <-payment)

      // Increment the purchase amount
      self.details.recordPurchase()

      // Emit the Purchase event
      emit Purchase(
        blueprintId: self.details.blueprintId,
        serialNumber: nftSerialNumber,
        // If all minted NFTs for a bluprint are purchased, the offer is soldOut.
        soldOut: nftSerialNumber == blueprint.maxQuantity
      )
      return <- nft
    }

    // Initialize resource fields
    init (
      nftProviderCapability: Capability<&{NonFungibleToken.Provider, FrontRow.CollectionPublic}>,
      blueprintId: UInt32,
      price: UFix64,
      beneficiary: Capability<&{FungibleToken.Receiver}>,
      minterCapability: Capability<&{FrontRow.Minter}>
    ) {
      //
      let blueprint: FrontRow.Blueprint? = FrontRow.getBlueprint(id: blueprintId)

      // Check if blueprint exists
      assert(blueprint != nil, message: "Blueprint doesn't exist.")

      // Store the sale information
      self.details = SaleOfferDetails(
        blueprintId: blueprintId,
        price: price,
        beneficiary: beneficiary,
      )

      // Store capabilities
      self.nftProviderCapability = nftProviderCapability
      self.minterCapability = minterCapability
    }
  }

  // StorefrontManager is the interface for adding and removing SaleOffers within
  // a Storefront. It is intended for use by the Storefront's owner.
  //
  pub resource interface StorefrontManager {

    // createSaleOffer allows the Storefront owner to create and insert sale offers
    // Each sale offer has 1-to-1 relationship to a blueprint and a blueprint ID
    // uniquely identifies each sale offer.
    //
    // Parameters:  nftProviderCapability: capability to borrow NFT
    //              blueprintId: ID of the blueprint to sell
    //              price: listing price for each NFT
    //              beneficiary:  the capability used for depositing the beneficiary's
    //                            sale proceeds
    //
    pub fun createSaleOffer(
      nftProviderCapability: Capability<&{NonFungibleToken.Provider, FrontRow.CollectionPublic}>,
      blueprintId: UInt32,
      price: UFix64,
      beneficiary: Capability<&{FungibleToken.Receiver}>,
      minterCapability: Capability<&{FrontRow.Minter}>
    )

    // removeSaleOffer removes sales offer from the owner's Storefront, sold out or not.
    //
    // Parameters:  blueprintId: ID of the blueprint which is on sale via a sale offer.
    //                           Since there is 1-to-1 relationship between a blueprint
    //                           and a sale offer, the blueprint ID uniquely idetifies
    //                           each sale offer
    //
    pub fun removeSaleOffer(blueprintId: UInt32)
  }

  // StorefrontPublic is the interface to allow listing and borrowing SaleOffers,
  // and purchasing items via SaleOffers in a Storefront.
  //
  pub resource interface StorefrontPublic {
    // getSaleOfferIDs returns an array of all sale offer IDs
    //
    // Returns: an array of all sale offers
    //
    pub fun getSaleOfferIDs(): [UInt32]

    // borrowSaleOffer returns a reference to the SaleOffer resource
    //
    // Parameters: blueprintId: The ID of the blueprint (or a sale offer)
    //
    // Returns: A reference to the SaleOffer resource
    //
    pub fun borrowSaleOffer(blueprintId: UInt32): &SaleOffer{SaleOfferPublic}?
  }

  // Storefront is a resource that allows its owner to manage a list of SaleOffers,
  // and anyone to interact with them in order to query their details and purchase
  // the NFTs that they represent.
  //
  pub resource Storefront: StorefrontManager, StorefrontPublic {

    // The dictionary of SaleOffers. Blueprint ID to SaleOffer resources.
    access(self) var saleOffers: @{UInt32: SaleOffer}

    // Create and publish a SaleOffer for an NFT.
    //
    // Parameters:  nftProviderCapability: capability to borrow NFT
    //              blueprintId: ID of the blueprint to sell
    //              price: listing price for each NFT
    //              beneficiary: capability used for depositing the beneficiary's
    //                            sale proceeds
    //              minterCapability: capability to mint NFTs. Used in on demand minting.
    //
    pub fun createSaleOffer(
      nftProviderCapability: Capability<&{NonFungibleToken.Provider, FrontRow.CollectionPublic}>,
      blueprintId: UInt32,
      price: UFix64,
      beneficiary: Capability<&{FungibleToken.Receiver}>,
      minterCapability: Capability<&{FrontRow.Minter}>
    ) {
      //
      let blueprint: FrontRow.Blueprint? = FrontRow.getBlueprint(id: blueprintId)

      // Check if blueprint exists and if the sale offer is not a duplicate
      assert(blueprint != nil, message: "Blueprint doesn't exist.")
      assert(self.saleOffers[blueprintId] == nil, message: "Sale offer already exists.")

      // Create a sale offer
      let saleOffer <- create SaleOffer(
        nftProviderCapability: nftProviderCapability,
        blueprintId: blueprintId,
        price: price,
        beneficiary: beneficiary,
        minterCapability: minterCapability,
      )

      // Add the new sale offer to the dictionary
      let oldOffer <- self.saleOffers[blueprintId] <- saleOffer

      // Note that oldOffer will always be nil, but we have to handle it
      destroy oldOffer

      emit SaleOfferAvailable(
        storefrontAddress: self.owner?.address!,
        blueprintId: blueprintId,
        price: price
      )
    }

    // removeSaleOffer removes a sale offer from the collection and destroys it.
    //
    // Parameters: blueprintId: The ID of the blueprint to remove from sale
    //
    pub fun removeSaleOffer(blueprintId: UInt32) {

      let blueprint: FrontRow.Blueprint? = FrontRow.getBlueprint(id: blueprintId)
      assert(blueprint != nil, message: "Blueprint doesn't exist.")

      let offer <- self.saleOffers.remove(key: blueprintId)
        ?? panic("Missing sale offer.")

      // Get details of the soon to be removed sale offer
      let saleOfferDetails: SaleOfferDetails = offer.getDetails()

      // This will emit a SaleOfferRemoved event
      destroy offer

      // Let event consumers know that the SaleOffer has been removed
      emit SaleOfferRemoved(
        blueprintId: blueprintId,
        serialNumber: saleOfferDetails.sold,
        soldOut: saleOfferDetails.sold == blueprint!.maxQuantity
      )
    }

    // getSaleOfferIDs returns an array of sale offers listed for sale
    //
    // Returns: an array of blueprint IDs listed in the Storefront for sale
    pub fun getSaleOfferIDs(): [UInt32] {
      return self.saleOffers.keys
    }

    // borrowSaleOffer returns a reference to the sale offer for the given blueprint ID
    //
    // Parameters: blueprintId: The ID of the blueprint of the corresponding sale offer
    //
    // Returns: a reference to the &SaleOffer resource limited only to functionality
    //          outlined in the SaleOfferPublic interface
    //
    pub fun borrowSaleOffer(blueprintId: UInt32): &SaleOffer{SaleOfferPublic}? {
      if self.saleOffers[blueprintId] != nil {
        return &self.saleOffers[blueprintId] as! &SaleOffer{SaleOfferPublic}
      } else {
        return nil
      }
    }

    // Removes all sale offers from the Storefront
    destroy () {
      destroy self.saleOffers

      // Let event consumers know that this storefront will no longer exist
      emit StorefrontDestroyed(storefrontResourceId: self.uuid)
    }

    // Initialize resource fields
    init () {
      self.saleOffers <- {}

      // Let event consumers know that now they can expect events from this Storefront
      emit StorefrontInitialized(storefrontResourceId: self.uuid)
    }
  }

  // Initialize contract fields.
  init () {
    // Set paths aliases
    self.StorefrontStoragePath = /storage/FrontRowStorefront
    self.StorefrontPublicPath = /public/FrontRowStorefront

    // Create the Storefront resource and save it to storage
    self.account.save<@Storefront>(<- create Storefront(), to: self.StorefrontStoragePath)

    // Create a public capability for the FrontRowStorefront
    self.account.link<&FrontRowStorefront.Storefront{FrontRowStorefront.StorefrontPublic}>
      (FrontRowStorefront.StorefrontPublicPath,
      target: FrontRowStorefront.StorefrontStoragePath)

    emit ContractInitialized()
  }
}
