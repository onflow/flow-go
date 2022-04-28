import NonFungibleToken from 0x1d7e57aa55817448

pub contract FantastecNFT: NonFungibleToken {

  // Events
  //
  pub event ContractInitialized()
  pub event Withdraw(id: UInt64 from: Address?)
  pub event Deposit(id: UInt64 to: Address?)
  pub event Minted(item: Item)
  pub event Destroyed(id: UInt64, reason: String)

  // Named Paths
  //
  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath
  pub let MinterStoragePath: StoragePath

  // totalSupply
  // The total number of FantastecNFT that have ever been minted
  //
  pub var totalSupply: UInt64

  pub struct Item {
    pub let id: UInt64
    pub let cardId: UInt64
    pub let edition: UInt64
    pub let mintNumber: UInt64
    pub let metadata: {String: String}
    init(id: UInt64, cardId: UInt64, edition: UInt64, mintNumber: UInt64, metadata: {String: String}){
      self.id = id
      self.cardId = cardId
      self.edition = edition
      self.mintNumber = mintNumber
      self.metadata = metadata
    }
  }

  // NFT: FantastecNFT.NFT
  // Raw NFT, doesn't currently restrict the caller instantiating an NFT
  //
  pub resource NFT: NonFungibleToken.INFT {
    // The token's ID
    pub let id: UInt64
    pub let cardId: UInt64
    pub let edition: UInt64
    pub let mintNumber: UInt64
    pub let metadata: {String: String}

    // initializer
    //
    init(item: Item) {
      self.id = item.id
      self.cardId = item.cardId
      self.edition = item.edition
      self.mintNumber = item.mintNumber
      self.metadata = item.metadata
    }
  }

  // This is the interface that users can cast their FantastecNFT Collection as
  // to allow others to deposit FantastecNFTs into their Collection. It also allows for reading
  // the details of FantastecNFTs in the Collection.
  pub resource interface FantastecNFTCollectionPublic {
    pub fun deposit(token: @NonFungibleToken.NFT)
    pub fun getIDs(): [UInt64]
    pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
    pub fun borrowFantastecNFT(id: UInt64): &FantastecNFT.NFT? {
      post {
        (result == nil) || (result?.id == id):
          "Cannot borrow FantastecNFT reference: The ID of the returned reference is incorrect"
      }
    }
  }

  // Collection
  // A collection of Moment NFTs owned by an account
  //
  pub resource Collection: FantastecNFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
    // dictionary of NFT conforming tokens
    // NFT is a resource type with an `UInt64` ID field
    // metadataObjs is a dictionary of metadata mapped to NFT IDs
    //
    pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

    // deposit
    // Takes a NFT and adds it to the collections dictionary
    // and adds the ID to the id array
    //
    pub fun deposit(token: @NonFungibleToken.NFT) {
      let token <- token as! @FantastecNFT.NFT

      let id: UInt64 = token.id

      // add the new token to the dictionary which removes the old one
      // TODO: This should never happen
      let oldToken <- self.ownedNFTs[id] <- token

      emit Deposit(id: id, to: self.owner?.address)

      if (oldToken != nil){
        emit Destroyed(id: id, reason: "replaced existing resource with the same id")
      }

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
      return &self.ownedNFTs[id] as &NonFungibleToken.NFT
    }

    // borrowFantastecNFT
    // Gets a reference to an NFT in the collection as a FantastecNFT,
    // exposing all of its fields.
    // This is safe as there are no functions that can be called on the FantastecNFT.
    //
    pub fun borrowFantastecNFT(id: UInt64): &FantastecNFT.NFT? {
      if self.ownedNFTs[id] != nil {
        let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
        return ref as! &FantastecNFT.NFT
      } else {
        return nil
      }
    }

    // withdraw
    // Removes an NFT from the collection and moves it to the caller
    //
    pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
      let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

      emit Withdraw(id: token.id, from: self.owner?.address)

      return <-token
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
    // Mints a new NFTs
    // Increments mintNumber
    // deposits the NFT into the recipients collection using their collection reference
    //
    pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, cardId: UInt64, edition: UInt64, mintNumber: UInt64, metadata: {String: String}) {

      let newId = FantastecNFT.totalSupply + (1 as UInt64)

      let nftData: Item = Item(
        id: FantastecNFT.totalSupply,
        cardId: cardId,
        edition: edition,
        mintNumber: mintNumber,
        metadata: metadata,
      )
      var newNFT <-create FantastecNFT.NFT(item: nftData)

      // deposit it in the recipient's account using their reference
      recipient.deposit(token: <- newNFT)

      // emit and update contract
      emit Minted(item: nftData)

      // update contracts
      FantastecNFT.totalSupply = newId
    }
  }

  pub fun getTotalSupply(): UInt64 {
    return FantastecNFT.totalSupply;
  }

  init(){
    // Set our named paths
    self.CollectionStoragePath = /storage/FantastecNFTCollection
    self.CollectionPublicPath = /public/FantastecNFTCollection
    self.MinterStoragePath = /storage/FantastecNFTMinter

    // Initialize the total supply
    self.totalSupply = 0

    // Create a Minter resource and save it to storage
    let minter <- create NFTMinter()
    self.account.save(<-minter, to: self.MinterStoragePath)

    emit ContractInitialized()
  }
}
