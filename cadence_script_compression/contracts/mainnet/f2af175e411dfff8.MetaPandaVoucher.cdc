/**
  This program is free software: you can redistribute it and/or modify
  it under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  This program is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with this program.  If not, see <https://www.gnu.org/licenses/>.
**/
import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448
import AnchainUtils from 0x7ba45bdcac17806a

// MetaPandaVoucher
// NFT items for MetaPandaVoucher!
//
pub contract MetaPandaVoucher: NonFungibleToken {

  // Standard Events
  //
  pub event ContractInitialized()
  pub event Withdraw(id: UInt64, from: Address?)
  pub event Deposit(id: UInt64, to: Address?)
  pub event Minted(id: UInt64, metadata: Metadata)

  // Redeemed
  // Fires when a user redeems a voucher, prepping
  // it for consumption to receive a reward
  //
  pub event Redeemed(id: UInt64, redeemer: Address)
  
  // Consumed
  // Fires when an Admin consumes a voucher, deleting
  // it forever
  //
  pub event Consumed(id: UInt64)

  // redeemers
  // Tracks all accounts that have redeemed a voucher 
  //
  access(contract) let redeemers: { UInt64 : Address }

  // Named Paths
  //
  pub let RedeemedCollectionStoragePath: StoragePath
  pub let RedeemedCollectionPublicPath: PublicPath
  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath
  pub let MinterStoragePath: StoragePath

  // totalSupply
  // The total number of MetaPandaVoucher that have been minted
  //
  pub var totalSupply: UInt64

  // MetaPandaVoucher Metadata
  //
  pub struct Metadata {
    // Metadata is kept as flexible as possible so we can introduce 
    // any type of sale conditions we want and enforce these off-chain.
    // It would be great to eventually move this validation on-chain.
    pub let details: {String:String}
    init(
      details: {String:String}
    ) {
      self.details = details
    }
  }

  // MetaPandaVoucherView
  //
  pub struct MetaPandaVoucherView {
    pub let uuid: UInt64
    pub let id: UInt64
    pub let metadata: Metadata
    pub let file: AnchainUtils.File
    init(
      uuid: UInt64,
      id: UInt64,
      metadata: Metadata,
      file: AnchainUtils.File
    ) {
      self.uuid = uuid
      self.id = id
      self.metadata = metadata
      self.file = file
    }
  }

  // NFT
  // A MetaPandaVoucher as an NFT
  //
  pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
    // The token's ID
    pub let id: UInt64

    // The token's metadata
    pub let metadata: Metadata

    // The token's file
    pub let file: AnchainUtils.File

    // initializer
    //
    init(id: UInt64, metadata: Metadata, file: AnchainUtils.File) {
      self.id = id
      self.metadata = metadata
      self.file = file
    }

    // getViews
    // Returns a list of ways to view this NFT.
    //
    pub fun getViews(): [Type] {
      return [
        Type<MetadataViews.Display>(),
        Type<MetaPandaVoucherView>(),
        Type<AnchainUtils.File>()
      ]
    }

    // resolveView
    // Returns a particular view of this NFT.
    //
    pub fun resolveView(_ view: Type): AnyStruct? {
      switch view {

        case Type<MetadataViews.Display>():
          return MetadataViews.Display(
            name: "MetaPandaVoucher ".concat(self.id.toString()),
            description: "",
            thumbnail: self.file.thumbnail
          )

        case Type<MetaPandaVoucherView>():
          return MetaPandaVoucherView(
            uuid: self.uuid,
            id: self.id,
            metadata: self.metadata,
            file: self.file
          )
        
        case Type<AnchainUtils.File>():
          return self.file
        
      }
      return nil
    }

  }

  // Collection
  // A collection of MetaPandaVoucher NFTs owned by an account
  //
  pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection, AnchainUtils.ResolverCollection {
    // dictionary of NFT conforming tokens
    // NFT is a resource type with an 'UInt64' ID field
    //
    pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

    // borrowViewResolverSafe
    //
    pub fun borrowViewResolverSafe(id: UInt64): &AnyResource{MetadataViews.Resolver}? {
      if self.ownedNFTs[id] != nil {
        return (&self.ownedNFTs[id] as auth &NonFungibleToken.NFT) 
          as! &MetaPandaVoucher.NFT 
          as &AnyResource{MetadataViews.Resolver}
      } else {
        return nil
      }
    }

    // borrowViewResolver
    //
    pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
      if self.ownedNFTs[id] != nil {
        return (&self.ownedNFTs[id] as auth &NonFungibleToken.NFT) 
          as! &MetaPandaVoucher.NFT 
          as &AnyResource{MetadataViews.Resolver}
      }
      panic("NFT not found in collection.")
    }

    // withdraw
    // Removes an NFT from the collection and moves it to the caller
    //
    pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
      let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

      emit Withdraw(id: token.id, from: self.owner?.address)

      return <-token
    }

    // deposit
    // Takes a NFT and adds it to the collections dictionary
    // and adds the ID to the id array
    //
    pub fun deposit(token: @NonFungibleToken.NFT) {
      let token <- token as! @MetaPandaVoucher.NFT

      let id: UInt64 = token.id

      // add the new token to the dictionary which removes the old one
      let oldToken <- self.ownedNFTs[id] <- token

      emit Deposit(id: id, to: self.owner?.address)

      destroy oldToken
    }

    // getIDs
    // Returns an array of the IDs that are in the collection
    //
    pub fun getIDs(): [UInt64] {
      return self.ownedNFTs.keys
    }

    // borrowNFT
    // Gets a reference to an NFT in the collection
    // so that the caller can read its metadata and call its methods
    //
    pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
      if self.ownedNFTs[id] != nil {
        return &self.ownedNFTs[id] as &NonFungibleToken.NFT
      }
      panic("NFT not found in collection.")
    }

    // destructor
    destroy() {
      destroy self.ownedNFTs
    }

    // initializer
    //
    init () {
      self.ownedNFTs <- {}
    }
  }

  // NFTMinter
  // Resource that an admin or something similar would own to be
  // able to mint new NFTs
  //
  pub resource NFTMinter {

	// mintNFT
    // Mints a new NFT with a new ID and deposits it in the recipients 
    // collection using their collection reference
    //
	pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, metadata: Metadata, file: AnchainUtils.File) {
      emit Minted(id: MetaPandaVoucher.totalSupply, metadata: metadata)
      recipient.deposit(token: <-create MetaPandaVoucher.NFT(id: MetaPandaVoucher.totalSupply, metadata: metadata, file: file))
      MetaPandaVoucher.totalSupply = MetaPandaVoucher.totalSupply + (1 as UInt64)
    }

    // consume
    // Consumes a voucher from the redeemed collection by destroying it
    //
    pub fun consume(_ voucherID: UInt64): Address {
      // Obtain a reference to the redeemed collection
      let redeemedCollection = MetaPandaVoucher.account
        .borrow<&MetaPandaVoucher.Collection>(
          from: MetaPandaVoucher.RedeemedCollectionStoragePath
        )!

      // Burn the voucher
      destroy <- redeemedCollection.withdraw(withdrawID: voucherID)
      
      // Let event listeners know that this voucher has been consumed
      emit Consumed(id: voucherID)
      return MetaPandaVoucher.redeemers.remove(key: voucherID)!
    }

  }

  // createEmptyCollection
  // A public function that anyone can call to create a new empty collection
  //
  pub fun createEmptyCollection(): @NonFungibleToken.Collection {
    return <- create Collection()
  }


  // redeem
  // This public function represents the core feature of this contract: redemptions.
  // The NFTs, aka vouchers, can be 'redeemed' into the RedeemedCollection. The admin
  // can then consume these in exchange for merchandise.
  //
  pub fun redeem(collection: &MetaPandaVoucher.Collection, voucherID: UInt64) {
    // Withdraw the voucher
    let token <- collection.withdraw(withdrawID: voucherID)
    
    // Get a reference to our redeemer collection
    let receiver = MetaPandaVoucher.account
      .getCapability(MetaPandaVoucher.RedeemedCollectionPublicPath)
      .borrow<&{NonFungibleToken.CollectionPublic}>()!
    
    // Deposit the voucher for consumption
    receiver.deposit(token: <- token)

    // Store who redeemed this voucher
    MetaPandaVoucher.redeemers[voucherID] = collection.owner!.address
    emit Redeemed(id: voucherID, redeemer: collection.owner!.address)
  }

  // getRedeemers
  // Return the redeemers dictionary
  //
  pub fun getRedeemers(): { UInt64 : Address } {
    return self.redeemers
  }

  // initializer
  //
  init() {
    // Set our named paths
    self.RedeemedCollectionStoragePath = /storage/MetaPandaVoucherRedeemedCollection
    self.RedeemedCollectionPublicPath = /public/MetaPandaVoucherRedeemedCollection    
    self.CollectionStoragePath = /storage/MetaPandaVoucherCollection
    self.CollectionPublicPath = /public/MetaPandaVoucherCollection
    self.MinterStoragePath = /storage/MetaPandaVoucherMinter

    // Initialize the total supply
    self.totalSupply = 0

    // Initialize redeemers mapping
    self.redeemers = {}

    // Create a Minter resource and save it to storage
    let minter <- create NFTMinter()
    self.account.save(<-minter, to: self.MinterStoragePath)

    // Create a collection that users can place their redeemed vouchers in
    self.account.save(<-create Collection(), to: self.RedeemedCollectionStoragePath) 
    self.account.link<&{
      NonFungibleToken.CollectionPublic, 
      MetadataViews.ResolverCollection, 
      AnchainUtils.ResolverCollection
    }>(
      self.RedeemedCollectionPublicPath, 
      target: self.RedeemedCollectionStoragePath
    )
    
    // Create a personal collection just in case the contract ever holds vouchers to distribute later
    self.account.save(<-create Collection(), to: self.CollectionStoragePath)
    self.account.link<&{
      NonFungibleToken.CollectionPublic, 
      MetadataViews.ResolverCollection, 
      AnchainUtils.ResolverCollection
    }>(
      self.CollectionPublicPath, 
      target: self.CollectionStoragePath
    )

    emit ContractInitialized()
  }
}
