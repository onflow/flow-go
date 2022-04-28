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

// MetaPanda
// NFT items for MetaPanda!
//
pub contract MetaPanda: NonFungibleToken {

  // Events
  //
  pub event ContractInitialized()
  pub event Withdraw(id: UInt64, from: Address?)
  pub event Deposit(id: UInt64, to: Address?)
  pub event Minted(id: UInt64, metadata: Metadata)

  // Named Paths
  //
  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath
  pub let MinterStoragePath: StoragePath

  // totalSupply
  // The total number of MetaPanda that have been minted
  //
  pub var totalSupply: UInt64

  // Panda Metadata
  //
  pub struct Metadata {
    pub let clothesAccessories: String?
    pub let facialAccessories: String?
    pub let facialExpression: String?
    pub let headAccessories: String?
    pub let handAccessories: String?
    pub let clothesBody: String?
    pub let background: String?
    pub let foreground: String?
    pub let basePanda: String?
    init(
      clothesAccessories: String?,
      facialAccessories: String?,
      facialExpression: String?,
      headAccessories: String?,
      handAccessories: String?,
      clothesBody: String?,
      background: String?,
      foreground: String?,
      basePanda: String?
    ) {
      self.clothesAccessories = clothesAccessories
      self.facialAccessories = facialAccessories
      self.facialExpression = facialExpression
      self.headAccessories = headAccessories
      self.handAccessories = handAccessories
      self.clothesBody = clothesBody
      self.background = background
      self.foreground = foreground
      self.basePanda = basePanda
    }
  }

  // MetaPandaView
  //
  pub struct MetaPandaView {
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
  // A MetaPanda as an NFT
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
        Type<MetaPandaView>(),
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
            name: "Panda ".concat(self.id.toString()),
            description: "",
            thumbnail: self.file.thumbnail
          )
        
        case Type<MetaPandaView>():
          return MetaPandaView(
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
  // A collection of MetaPanda NFTs owned by an account
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
          as! &MetaPanda.NFT 
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
          as! &MetaPanda.NFT 
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
      let token <- token as! @MetaPanda.NFT

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

  // createEmptyCollection
  // public function that anyone can call to create a new empty collection
  //
  pub fun createEmptyCollection(): @NonFungibleToken.Collection {
    return <- create Collection()
  }

  // NFTMinter
  // Resource that an admin or something similar would own to be
  // able to mint new NFTs
  //
	pub resource NFTMinter {

		// mintNFT
    // Mints a new NFT with a new ID
		// and deposit it in the recipients collection using their collection reference
    //
		pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, metadata: Metadata, file: AnchainUtils.File) {
      emit Minted(id: MetaPanda.totalSupply, metadata: metadata)
			recipient.deposit(token: <-create MetaPanda.NFT(id: MetaPanda.totalSupply, metadata: metadata, file: file))
      MetaPanda.totalSupply = MetaPanda.totalSupply + (1 as UInt64)
		}

	}

  // initializer
  //
	init() {
    // Set our named paths
    self.CollectionStoragePath = /storage/MetaPandaCollection
    self.CollectionPublicPath = /public/MetaPandaCollection
    self.MinterStoragePath = /storage/MetaPandaMinter

    // Initialize the total supply
    self.totalSupply = 0

    // Create a Minter resource and save it to storage
    let minter <- create NFTMinter()
    self.account.save(<-minter, to: self.MinterStoragePath)

    emit ContractInitialized()
	}
}
