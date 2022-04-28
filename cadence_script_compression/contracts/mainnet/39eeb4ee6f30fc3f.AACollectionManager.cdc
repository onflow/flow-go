import AACommon from 0x39eeb4ee6f30fc3f

pub contract AACollectionManager {
    access(self) let collections: @{UInt64: Collection}


    pub let AdminStoragePath: StoragePath

    // A map from nftType-nftID -> Collection ID
    access(self) let items: {String: UInt64}

    pub event CollectionCreated(collectionID: UInt64, name: String)
    pub event CollectionItemAdd(collectionID: UInt64, type: Type, nftID: UInt64)
    pub event CollectionItemRemove(collectionID: UInt64, type: Type, nftID: UInt64)

    pub struct Item {
      pub let type: Type
      pub let nftID: UInt64

      init (type: Type, nftID: UInt64) {
        self.type = type
        self.nftID = nftID
      }
    }

    pub resource interface CollectionPublic {
      pub fun getCuts(): [AACommon.PaymentCut]
    }

    pub resource Collection: CollectionPublic {
      pub var name: String
      pub let items: { String: Item }
      pub let cuts: [AACommon.PaymentCut]

      init(name: String, cuts: [AACommon.PaymentCut]) {
        self.name = name
        self.items = {}
        self.cuts = cuts
      } 

      pub fun addItemToCollection(type: Type, nftID: UInt64) {
        self.items[AACommon.itemIdentifier(type: type, id: nftID)] = Item(type: type, nftID: nftID)

        emit CollectionItemAdd(collectionID: self.uuid, type: type, nftID: nftID)
      }

      pub fun removeItemFromCollection(type: Type, nftID: UInt64) {
          self.items.remove(key: AACommon.itemIdentifier(type: type, id: nftID))

          emit CollectionItemRemove(collectionID: self.uuid, type: type, nftID: nftID)
      }

      pub fun getCuts(): [AACommon.PaymentCut] {
          return self.cuts
      }
    } 

    pub fun borrowCollection(id: UInt64): &Collection{CollectionPublic}? {
        if self.collections[id] != nil {
          return &self.collections[id] as! &Collection{CollectionPublic}
        }
        return  nil
    }

    pub fun getCollectionCuts(type: Type, nftID: UInt64): [AACommon.PaymentCut]? {
      if let collectionID = self.items[AACommon.itemIdentifier(type: type, id: nftID)] {
        let collection = self.borrowCollection(id: collectionID) 

        return collection?.getCuts()
      }

      return nil
    }

    pub resource Administrator {

      pub fun createCollection(name: String, cuts: [AACommon.PaymentCut]) {
        let collection <- create Collection(name: name, cuts: cuts)
        let collectionID = collection.uuid
        let old <- AACollectionManager.collections[collectionID] <- collection
        destroy  old

        emit CollectionCreated(collectionID: collectionID, name: name)
      }
      
      pub fun addItemToCollection(collectionID: UInt64, type: Type, nftID: UInt64) {
        pre {
          AACollectionManager.collections.containsKey(collectionID): "Collection not exist"
        }

        let id = AACommon.itemIdentifier(type: type, id: nftID)
        assert(AACollectionManager.items[id] == nil, message: "1 NFT should only in a collection")

        let collection = &AACollectionManager.collections[collectionID] as &Collection
        collection.addItemToCollection(type: type, nftID: nftID)
        AACollectionManager.items[id] = collectionID
      }


      pub fun removeItemFromCollection(collectionID: UInt64, type: Type, nftID: UInt64) {
        pre {
          AACollectionManager.collections.containsKey(collectionID): "Collection not exist"
        }

        let collection = &AACollectionManager.collections[collectionID] as &Collection
        collection.removeItemFromCollection(type: type, nftID: nftID)

        let id = AACommon.itemIdentifier(type: type, id: nftID)
        AACollectionManager.items.remove(key: id)
      }
    }

    init() {
      self.collections <- {}
      self.items = {}


      self.AdminStoragePath = /storage/AACollectionManagerAdmin
      self.account.save(<- create Administrator(), to: self.AdminStoragePath)
    }
}