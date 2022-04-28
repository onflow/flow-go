import NonFungibleToken from 0x1d7e57aa55817448
import MotoGPAdmin from 0xa49cc0ee46c54bfb
import MotoGPCardMetadata from 0xa49cc0ee46c54bfb
import MotoGPCounter from 0xa49cc0ee46c54bfb

pub contract MotoGPCard: NonFungibleToken {

    pub fun getVersion(): String {
        return "0.7.8"
    }

    // The total number of Cards in existence
    pub var totalSupply: UInt64

    // Event that emitted when the MotoGPCard contract is initialized
    //
    pub event ContractInitialized()

    // Event that is emitted when a Card is withdrawn,
    // indicating the owner of the collection that it was withdrawn from.
    //
    // If the collection is not in an account's storage, `from` will be `nil`.
    //
    pub event Withdraw(id: UInt64, from: Address?)

    // Event that emitted when a Card is deposited to a collection.
    //
    // It indicates the owner of the collection that it was deposited to.
    //
    pub event Deposit(id: UInt64, to: Address?)


    pub event Mint(id: UInt64)

    pub event Burn(id: UInt64)

    // An array UInt128s that represents cardID + serial keys
    // This is primarily used to ensure there
    // are no duplicates when we mint a new Card (NFT)
   access(account) var allCards: [UInt128]

    // NFT
    // The NFT resource defines a specific Card
    // that has an id, cardID, and serial.
    // This resource will be created every time
    // a pack opens and when we need to mint Cards.
    //
    // This NFT represents a Card. These names
    // will be used interchangeably.
    //

    pub resource NFT: NonFungibleToken.INFT {
        // The card's Issue ID (completely sequential)
        pub let id: UInt64

        // The card's cardID, which will be determined
        // once the pack is opened
        pub let cardID: UInt64

        // The card's Serial, which will also be determined
        // once the pack is opened
        pub let serial: UInt64

        // initializer
        //
        init(_cardID: UInt64, _serial: UInt64) {
            let keyValue = UInt128(_cardID) + (UInt128(_serial) * (0x4000000000000000 as UInt128))
            if (MotoGPCard.allCards.contains(keyValue)) {
                panic("This cardID and serial combination already exists")
            }
            MotoGPCard.allCards.append(keyValue)

            self.cardID = _cardID
            self.serial = _serial

            self.id = MotoGPCounter.increment("MotoGPCard")

            MotoGPCard.totalSupply = MotoGPCard.totalSupply + (1 as UInt64)

            emit Mint(id: self.id)
        }

        pub fun getCardMetadata(): MotoGPCardMetadata.Metadata? {
            return MotoGPCardMetadata.getMetadataForCardID(cardID: self.cardID)
        }

        destroy(){
            MotoGPCard.totalSupply = MotoGPCard.totalSupply - (1 as UInt64)
            emit Burn(id: self.id)
        }
    }

    // createNFT
    // allows us to create an NFT from another contract in this
    // account because we want to be able to mint NFTs
    // in MotoGPPack
    //
    access(account) fun createNFT(cardID: UInt64, serial: UInt64): @NFT {
        return <- create NFT(_cardID: cardID, _serial: serial)
    }

    // ICardCollectionPublic
    // Defines an interface that specifies
    // what fields and functions 
    // we want to expose to the public.
    //
    // Allows users to deposit Cards, read all the Card IDs,
    // borrow a NFT reference to access a Card's ID,
    // deposit a batch of Cards, and borrow a Card reference
    // to access all of the Card's fields.
    //
    pub resource interface ICardCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT

        pub fun depositBatch(cardCollection: @NonFungibleToken.Collection)
        pub fun borrowCard(id: UInt64): &MotoGPCard.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Card reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection
    // This resource defines a collection of Cards
    // that a user will have. We must give each user
    // this collection so they can
    // interact with Cards.
    //
    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, ICardCollectionPublic {
        // A dictionary that maps a Card's id to a 
        // Card in the collection. It holds all the 
        // Cards in a collection.
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // deposit
        // deposits a Card into the Collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @MotoGPCard.NFT

            let id: UInt64 = token.id

            // add the new Card to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            // Only emit a deposit event if the Collection 
            // is in an account's storage
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            destroy oldToken
        }

        // depositBatch
        // This method deposits a collection of Cards into the
        // user's Collection.
        //
        // This is primarily called by an Admin to
        // deposit newly minted Cards into this Collection.
        //
        pub fun depositBatch(cardCollection: @NonFungibleToken.Collection) {
            pre {
                cardCollection.getIDs().length <= 100:
                    "Too many cards being deposited. Must be less than or equal to 100"
            }

            // Get an array of the IDs to be deposited
            let keys = cardCollection.getIDs()

            // Iterate through the keys in the collection and deposit each one
            for key in keys {
                self.deposit(token: <-cardCollection.withdraw(withdrawID: key))
            }

            // Destroy the empty Collection
            destroy cardCollection
        }

        // withdraw
        // withdraw removes a Card from the collection and moves it to the caller
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // getIDs
        // returns the ids of all the Card this
        // collection has
        // 
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT
        // borrowNFT gets a reference to an NFT in the collection
        // so that the caller can read its id field
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowCard
        // borrowCard returns a borrowed reference to a Card
        // so that the caller can read data from it.
        // They can use this to read its id, cardID, and serial
        //
        pub fun borrowCard(id: UInt64): &MotoGPCard.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &MotoGPCard.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }

        init () {
            self.ownedNFTs <- {}
        }
    }

    // createEmptyCollection
    // creates a new Collection resource and returns it.
    // This will primarily be used to give a user a new Collection
    // so they can store Cards
    //
    pub fun createEmptyCollection(): @Collection {
        return <- create Collection()
    }

    init() {
        self.totalSupply = 0
        self.allCards = []

        emit ContractInitialized()
    }

}