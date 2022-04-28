/*
    Description: Central Smart Contract for ROX digital collectibles

    This smart contract contains the core functionality for 
    ROX digital collectibles, created by Rox.gg team

    The contract provides functionality to mint Rox boxes,
    fill boxes with newly minted Rox NFTs and transfer them.

    Each Rox Box contains a track list of different types of Rox NFTs
    where type is specified by the roxId and tier.

    Rox Box is managed by the admin and only admin has the ability
    to mint boxes with nfts and lock the box. When the box is locked
    no NFTs can be minted inside that box.

    There is also a Collection resource that every Rox NFT user will
    use to read the NFTs data and transfer them between accounts.
*/

import NonFungibleToken from 0x1d7e57aa55817448

pub contract RoxContract: NonFungibleToken {

    // -----------------------------------------------------------------------
    // RoxContract Events
    // -----------------------------------------------------------------------

    pub event ContractInitialized()

    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, roxId: String)
    pub event BatchMinted(quantity: UInt64, roxId: String)

    pub event BoxCreated(boxId: UInt32)
    // Emitted when a Box is locked, meaning Rox Nfts cannot be added
    pub event BoxLocked(boxId: UInt32)
    pub event RoxDestroyed(id: UInt64)

    // Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath

    // The dictionary of all the boxes by id
    access(self) var boxes: @{UInt32: Box}
    // The number of Rox NFTs each box contains
    access(self) var mintedNumberPerBox: {UInt32: UInt32}

    // The id of the next minted box
    pub var nextBoxId: UInt32

    // The total number of Rox NFTs that have been minted
    pub var totalSupply: UInt64

    // Box is used to manage NFTs
    // It tracks how many NFTs are created per specific rox type
    // and all the NFTs are created only through Box
    pub resource Box {

        // Unique ID for the box
        pub let boxId: UInt32
        pub let name: String
        pub let metadata: {String: String}
        pub var locked: Bool

        // The number of minted Rox NFTs per specific rox type (roxId)
        pub var mintedNumberPerRox: {String: UInt32}

        init(name: String, metadata: {String: String}) {
            pre {
                name.length > 0: "New Box name cannot be empty"
            }
            self.boxId = RoxContract.nextBoxId
            self.name = name 
            self.locked = false
            self.mintedNumberPerRox = {}
            self.metadata = metadata

            // Increment the boxID so that it isn't used again
            RoxContract.nextBoxId = RoxContract.nextBoxId + (1 as UInt32)
            RoxContract.mintedNumberPerBox[self.boxId] = 0 // At the moment the box contains zero NFTs

            emit BoxCreated(boxId: self.boxId)
        }

        // locks the Box so that no more Rox Nfts can be added to it
        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit BoxLocked(boxId: self.boxId)
            }
        }

        pub fun mintRox(recipient: &{NonFungibleToken.CollectionPublic}, roxId: String, tier: String, metadata: {String: String}) {
            pre {
                !self.locked: "Cannot mint the rox: This box is locked"
            }

            if (self.mintedNumberPerRox[roxId] == nil){
                self.mintedNumberPerRox[roxId] = 0
            }
            self.mintedNumberPerRox[roxId] = self.mintedNumberPerRox[roxId]! + (1 as UInt32) // +1 minted number of NFTs for this specific rox type in box
            RoxContract.mintedNumberPerBox[self.boxId] = RoxContract.mintedNumberPerBox[self.boxId]! + (1 as UInt32) // +1 total minted number of NFTs in box

            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-create NFT(boxId: self.boxId,
                                                  roxId: roxId,
                                                  tier: tier,
                                                  mintNumber: self.mintedNumberPerRox[roxId]!,
                                                  metadata: metadata))

            emit Minted(id: RoxContract.totalSupply, roxId: roxId)
        }

        pub fun batchMintRox(recipient: &{NonFungibleToken.CollectionPublic}, quantity: UInt64, roxId: String, tier: String, metadata: {String: String}) {
            pre {
                !self.locked: "Cannot mint the rox: This box is locked"
            }

            var i: UInt64 = 0
            while i < quantity {
                self.mintRox(recipient: recipient, roxId: roxId, tier: tier, metadata: metadata)
                i = i + 1 as UInt64
            }

            emit BatchMinted(quantity: quantity, roxId: roxId)
        }
    }

    // NFT
    // A Rox collectible as an NFT
    pub resource NFT: NonFungibleToken.INFT {
        
        // Global unique rox id
        pub let id: UInt64

        // Id of the box that the Rox comes from
        pub let boxId: UInt32

        // Specifies the Rox NFT collectible type
        pub let roxId: String

        // Specifies the rox tier: platinum, bronze, gold etc.
        pub let tier: String

        // The mint number for this specific rox type in the box
        pub let mintNumber: UInt32
        
        pub let metadata: {String: String}

        init(boxId: UInt32, roxId: String, tier: String, mintNumber: UInt32, metadata: {String: String}) {
            
            RoxContract.totalSupply = RoxContract.totalSupply + 1 as UInt64
            
            self.id = RoxContract.totalSupply

            self.boxId = boxId
            self.roxId = roxId
            self.tier = tier
            self.mintNumber = mintNumber
            self.metadata = metadata
        }

        destroy() {
            emit RoxDestroyed(id: self.id)
        }
    }

    pub resource interface RoxCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowRoxNft(id: UInt64): &RoxContract.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Rox reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: RoxCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @RoxContract.NFT

            let id: UInt64 = token.id

            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowRoxNft(id: UInt64): &RoxContract.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &RoxContract.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create RoxContract.Collection()
    }

    // fetch
    // Get a reference to a RoxNft from an account's Collection, if available.
    // If an account does not have a RoxContract.Collection, panic.
    // If it has a collection but does not contain the itemId, return nil.
    // If it has a collection and that collection contains the itemId, return a reference to that.
    pub fun fetch(_ from: Address, itemID: UInt64): &RoxContract.NFT? {
        let collection = getAccount(from)
            .getCapability(RoxContract.CollectionPublicPath)
            .borrow<&RoxContract.Collection{RoxContract.RoxCollectionPublic}>()
            ?? panic("Couldn't get collection")
        return collection.borrowRoxNft(id: itemID)
    }

    pub resource Admin {

        // Mints a new box
        pub fun mintBox(name: String, metadata: {String: String}) {
            var newBox <- create Box(name: name, metadata: metadata)
            RoxContract.boxes[newBox.boxId] <-! newBox
        }

        // In order to mint NFT, box reference should be received
        // All the NFTs are minted via unlocked box
        pub fun borrowBox(boxId: UInt32): &Box {
            pre {
                RoxContract.boxes[boxId] != nil: "Cannot borrow Box: The Box doesn't exist"
            }
            
            return &RoxContract.boxes[boxId] as &Box
        }

        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    pub struct BoxData {
        pub let boxId: UInt32
        pub let name: String
        pub let metadata: {String: String}
        pub var locked: Bool
        pub var mintedNumberPerRox: {String: UInt32}

        init(boxId: UInt32) {
            pre {
                RoxContract.boxes[boxId] != nil: "Box does not exist"
            }

            self.boxId = RoxContract.boxes[boxId]?.boxId!
            self.name = RoxContract.boxes[boxId]?.name!
            self.metadata = RoxContract.boxes[boxId]?.metadata!
            self.locked = RoxContract.boxes[boxId]?.locked!
            self.mintedNumberPerRox = RoxContract.boxes[boxId]?.mintedNumberPerRox!
        }
    }

    // -----------------------------------------------------------------------
    // RoxContract initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        self.CollectionStoragePath = /storage/RoxCollection
        self.CollectionPublicPath = /public/RoxCollection
        self.AdminStoragePath = /storage/RoxAdmin

        self.totalSupply = 0
        self.nextBoxId = 1
        self.boxes <- {}
        self.mintedNumberPerBox = {}

        let collection <- RoxContract.createEmptyCollection()
        self.account.save(<-collection, to: RoxContract.CollectionStoragePath)
        self.account.link<&RoxContract.Collection{NonFungibleToken.CollectionPublic, RoxContract.RoxCollectionPublic}>(RoxContract.CollectionPublicPath, target: RoxContract.CollectionStoragePath)

        let admin <- create Admin()
        self.account.save(<-admin, to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}