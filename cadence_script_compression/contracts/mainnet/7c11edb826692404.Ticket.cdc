import NonFungibleToken from 0x1d7e57aa55817448

pub contract Ticket: NonFungibleToken {
    pub var totalSupply: UInt64

    pub event ContractInitialized()
    pub event Minted(to: Address, tokenId: UInt64, level: UInt8, issuePrice: UFix64)
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event TicketDestroyed(id: UInt64)

    pub enum Level: UInt8 {
        pub case One
        pub case Two
        pub case Three
    }

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let issuePrice: UFix64
        pub let level: UInt8

        init(
            id: UInt64,
            issuePrice: UFix64,
            level: Level
        ) {
            self.id = id
            self.issuePrice = issuePrice
            self.level = level.rawValue
        } 

        destroy() {
            emit TicketDestroyed(id: self.id)
        }
    }

    pub resource interface TicketCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowTicket(id: UInt64): &Ticket.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Ticket reference: the ID of the returned reference is incorrect"
            }
        }
        pub fun getCounts(): {UInt8: UInt64}
    }

    pub resource Collection: TicketCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
        pub var counts: {UInt8: UInt64}

        init () {
            self.ownedNFTs <- {}
            self.counts = {}
        }

        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let level = self.borrowTicket(id: withdrawID)?.level ?? panic("Level missing")
            self.counts[level] = self.counts[level]! - 1

            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")
            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @Ticket.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            self.counts[token.level] = (self.counts[token.level] ?? 0) + 1
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }
 
        pub fun borrowTicket(id: UInt64): &Ticket.NFT? {
            if self.ownedNFTs[id] != nil {
                // Create an authorized reference to allow downcasting
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Ticket.NFT
            }

            return nil
        }

        pub fun getCounts(): {UInt8: UInt64} {
            return  self.counts
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    //
    pub resource NFTMinter {

        // mintNFT mints a new NFT with a new ID
        // and deposit it in the recipients collection using their collection reference
        pub fun mintNFT(
            recipient: &{NonFungibleToken.CollectionPublic},
            issuePrice: UFix64,
            level: Level
        ) {

            // create a new NFT
            var newNFT <- create NFT(
                id: Ticket.totalSupply + 1,
                issuePrice: issuePrice,
                level: level
            )

            let id = newNFT.id

            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-newNFT)

            Ticket.totalSupply = Ticket.totalSupply + 1

            emit Minted(to: recipient.owner!.address, tokenId: id, level: level.rawValue, issuePrice: issuePrice)
        }
    }

    init() {
        // Initialize the total supply
        self.totalSupply = 0

        // Set the named paths
        self.CollectionStoragePath = /storage/BNMUTicketNFTCollection
        self.CollectionPublicPath = /public/BNMUTicketNFTCollection
        self.MinterStoragePath = /storage/BNMUTicketNFTMinter

        // Create a Collection resource and save it to storage
        let collection <- create Collection()
        self.account.save(<-collection, to: self.CollectionStoragePath)

        // create a public capability for the collection
        self.account.link<&Ticket.Collection{NonFungibleToken.CollectionPublic, Ticket.TicketCollectionPublic}>(
            self.CollectionPublicPath,
            target: self.CollectionStoragePath
        )

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}