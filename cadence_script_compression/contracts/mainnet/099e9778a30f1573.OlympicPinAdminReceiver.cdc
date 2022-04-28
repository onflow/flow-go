/*

  AdminReceiver.cdc

  This contract defines a function that takes a Olympic Admin
  object and stores it in the storage of the contract account
  so it can be used.

  Do not deploy this contract to initial Admin.
 */

import OlympicPin from  0x1d007eed492fdbbe
import OlympicPinShardedCollection from 0xf087790fe77461e4

pub contract OlympicPinAdminReceiver {

    // storeAdmin takes a OlympicPin Admin resource and 
    // saves it to the account storage of the account
    // where the contract is deployed
    pub fun storeAdmin(newAdmin: @OlympicPin.Admin) {
        self.account.save(<-newAdmin, to: OlympicPin.AdminStoragePath)
    }
    
    init() {
        // Save a copy of the sharded Piece Collection to the account storage
        if self.account.borrow<&OlympicPinShardedCollection.ShardedCollection>(from: OlympicPinShardedCollection.ShardedPieceCollectionPath) == nil {
            let collection <- OlympicPinShardedCollection.createEmptyCollection(numBuckets: 32)
            // Put a new Collection in storage
            self.account.save(<-collection, to: OlympicPinShardedCollection.ShardedPieceCollectionPath)

            self.account.link<&{OlympicPin.PieceCollectionPublic}>(OlympicPin.CollectionPublicPath, target: OlympicPinShardedCollection.ShardedPieceCollectionPath)
        }
    }
}
