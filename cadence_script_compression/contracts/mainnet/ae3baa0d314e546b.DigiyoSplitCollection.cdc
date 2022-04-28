//SPDX-License-Identifier: MIT
import NonFungibleToken from 0x1d7e57aa55817448
import Digiyo from 0xae3baa0d314e546b

pub contract DigiyoSplitCollection {

    // SplitCollection stores a dictionary of Digiyo Collections
    // An instance is stored in the field corresponding to its id % numBuckets
    pub resource SplitCollection: Digiyo.DigiyoNFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var collections: @{UInt64: Digiyo.Collection}
        pub let numBuckets: UInt64
        init(numBuckets: UInt64) {
            self.collections <- {}
            self.numBuckets = numBuckets
            var i: UInt64 = 0
            while i < numBuckets {
                self.collections[i] <-! Digiyo.createEmptyCollection() as! @Digiyo.Collection
                i = i + UInt64(1)
            }
        }

        // withdraw removes an instance from a collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            post {
                result.id == withdrawID: "The ID of the withdrawn NFT is incorrect"
            }
            let bucket = withdrawID % self.numBuckets
            let token <- self.collections[bucket]?.withdraw(withdrawID: withdrawID)!
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- Digiyo.createEmptyCollection()
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            return <-batchCollection
        }

        // deposit takes a instance and adds it to the collections dictionary
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let bucket = token.id % self.numBuckets
            let collection <- self.collections.remove(key: bucket)!
            collection.deposit(token: <-token)
            self.collections[bucket] <-! collection
        }

        // batchDeposit deposits all instances into the passed collection
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {
            let keys = tokens.getIDs()
            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }
            destroy tokens
        }

        // getIDs returns an array of IDs corresponding to instances in the collection
        pub fun getIDs(): [UInt64] {
            var ids: [UInt64] = []
            for key in self.collections.keys {
                for id in self.collections[key]?.getIDs() ?? [] {
                    ids.append(id)
                }
            }
            return ids
        }

        // borrowNFT Returns a reference to a Instance in the collection
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            post {
                result.id == id: "The ID of the reference is incorrect"
            }
            let bucket = id % self.numBuckets
            return self.collections[bucket]?.borrowNFT(id: id)!
        }

        // borrowInstance Returns a reference to an Instance in the collection
        pub fun borrowInstance(id: UInt64): &Digiyo.NFT? {
            let bucket = id % self.numBuckets
            return self.collections[bucket]?.borrowInstance(id: id) ?? nil
        }
        destroy() {
            destroy self.collections
        }
    }
    
    pub fun createEmptyCollection(numBuckets: UInt64): @SplitCollection {
        return <-create SplitCollection(numBuckets: numBuckets)
    }
}