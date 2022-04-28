// SPDX-License-Identifier: UNLICENSED

/*
    Description: Central Collection for a large number of CricketMoments
                 NFTs

    This contract bundles together a bunch of Collection objects 
    in a dictionary, and then distributes the individual Moments between them 
    while implementing the same public interface 
    as the default CricketMomentCollection implementation. 

    If we assume that Moment IDs are uniformly distributed, 
    a ShardedCollection with 10 inner Collections should be able 
    to store 10x as many Moments (or ~1M).

    When Cadence is updated to allow larger dictionaries, 
    then this contract can be retired.

*/
import NonFungibleToken from 0x1d7e57aa55817448
import CricketMoments from 0xed398881d9bf40fb

pub contract CricketMomentsShardedCollection {

    pub let ShardedCollectionStoragePath: StoragePath

    // ShardedCollection stores a dictionary of CricketMoments Collections
    // A Moment is stored in the field that corresponds to its id % numBuckets
    pub resource ShardedCollection: CricketMoments.CricketMomentsCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        
        // Dictionary of CricketMoments collections
        pub var collections: @{UInt64: CricketMoments.Collection}

        // The number of buckets to split Moments into
        // This makes storage more efficient and performant
        pub let numBuckets: UInt64

        init(numBuckets: UInt64) {
            self.collections <- {}
            self.numBuckets = numBuckets

            // Create a new empty collection for each bucket
            var i: UInt64 = 0
            while i < numBuckets {

                self.collections[i] <-! CricketMoments.createEmptyCollection() as! @CricketMoments.Collection

                i = i + 1 as UInt64
            }
        }

        // withdraw removes a Moment from one of the Collections 
        // and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            post {
                result.id == withdrawID: "The ID of the withdrawn NFT is incorrect"
            }
            // Find the bucket it should be withdrawn from
            let bucket = withdrawID % self.numBuckets

            // Withdraw the moment
            let token <- self.collections[bucket]?.withdraw(withdrawID: withdrawID)!
            
            return <-token
        }

        // deposit takes a Moment and adds it to the Collections dictionary
        pub fun deposit(token: @NonFungibleToken.NFT) {

            // Find the bucket this corresponds to
            let bucket = token.id % self.numBuckets

            // Remove the collection
            let collection <- self.collections.remove(key: bucket)!

            // Deposit the nft into the bucket
            collection.deposit(token: <-token)

            // Put the Collection back in storage
            self.collections[bucket] <-! collection
        }

        // getIDs returns an array of the IDs that are in the Collection
        pub fun getIDs(): [UInt64] {

            var ids: [UInt64] = []
            // Concatenate IDs in all the Collections
            for key in self.collections.keys {
                for id in self.collections[key]?.getIDs() ?? [] {
                    ids.append(id)
                }
            }
            return ids
        }

        // borrowNFT Returns a borrowed reference to a Moment in the Collection
        // so that the caller can read data and call methods from it
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            post {
                result.id == id: "The ID of the reference is incorrect"
            }

            // Get the bucket of the nft to be borrowed
            let bucket = id % self.numBuckets

            // Find NFT in the collections and borrow a reference
            return self.collections[bucket]?.borrowNFT(id: id)!
        }


        // borrowCricketMoment Returns a borrowed reference to a Moment in the Collection
        // so that the caller can read data and call methods from it
        // They can use this to read its serial, momentId, metadata
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowCricketMoment(id: UInt64): &CricketMoments.NFT? {

            // Get the bucket of the nft to be borrowed
            let bucket = id % self.numBuckets

            return self.collections[bucket]?.borrowCricketMoment(id: id) ?? nil
        }

        // If a transaction destroys the Collection object,
        // All the NFTs contained within are also destroyed
        destroy() {
            destroy self.collections
        }
    }

    // Creates an empty ShardedCollection and returns it to the caller
    pub fun createEmptyCollection(numBuckets: UInt64): @ShardedCollection {
        return <-create ShardedCollection(numBuckets: numBuckets)
    }

    init() {
        self.ShardedCollectionStoragePath = /storage/CricketMomentsShardedCollection
    }
}
 