import NonFungibleToken from 0x1d7e57aa55817448

pub contract BSListings: NonFungibleToken {

  // Events
  //
  pub event ContractInitialized()
  pub event Withdraw(id: UInt64, from: Address?)
  pub event Deposit(id: UInt64, to: Address?)
  pub event Minted(id: UInt64, listingID: String)

  // Named Paths
  //
  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath
  pub let MinterStoragePath: StoragePath

  // totalSupply
  // The total number of BSListings that have been minted
  //
  pub var totalSupply: UInt64

  // NFT
  // A listing as an NFT
  //
  pub resource NFT: NonFungibleToken.INFT {
    // The token's ID
    pub let id: UInt64
    // The token's listing ID on Basic Space
    pub let listingID: String

    // initializer
    //
    init(initID: UInt64, initListingID: String) {
      self.id = initID
      self.listingID = initListingID
    }
  }

  // This is the interface that users can cast their BSListings Collection as
  // to allow others to deposit BSListings into their Collection. It also allows for reading
  // the details of BSListings in the Collection.
  pub resource interface BSListingsCollectionPublic {
    pub fun deposit(token: @NonFungibleToken.NFT)
    pub fun getIDs(): [UInt64]
    pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
    pub fun borrowBSListing(id: UInt64): &BSListings.NFT? {
      // If the result isn't nil, the id of the returned reference
      // should be the same as the argument to the function
      post {
        (result == nil) || (result?.id == id):
        "Cannot borrow BSListing reference: The ID of the returned reference is incorrect"
      }
    }
  }

  // Collection
  // A collection of BSListing NFTs owned by an account
  //
  pub resource Collection: BSListingsCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
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
      let token <- token as! @BSListings.NFT

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
      return &self.ownedNFTs[id] as &NonFungibleToken.NFT
    }

    // borrowBSListing
    // Gets a reference to an NFT in the collection as a BSListing,
    // exposing all of its fields (including the listingID).
    // This is safe as there are no functions that can be called on the BSListing.
    //
    pub fun borrowBSListing(id: UInt64): &BSListings.NFT? {
      if self.ownedNFTs[id] != nil {
        let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
        return ref as! &BSListings.NFT
      } else {
        return nil
      }
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
    pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, listingID: String) {

      emit Minted(id: BSListings.totalSupply, listingID: listingID)

      // deposit it in the recipient's account using their reference
      recipient.deposit(token: <-create BSListings.NFT(initID: BSListings.totalSupply,
                                                       initListingID: listingID))

      BSListings.totalSupply = BSListings.totalSupply + (1 as UInt64)
    }
  }

  // fetch
  // Get a reference to a BSListing from an account's Collection, if available.
  // If an account does not have a BSListings.Collection, panic.
  // If it has a collection but does not contain the itemId, return nil.
  // If it has a collection and that collection contains the itemId, return a reference to that.
  //
  pub fun fetch(_ from: Address, itemID: UInt64): &BSListings.NFT? {
    let collection = getAccount(from)
      .getCapability(BSListings.CollectionPublicPath)
      .borrow<&BSListings.Collection{BSListings.BSListingsCollectionPublic}>()
        ?? panic("Couldn't get collection")
    // We trust BSListings.Collection.borowBSListing to get the correct itemID
    // (it checks it before returning it).
    return collection.borrowBSListing(id: itemID)
  }

  // initializer
  //
  init() {
    // Set our named paths
    self.CollectionStoragePath = /storage/bsListingsCollection
    self.CollectionPublicPath = /public/bsListingsCollection
    self.MinterStoragePath = /storage/bsListingsMinter

    // Initialize the total supply
    self.totalSupply = 0

    // Create a Minter resource and save it to storage
    let minter <- create NFTMinter()
    self.account.save(<-minter, to: self.MinterStoragePath)

    emit ContractInitialized()
  }
}
