/**
*  SPDX-License-Identifier: GPL-3.0-only
*/
import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import FUSD from 0x3c5959b568896393

// GooberXContract - The contract Erik needs to party !
pub contract GooberXContract : NonFungibleToken {

  // Data 
  //
  // Array of goober IDs in creation order for paged access
  access(self) var gooberRegister: [UInt64]
  // Contains all minted goobers including the payload
  access(self) var mintedGoobers: [GooberStruct]
  // Index containing the reference between GooberID and the GooberStruct
  access(self) var mintedGoobersIndex: {UInt64: Int}
  // Pool of mintable goobers
  access(self) var gooberPool: [GooberStruct]
  // Giveaways
  access(self) var giveaways: {String: GooberStruct}

  // Events
  //
  // Event to be emitted when the contract is initialized
  pub event ContractInitialized()
  // Event to be emitted whenever a NFT is withdrawn from a collection
  pub event Withdraw(id: UInt64, from: Address?)
  // Event to be emitted whenever a NFT is deposited to a collection
  pub event Deposit(id: UInt64, to: Address?)
  // Event to be eitted whenever a new NFT is minted
  pub event Minted(id: UInt64, uri: String, price: UFix64)
  // Event to be emitted whenever a new NFT is minted
  pub event Airdropped(id: UInt64)
  // Event to be emitted whenever a Goober is named differently
  pub event NamedGoober(id: UInt64, name: String)

  // Collection Paths
  //
  // Private collection storage path
  pub let CollectionStoragePath: StoragePath
  // Public collection storage path
  pub let CollectionPublicPath: PublicPath
  // Admin storage path
  pub let AdminStoragePath: StoragePath

  // totalSupply
  pub var totalSupply: UInt64

  // Index pointing to the first Goober in the pool
  pub var firstGooberInPoolIndex: UInt64

  // Goober data structure
  // This structure is used to store the payload of the Goober NFT
  //
  pub struct GooberStruct {

    pub let gooberID: UInt64
    // Metadata of the Goober
    pub let metadata: {String: AnyStruct}
    // URI pointing to the IPFS address of the picture related to the Goober NFT
    pub let uri: String
    // Price of the Goober NFT
    pub let price: UFix64

    // init
    // Constructor method to initialize a Goober
    //
    init( gooberID: UInt64, 
          uri: String, 
          metadata: {String: AnyStruct},
          price: UFix64) {
      self.gooberID = gooberID
      self.uri = uri
      self.metadata = metadata
      self.price = price
    }
  }

  // Goober NFT
  pub resource NFT: NonFungibleToken.INFT {
  
    // NFT id
    pub let id: UInt64
    // Data structure containing all relevant describing data
    pub let data: GooberStruct

    // init 
    // Constructor to initialize the Goober NFT 
    // Requires a GooberStruct to be passed
    //
    init(goober: GooberStruct) {
      GooberXContract.totalSupply = GooberXContract.totalSupply + 1
      let gooberID = GooberXContract.totalSupply

      self.data = GooberStruct(
        gooberID: gooberID, 
        uri: goober.uri, 
        metadata: goober.metadata,
        price: goober.price)
      self.id = UInt64(self.data.gooberID)

      GooberXContract.gooberRegister.append(self.data.gooberID)
      GooberXContract.mintedGoobers.append(self.data)
      GooberXContract.mintedGoobersIndex[self.data.gooberID] = GooberXContract.mintedGoobers.length - 1
    }
  }

  // Interface to publicly acccess GooberCollections 
  pub resource interface GooberCollectionPublic {

    // Deposit NFT
    // Param token refers to a NonFungibleToken.NFT to be deposited within this collection
    //
    pub fun deposit(token: @NonFungibleToken.NFT)

    // Get all IDs related to the current Goober Collection
    // returns an Array of IDs
    //
    pub fun getIDs(): [UInt64]

    // Borrow NonFungibleToken NFT with ID from current Goober Collection 
    // Param id refers to a Goober ID
    // Returns a reference to the NonFungibleToken.NFT
    //
    pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT

    // Eventually borrow Goober NFT with ID from current Goober Collection
    // Returns an option reference to GooberXContract.NFT
    //
    pub fun borrowGoober(id: UInt64): &GooberXContract.NFT? {
      // If the result isn't nil, the id of the returned reference
      // should be the same as the argument to the function
      post {
          (result == nil) || (result?.id == id):
              "Cannot borrow Goober reference: The ID of the returned reference is incorrect"
      }
    }

    // List all goobers the user of the current collection owns
    // Returns a dictionary having the id as key and a GooberStruct as value
    //
    pub fun listUsersGoobers(): {UInt64: GooberStruct}

    // List a paged resultset of Goobers of the users collection
    // Param page determines the current page of the resultset
    // Param pageSize determines the maximum size of the resultset
    // Returns a dictionary having the id as key and a GooberStruct as value
    //
    pub fun listUsersGoobersPaged(page: UInt32, pageSize: UInt32): {UInt64: GooberStruct}
  }

  // Resource to define methods to be used by an Admin in order to administer and populate
  // the Goobers 
  //
  pub resource Admin {

    // Pre-mints Goobers in a global pool
    // Populates a pool. Each time a user mints a goober from the pool,
    // the first Goober is removed from the pool and minted as a NFT
    // in the users collection.
    //
    pub fun addGooberToPool(
      uri: String, 
      metadata: {String: AnyStruct}
      address: Address,
      price: UFix64) {
      pre {
        uri.length > 0 : "Could not create Goober: uri is required."
      }
      
      GooberXContract.gooberPool.append(
        GooberStruct( gooberID: 0 as UInt64, 
                      uri: uri, 
                      metadata: metadata,
                      price: price))
    }

    // Replaces Goobers in global pool
    // This is used to exchange existing Goober Pool 
    //
    pub fun replaceGooberInPool(
      uri: String, 
      metadata: {String: AnyStruct}
      address: Address,
      price: UFix64,
      index: Int) {
      pre {
        uri.length > 0 : "Could not create Goober: uri is required."
        index >= 0 : "Index is out of bounds"
        GooberXContract.gooberPool.length > index : "Index is out of bounds."
      }
      
      GooberXContract.gooberPool[index] = GooberStruct(
        gooberID: 0 as UInt64, 
        uri: uri, 
        metadata: metadata,
        price: price)
    }

    // removeGooberFromPool
    // removes a goober from the pool using a index paramter
    //
    pub fun removeGooberFromPool(index: UInt64) {
      pre {
        GooberXContract.gooberPool[index] != nil : "Could not delete goober from pool: goober does not exist."
      }
      GooberXContract.gooberPool.remove(at: index)
    }

    // adminMintNFT
    // Mints a new NFT with a new ID
    // to the contract owner account for free
    //
    pub fun adminMintNFT(recipient: &{NonFungibleToken.CollectionPublic}) {
      pre {
        GooberXContract.gooberPool.length > 0 : "GooberPool doesnt contain any goober to mint"
      }

      // deposit it in the recipient's account using their reference
      let gooberFromPool : GooberStruct = GooberXContract.gooberPool.remove(at: GooberXContract.firstGooberInPoolIndex)
      recipient.deposit(token: <-create GooberXContract.NFT(goober: gooberFromPool!))
      
      emit Minted(id: GooberXContract.totalSupply, uri: gooberFromPool!.uri, price: gooberFromPool!.price)
    }

    // adminMintAirDropNFT
    // Mints a new NFT with a new ID and airdrops the Goober to a Recipient
    //
    pub fun adminMintAirDropNFT(
        uri: String, 
        metadata: {String: AnyStruct}
        address: Address,
        price: UFix64,
        recipient: &{NonFungibleToken.CollectionPublic}) {
      pre {
        uri.length > 0 : "Could not create Goober: uri is required."
      }
      // GooberStruct initializing
      let goober : GooberStruct = GooberStruct(
        gooberID: 0 as UInt64, 
        uri: uri, 
        metadata: metadata,
        price: price)
      recipient.deposit(token: <-create GooberXContract.NFT(goober: goober))
      
      emit Minted(id: GooberXContract.totalSupply, uri: goober!.uri, price: goober!.price)
      emit Airdropped(id: GooberXContract.totalSupply)
    }   


    // createGiveaway
    // add a new Giveaway key to the available giveaways
    // Important: the code needs to be hashed sha3_256 off chain
    //
    pub fun createGiveaway(giveawayKey: String) {
      pre {
        GooberXContract.gooberPool.length > 0 : "GooberPool doesnt contain any goober to add as giveaway"
      }

      if (GooberXContract.giveaways.containsKey(giveawayKey)){
        panic("Giveaway code already known.")
      }
      let gooberFromPool : GooberStruct = GooberXContract.gooberPool.remove(at: GooberXContract.firstGooberInPoolIndex)
      GooberXContract.giveaways.insert(key: giveawayKey, gooberFromPool)
    }

  }

  // Collection
  // A collection of Goober NFTs owned by an account
  //
  pub resource Collection: GooberCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
    // dictionary of NFT conforming tokens
    // NFT is a resource type with an `UInt64` ID field
    //
    pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

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
        let token <- token as! @GooberXContract.NFT

        let id: UInt64 = token.id

        // add the new token to the dictionary which removes the old one
        let oldToken <- self.ownedNFTs[id] <- token
        emit Deposit(id: id, to: self.owner?.address)
        // destroy old resource
        destroy oldToken

        // update goober overview
        let index = GooberXContract.mintedGoobersIndex[id]
        let tmpGoober : GooberStruct = GooberXContract.mintedGoobers[index!]
        GooberXContract.mintedGoobers[index!] = GooberStruct(
                                                  gooberID: tmpGoober.gooberID, 
                                                  uri: tmpGoober.uri, 
                                                  metadata: tmpGoober.metadata,
                                                  price: tmpGoober.price)
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
        return &self.ownedNFTs[id] as &NonFungibleToken.NFT
    }

    // borrowGoober
    // Gets a reference to an NFT in the collection as a Goober,
    // exposing all data.
    // This is safe as there are no functions that can be called on the Goober.
    //
    pub fun borrowGoober(id: UInt64): &GooberXContract.NFT? {
        if self.ownedNFTs[id] != nil {
          let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
          return ref as! &GooberXContract.NFT
        } else {
          return nil
        }
    }

    // List all goobers the user of the current collection owns
    // Returns a dictionary having the id as key and a GooberStruct as value
    //
    pub fun listUsersGoobers(): {UInt64: GooberStruct} {
      var goobers: {UInt64: GooberStruct} = {}
      for key in self.ownedNFTs.keys {
        let el = &self.ownedNFTs[key] as auth &NonFungibleToken.NFT
        let goober = el as! &GooberXContract.NFT
        goobers.insert(key: goober.id, goober.data)
      }
      return goobers
    }

    // List a paged resultset of Goobers of the users collection
    // Param page determines the current page of the resultset
    // Param pageSize determines the maximum size of the resultset
    // Returns a dictionary having the id as key and a GooberStruct as value
    //
    pub fun listUsersGoobersPaged(page: UInt32, pageSize: UInt32): {UInt64: GooberStruct} {
      var it : UInt32 = 0
      var goobers: {UInt64: GooberStruct} = {}
      let len : Int = self.ownedNFTs.length
      
      let beginIndex : UInt32 = (page * pageSize)
      let endIndex : UInt32 = (page * pageSize) + pageSize

      // Optimize method to return empty dictionary when the paging is beyond borders
      if ((beginIndex + 1) > UInt32(len)) {
        return goobers;
      }

      // Iterate all keys because regular index, contains not possible
      for key in self.ownedNFTs.keys {
        if (it >= beginIndex && it < endIndex) {
          let el = &self.ownedNFTs[key] as auth &NonFungibleToken.NFT
          let goober = el as! &GooberXContract.NFT
          goobers.insert(key: goober.id, goober.data)
        }
        // Increment iterator
        it = it +1
        // Check boundary
        if (it > endIndex) {
          break
        }
      }
      return goobers
    }

    // destructor
    //
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

  // mintNFT
  // Mints a new NFT with a new ID
  // and deposit it in the recipients collection using their collection reference
  //
  pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, address: Address, paymentVault: @FungibleToken.Vault) {
    pre {
      GooberXContract.gooberPool.length > 0 : "GooberPool doesnt contain any goober to mint"
      paymentVault.balance >= GooberXContract.gooberPool[GooberXContract.firstGooberInPoolIndex].price : "Could not mint goober: payment balance insufficient."
      paymentVault.isInstance(Type<@FUSD.Vault>()): "payment vault is not requested fungible token"   
    }

    // pay
    let gooberContractAccount: PublicAccount = getAccount(GooberXContract.account.address)
    let gooberContractReceiver: Capability<&{FungibleToken.Receiver}> = gooberContractAccount.getCapability<&FUSD.Vault{FungibleToken.Receiver}>(/public/fusdReceiver)!
    let borrowGooberContractReceiver = gooberContractReceiver.borrow()!
    borrowGooberContractReceiver.deposit(from: <- paymentVault.withdraw(amount: paymentVault.balance))

    // deposit it in the recipient's account using their reference
    let gooberFromPool : GooberStruct = GooberXContract.gooberPool.remove(at: GooberXContract.firstGooberInPoolIndex)
    recipient.deposit(token: <-create GooberXContract.NFT(goober: gooberFromPool!))
    
    emit Minted(id: GooberXContract.totalSupply, uri: gooberFromPool!.uri, price: gooberFromPool!.price)
    destroy paymentVault
  }

  // retrieveGiveaway
  // Retrieves a giveaway and adds a new NFT with a new ID
  // and deposit it in the recipients collection using their collection reference
  //
  pub fun retrieveGiveaway(recipient: &{NonFungibleToken.CollectionPublic}, address: Address, giveawayCode: String) {
    // Hash giveawayCode
    let digest = HashAlgorithm.SHA3_256.hash(giveawayCode.decodeHex())
    let giveawayKey = String.encodeHex(digest)
    if (!GooberXContract.giveaways.containsKey(giveawayKey)) {
       let msg = "Unknown Giveaway Code:"
       panic(msg.concat(giveawayKey))
    }
    
    // deposit it in the recipient's account using their reference
    let goober = GooberXContract.giveaways[giveawayKey]
    recipient.deposit(token: <-create GooberXContract.NFT(goober: goober!))
    GooberXContract.giveaways.remove(key: giveawayKey)

    emit Minted(id: GooberXContract.totalSupply, uri: goober!.uri, price: goober!.price)
  }

  // nameYourGoober
  // Give your Goober a name
  pub fun nameYourGoober(newName: String, gooberID: UInt64, collection: &GooberXContract.Collection, paymentVault: @FungibleToken.Vault) {
    pre {
      paymentVault.balance >= UFix64(5) : "Could not mint goober: payment balance insufficient."
      paymentVault.isInstance(Type<@FUSD.Vault>()): "payment vault is not requested fungible token"
      newName.length <= 20 : "Cannot give the Goober a name which is longer than 20 characters"
    }

    // pay
    let gooberContractAccount: PublicAccount = getAccount(GooberXContract.account.address)
    let gooberContractReceiver: Capability<&{FungibleToken.Receiver}> = gooberContractAccount.getCapability<&FUSD.Vault{FungibleToken.Receiver}>(/public/fusdReceiver)!
    let borrowGooberContractReceiver = gooberContractReceiver.borrow()!
    borrowGooberContractReceiver.deposit(from: <- paymentVault.withdraw(amount: paymentVault.balance))

    // name Goober
    let token <- collection.withdraw(withdrawID: gooberID) as! @GooberXContract.NFT
    token.data.metadata.insert(key: "name", newName)

    // update Goober information in contract
    self.mintedGoobers[self.mintedGoobersIndex[token.data.gooberID]!] = token.data

    // Deposit updated NFT
    collection.deposit(token: <- token)

    // emit Naming Event
    emit NamedGoober(id: gooberID, name: newName)

    destroy paymentVault
  }

  // getGoober
  // fetches a goober from the minted goobers index
  // Param gooberID refers to a Goober ID
  // returns the GooberStruct refered by the Goober ID
  //
  pub fun getGoober(gooberID: UInt64): GooberStruct {
    let index: Int = self.mintedGoobersIndex[gooberID]!
    let goober = self.mintedGoobers[index]
    return goober
  }

  // getGooberzCount
  // return the amount of goobers minted
  // 
  pub fun getGooberzCount(): UInt64 {
    return self.totalSupply
  }

  // getAvailableGooberzCount
  // returns the amount of currently available goobers for minting
  //
  pub fun getAvailableGooberzCount(): UInt64 {
    return UInt64(self.gooberPool.length)
  }

  // isGooberzPoolEmpty
  // returns a bool to indicate whether or not there is a goober to mint
  //
  pub fun isGooberzPoolEmpty(): Bool {
    return (self.gooberPool.length > 0)
  }

  // listMintedGoobers
  // list all minted goobers
  //
  pub fun listMintedGoobers(): [GooberStruct] {
    return self.mintedGoobers
  }

  // listMintedGoobersPage
  // List minted goobers paged
  // Param page - indicates the page of the result set
  // Param page size - indicates the maximum size of the resultset
  // returns an array of GooberStructs
  //
  pub fun listMintedGoobersPaged(page: UInt32, pageSize: UInt32): [GooberStruct] {
    var it : UInt32 = 0
    var retValues : [GooberStruct] = []
    let len : Int = self.mintedGoobers.length
    
    while it < pageSize {
      let pointer : UInt32 = (page * pageSize) + it
      if pointer < UInt32(len) {
        retValues.append(self.mintedGoobers[pointer])
      }
      it = it + 1
    }
    return retValues
  }

  // getGooberPrice
  // return the goober price
  //
  pub fun getGooberPrice(): UFix64 {
    if (self.gooberPool.length > 0) {
      return self.gooberPool[self.firstGooberInPoolIndex].price
    }
    return UFix64(0)
  }

  // getContractAddress
  // returns address to smart contract
  //
  pub fun getContractAddress(): Address {
    return self.account.address
  }

  // init function of the smart contract
  //
  init() {

    // Initialize the total supply
    self.totalSupply = 0

    // Initialize the firstGooberInPoolIndex variable
    self.firstGooberInPoolIndex = 0

    // Init collections
    self.CollectionStoragePath = /storage/GooberzPartyFolksCollection
    self.CollectionPublicPath = /public/GooberzPartyFolksCollectionPublic
    
    // init & save Admin resource to Admin collection
    self.AdminStoragePath = /storage/GooberzPartyFolksAdmin
    self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)
  
    // Initialize global variables
    self.gooberRegister = []
    self.gooberPool = []
    self.mintedGoobersIndex = {}
    self.mintedGoobers = []
    self.giveaways = {}
  }
}

