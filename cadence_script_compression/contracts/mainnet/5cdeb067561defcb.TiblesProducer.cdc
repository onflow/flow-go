// TiblesProducer.cdc

import TiblesNFT from 0x5cdeb067561defcb

pub contract interface TiblesProducer {
  pub let ProducerStoragePath: StoragePath
  pub let ProducerPath: PrivatePath
  pub let ContentPath: PublicPath
  pub let contentCapability: Capability

  pub event MinterCreated(minterId: String)
  pub event TibleMinted(minterId: String, mintNumber: UInt32, id: UInt64)

  // Producers must provide a ContentLocation struct so that NFTs can access metadata.
  pub struct ContentLocation {}
  pub struct interface IContentLocation {}

  // This is a public resource that lets the individual tibles get their metadata.
  // Adding content is done through the Producer.
  pub resource interface IContent {
    // Content is stored in the set/item/variant structures. To retrieve it, we have a contentId that maps to the path.
    access(contract) let contentIdsToPaths: {String: TiblesProducer.ContentLocation}
    pub fun getMetadata(contentId: String): {String: AnyStruct}?
  }

  // Provides access to producer activities like content creation and NFT minting.
  // The resource is stored in the app account's storage with a link in /private.
  pub resource interface IProducer {
    // Minters create and store tibles before they are sold. One minter per set-item-variant combo.
    access(contract) let minters: @{String: Minter}
  }

  pub resource Producer: IContent, IProducer {
    access(contract) let minters: @{String: Minter}
  }

  // Mints new NFTs for a specific set/item/variant combination.
  pub resource interface IMinter {
    pub let id: String
    // Keeps track of the mint number for items.
    pub var lastMintNumber: UInt32
    // Stored with each minted NFT so that it can access metadata.
    pub let contentCapability: Capability
    // Used only on original purchase, when the NFT gets transferred from the producer to the user's collection.
    pub fun withdraw(mintNumber: UInt32): @AnyResource{TiblesNFT.INFT}
    pub fun mintNext()
  }

  pub resource Minter: IMinter {
    pub let id: String
    pub var lastMintNumber: UInt32
    pub let contentCapability: Capability
    pub fun withdraw(mintNumber: UInt32): @AnyResource{TiblesNFT.INFT}
    pub fun mintNext()
  }
}
