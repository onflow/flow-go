import NonFungibleToken from 0x1d7e57aa55817448

// eternal.gg
pub contract Shard: NonFungibleToken {
    // Total amount of Shards that have been minted
    pub var totalSupply: UInt64

    // Total amount of Clips that have been created
    pub var totalClips: UInt32

    // Total amount of Moments that have been created
    pub var totalMoments: UInt32

    // Variable size dictionary of Moment structs
    access(self) var moments: {UInt32: Moment}

    // Variable size dictionary of Clip structs
    access(self) var clips: {UInt32: Clip}

    // Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event MomentCreated(id: UInt32, influencerID: String, splits: UInt8, metadata: {String: String})
    pub event ClipCreated(id: UInt32, momentID: UInt32, sequence: UInt8, metadata: {String: String})
    pub event ShardMinted(id: UInt64, clipID: UInt32)

    pub struct Moment {
        // The unique ID of the Moment
        pub let id: UInt32

        // The influencer that the Moment belongs to
        pub let influencerID: String

        // The amount of Clips the Moments splits into
        pub let splits: UInt8

        // The metadata for a Moment
        access(contract) let metadata: {String: String}

        init(influencerID: String, splits: UInt8, metadata: {String: String}) {
            pre {
                metadata.length > 0: "Metadata cannot be empty"
            }

            self.id = Shard.totalMoments
            self.influencerID = influencerID
            self.splits = splits
            self.metadata = metadata

            // Increment the ID so that it isn't used again
            Shard.totalMoments = Shard.totalMoments + (1 as UInt32)

            // Broadcast the new Moment's data
            emit MomentCreated(
                id: self.id,
                influencerID: self.influencerID,
                splits: self.splits,
                metadata: self.metadata
            )
        }
    }

    pub struct Clip {
        // The unique ID of the Clip
        pub let id: UInt32

        // The moment the Clip belongs to
        pub let momentID: UInt32

        // The sequence of the provided clip
        pub let sequence: UInt8

        // Stores all the metadata about the Clip as a string mapping
        access(contract) let metadata: {String: String}

        init(momentID: UInt32, sequence: UInt8, metadata: {String: String}) {
            pre {
                Shard.moments.containsKey(momentID): "Provided Moment ID does not exist"
                Shard.moments[momentID]!.splits > sequence: "The Sequence must be within the Moment's splits limit"
                metadata.length > 0: "Metadata cannot be empty"
            }

            self.id = Shard.totalClips
            self.momentID = momentID
            self.sequence = sequence
            self.metadata = metadata

            // Increment the ID so that it isn't used again
            Shard.totalClips = Shard.totalClips + (1 as UInt32)

            // Broadcast the new Clip's data
            emit ClipCreated(
                id: self.id,
                momentID: self.momentID,
                sequence: self.sequence,
                metadata: self.metadata
            )
        }
    }

    // Add your own Collection interface so you can use it later
    pub resource interface ShardCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowShardNFT(id: UInt64): &Shard.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Shard reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource NFT: NonFungibleToken.INFT {
        // Identifier of NFT
        pub let id: UInt64

        // Clip ID corresponding to the Shard
        pub let clipID: UInt32

        init(initID: UInt64, clipID: UInt32) {
            pre {
                Shard.clips.containsKey(clipID): "Clip ID does not exist"
            }

            self.id = initID
            self.clipID = clipID

            // Increase the total supply counter
            Shard.totalSupply = Shard.totalSupply + (1 as UInt64)

            emit ShardMinted(id: self.id, clipID: self.clipID)
        }
    }

    pub resource Collection: ShardCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // A resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        // Removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        // Takes a NFT and adds it to the collections dictionary and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @Shard.NFT
            let id: UInt64 = token.id

            // Add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)
            destroy oldToken
        }

        // Returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // Gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // Gets a reference to the Shard NFT for metadata and such
        pub fun borrowShardNFT(id: UInt64): &Shard.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Shard.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // A special authorization resource with administrative functions
    pub resource Admin {
        // Creates a new Moment and returns the ID
        pub fun createMoment(influencerID: String, splits: UInt8, metadata: {String: String}): UInt32 {
            var newMoment = Moment(influencerID: influencerID, splits: splits, metadata: metadata)
            let newID = newMoment.id

            // Store it in the contract storage
            Shard.moments[newID] = newMoment

            return newID
        }

        // Creates a new Clip struct and returns the ID
        pub fun createClip(
            momentID: UInt32,
            sequence: UInt8,
            metadata: {String: String}
        ): UInt32 {
            // Create the new Clip
            var newClip = Clip(
                momentID: momentID,
                sequence: sequence,
                metadata: metadata
            )
            var newID = newClip.id

            // Store it in the contract storage
            Shard.clips[newID] = newClip

            return newID
        }

        // Mints a new NFT with a new ID
        pub fun mintNFT(
            recipient: &{Shard.ShardCollectionPublic},
            clipID: UInt32
        ) {
            // Creates a new NFT with provided arguments
            var newNFT <- create NFT(
                initID: Shard.totalSupply,
                clipID: clipID
            )

            // Deposits it in the recipient's account using their reference
            recipient.deposit(token: <-newNFT)
        }

        pub fun batchMintNFT(
            recipient: &{Shard.ShardCollectionPublic},
            clipID: UInt32,
            quantity: UInt64

        ) {
            var i: UInt64 = 0
            while i < quantity {
                self.mintNFT(recipient: recipient, clipID: clipID)
                i = i + (1 as UInt64)
            }
        }

        // Creates a new Admin resource to be given to an account
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    // Public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @Shard.Collection {
        return <- create Collection()
    }

    // Publicly get a Moment for a given Moment ID
    pub fun getMoment(momentID: UInt32): Moment? {
        return self.moments[momentID]
    }

    // Publicly get a Clip for a given Clip ID
    pub fun getClip(clipID: UInt32): Clip? {
        return self.clips[clipID]
    }

    // Publicly get metadata for a given Moment ID
    pub fun getMomentMetadata(momentID: UInt32): {String: String}? {
        return self.moments[momentID]?.metadata
    }

    // Publicly get metadata for a given Clip ID
    pub fun getClipMetadata(clipID: UInt32): {String: String}? {
        return self.clips[clipID]?.metadata
    }

    // Publicly get all Clips
    pub fun getAllClips(): [Shard.Clip] {
        return Shard.clips.values
    }

    init() {
        // Initialize the total supplies
        self.totalSupply = 0
        self.totalMoments = 0
        self.totalClips = 0

        // Initialize with an empty set of Moments
        self.moments = {}

        // Initialize with an empty set of Clips
        self.clips = {}

        // Create a Collection resource and save it to storage
        self.account.save(<-create Collection(), to: /storage/EternalShardCollection)

        // Create an Admin resource and save it to storage
        self.account.save(<- create Admin(), to: /storage/EternalShardAdmin)

        // Create a public capability for the collection
        self.account.link<&{Shard.ShardCollectionPublic}>(
            /public/EternalShardCollection,
            target: /storage/EternalShardCollection
        )

        emit ContractInitialized()
    }
}