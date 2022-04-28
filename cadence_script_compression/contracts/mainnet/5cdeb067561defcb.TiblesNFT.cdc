// TiblesNFT.cdc

pub contract interface TiblesNFT {
  pub let CollectionStoragePath: StoragePath
  pub let PublicCollectionPath: PublicPath

  pub resource interface INFT {
    pub let id: UInt64
    pub let mintNumber: UInt32
    pub fun metadata(): {String:AnyStruct}?
  }

  pub resource interface CollectionPublic {
    pub fun depositTible(tible: @AnyResource{TiblesNFT.INFT})
    pub fun borrowTible(id: UInt64): &AnyResource{TiblesNFT.INFT}
  }
}
