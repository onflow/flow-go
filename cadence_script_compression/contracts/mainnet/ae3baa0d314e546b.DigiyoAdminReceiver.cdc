//SPDX-License-Identifier: MIT
import Digiyo from 0xae3baa0d314e546b
import DigiyoSplitCollection from 0xae3baa0d314e546b

pub contract DigiyoAdminReceiver {

    pub let splitCollectionPath: StoragePath

    pub fun storeAdmin(newAdmin: @Digiyo.Admin) {
        self.account.save(<-newAdmin, to: Digiyo.digiyoAdminPath)
    }
    
    init() {
        self.splitCollectionPath = /storage/SplitDigiyoNFTCollection
        if self.account.borrow<&DigiyoSplitCollection.SplitCollection>(from: self.splitCollectionPath) == nil {
            let collection <- DigiyoSplitCollection.createEmptyCollection(numBuckets: 32)
            self.account.save(<-collection, to: self.splitCollectionPath)
            self.account.link<&{Digiyo.DigiyoNFTCollectionPublic}>(Digiyo.collectionPublicPath, target: self.splitCollectionPath)
        }
    }
}