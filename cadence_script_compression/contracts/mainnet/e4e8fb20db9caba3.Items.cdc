
import NonFungibleToken from 0x1d7e57aa55817448

pub contract Items: NonFungibleToken {

    // -----------------------------------------------------------------------
    // Public named paths
    // -----------------------------------------------------------------------
    pub let CollectionStoragePath: StoragePath
    pub let ItemsAdminStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // -----------------------------------------------------------------------
    // Public events
    // -----------------------------------------------------------------------

    // Emitted when the Items contract is created
    pub event ContractInitialized()

    // Emitted when a new Artist struct is created
    pub event ArtistCreated(id: UInt32, metadata: {String:String})

    // Emitted when a new Piece is created
    pub event PieceCreated(pieceID: UInt32, artistID: UInt32, metadata: {String:String})

    // Emitted when a Piece is locked, meaning Piece cannot be added
    pub event PieceLocked(pieceID: UInt32)

    // Emitted when a Items is minted from a Piece
    pub event ItemsMinted(itemID: UInt64, artistID: UInt32, pieceID: UInt32, serialNumber: UInt32)

    // Emitted when a item is withdrawn from a Collection
    pub event Withdraw(id: UInt64, from: Address?)

    // Emitted when a item is deposited into a Collection
    pub event Deposit(id: UInt64, to: Address?)

    // Emitted when a Items is destroyed
    pub event ItemsDestroyed(id: UInt64)

    // -----------------------------------------------------------------------
    // Items contract-level fields.
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------

    // Variable size dictionary of artist structs
    access(self) var artistDatas: {UInt32: Artist}

    // Variable size dictionary of piece structs
    access(self) var pieceDatas: {UInt32: PieceData}

    // Variable size dictionary of Piece resources
    access(self) var pieces: @{UInt32: Piece}

    // The ID that is used to create Artists. 
    pub var nextArtistID: UInt32

    // The ID that is used to create Pieces. 
    pub var nextPieceID: UInt32

    // The total number of Items NFTs that have been created
    pub var totalSupply: UInt64

    
    pub struct Artist {

        // The unique ID for the Artist
        pub let artistID: UInt32

        // Stores all the metadata about the artist as a string mapping
        pub let metadata: {String: String}

        init(metadata: {String: String}) {
            pre {
                metadata.length != 0: "New Play metadata cannot be empty"
            }
            self.artistID = Items.nextArtistID
            self.metadata = metadata
        }
    }

    pub struct PieceData {

        // The unique ID for the Piece
        pub let pieceID: UInt32

        // Stores all the metadata about the piece as a string mapping
        // name could be a key in metadata
        pub let metadata: {String: String} 

        init(metadata: {String: String}) {
            pre {
                metadata.length != 0: "New Piece metadata cannot be empty"
            }
            self.pieceID = Items.nextPieceID
            self.metadata = metadata
        }
    }

    pub resource Piece {

        // Unique ID for the piece
        pub let pieceID: UInt32

        // The Artist who created the piece
        access(contract) var artistID: UInt32

        pub var locked: Bool

        // The number of Itemss minted by using this Piece
        access(contract) var numberMinted: UInt32 // serial number

        init(artistID: UInt32, metadata: {String: String}) {
            self.pieceID = Items.nextPieceID
            self.artistID = artistID
            self.locked = false
            self.numberMinted = 0
            // Create a new PieceData for this Set Piece store it in contract storage
            Items.pieceDatas[self.pieceID] = PieceData(metadata: metadata)
        }


        pub fun lock() {
            if !self.locked {
                self.locked = true
                emit PieceLocked(pieceID: self.pieceID)
            }
        }

        pub fun mintItems(): @NFT {
            
            // Mint the new item
            let newItems: @NFT <- create NFT(serialNumber: self.numberMinted + (1 as UInt32),
                                              artistID: self.artistID,
                                              pieceID: self.pieceID)

            // Increment the count of Itemss minted for this Piece
            self.numberMinted = self.numberMinted + (1 as UInt32)
            emit ItemsMinted(itemID: newItems.id, artistID: self.artistID, pieceID: self.pieceID, serialNumber:self.numberMinted)
            return <-newItems
        }

       
        pub fun batchMintItems(quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintItems())
                i = i + (1 as UInt64)
            }

            return <-newCollection
        }

        pub fun getArtist(): UInt32 {
            return self.artistID
        }

        pub fun getNumMinted(): UInt32 {
            return self.numberMinted
        }
    }

    pub struct QueryPieceData {
        pub let pieceID: UInt32
        access(self) var artistID: UInt32
        pub var locked: Bool
        access(self) var numberMinted: UInt32

        pub let metadata: {String: String}

        init(pieceID: UInt32) {

            let piece = &Items.pieces[pieceID] as &Piece
            let pieceData = Items.pieceDatas[pieceID] as PieceData?

            self.pieceID = pieceID
            self.metadata = pieceData!.metadata
            self.artistID = piece.artistID
            self.locked = piece.locked
            self.numberMinted = piece.numberMinted
        }

        pub fun getArtist(): UInt32 {
            return self.artistID
        }

        pub fun getNumberMinted(): UInt32 {
            return self.numberMinted
        }
    }


    pub struct ItemsData {

        // The ID of the Piece that the Items comes from
        pub let pieceID: UInt32
        // The ID of the Artist that the Items comes from
        pub let artistID: UInt32

        pub let serialNumber: UInt32

        init(pieceID: UInt32, artistID: UInt32, serialNumber: UInt32) {
            self.pieceID = pieceID
            self.artistID = artistID
            self.serialNumber = serialNumber
        }

    }

    // total supply id is global
    pub resource NFT: NonFungibleToken.INFT {

        // Global unique Items ID
        pub let id: UInt64
        
        // Struct of Items metadata
        pub let data: ItemsData

        init(serialNumber: UInt32, artistID: UInt32, pieceID: UInt32) {
            // Increment the global Items IDs
            Items.totalSupply = Items.totalSupply + (1 as UInt64)

            self.id = Items.totalSupply

            // piece the metadata struct
            self.data = ItemsData(pieceID: pieceID, artistID: artistID, serialNumber: serialNumber)
        }

        // If the Items is destroyed, emit an event to indicate 
        // to outside ovbservers that it has been destroyed
        destroy() {
            emit ItemsDestroyed(id: self.id)
        }
    }


    pub resource Admin {

        pub fun createArtist(metadata: {String: String}): UInt32 {
            // Create the new Artist
            var newArtist = Artist(metadata: metadata)
            let newID = newArtist.artistID

            // Increment the ID so that it isn't used again
            Items.nextArtistID = Items.nextArtistID + (1 as UInt32)

            emit ArtistCreated(id: newArtist.artistID, metadata: metadata)

            // Store it in the Artists mapping field
            Items.artistDatas[newID] = newArtist

            return newID
        }

        pub fun createPiece(artistID: UInt32, metadata: {String: String}): UInt32 {

            // Create the new Set
            var newPiece <- create Piece(artistID:artistID, metadata: metadata)

            // Increment the PieceID so that it isn't used again
            Items.nextPieceID = Items.nextPieceID + (1 as UInt32)

            let newID = newPiece.pieceID

            emit PieceCreated(pieceID: newID, artistID: artistID, metadata: metadata)

            // Store it in the Pieces mapping field
            Items.pieces[newID] <-! newPiece

            return newID
        }

        pub fun borrowPiece(pieceID: UInt32): &Piece {
            pre {
                Items.pieces[pieceID] != nil: "Cannot borrow piece: The piece doesn't exist"
            }
            
            // Get a reference to the pice and return it
            // use `&` to indicate the reference to the object and type
            return &Items.pieces[pieceID] as &Piece
        }

        // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    pub resource interface ItemsCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowItems(id: UInt64): &Items.NFT? {
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Items reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: ItemsCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic { 
        // Dictionary of Items conforming tokens
        // NFT is a resource type with a UInt64 ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an Items from the Collection and moves it to the caller
        //
        // Parameters: withdrawID: The ID of the NFT 
        // that is to be removed from the Collection
        //
        // returns: @NonFungibleToken.NFT the token that was withdrawn
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Items does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }

        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn Itemss
        //
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            // Create a new empty Collection
            var batchCollection <- create Collection()
            
            // Iterate through the ids and withdraw them from the Collection
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            
            // Return the withdrawn tokens
            return <-batchCollection
        }

        // deposit takes a Moment and adds it to the Collections dictionary
        //
        // Paramters: token: the NFT to be deposited in the collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            
            // Cast the deposited token as a Items NFT to make sure
            // it is the correct type
            let token <- token as! @Items.NFT

            // Get the token's ID
            let id = token.id

            // Add the new token to the dictionary
            let oldToken <- self.ownedNFTs[id] <- token

            // Only emit a deposit event if the Collection 
            // is in an account's storage
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            // Destroy the empty old token that was "removed"
            destroy oldToken
        }

        // batchDeposit takes a Collection object as an argument
        // and deposits each contained NFT into this Collection
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {

            // Get an array of the IDs to be deposited
            let keys = tokens.getIDs()

            // Iterate through the keys in the collection and deposit each one
            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }

            // Destroy the empty Collection
            destroy tokens
        }

        // getIDs returns an array of the IDs that are in the Collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT Returns a borrowed reference to an Items in the Collection
        // so that the caller can read its ID
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowItems returns a borrowed reference to an Items
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowItems(id: UInt64): &Items.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Items.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }
    
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create Items.Collection()
    }

   
    pub fun getAllArtists(): [Items.Artist] {
        return Items.artistDatas.values
    }

    
    pub fun getArtistMetaData(artistID: UInt32): {String: String}? {
        return Items.artistDatas[artistID]?.metadata
    }


    pub fun getPieceData(pieceID: UInt32): QueryPieceData? {
        if Items.pieces[pieceID] == nil {
            return nil
        } else {
            return QueryPieceData(pieceID: pieceID)
        }
    }

    pub fun isPieceLocked(pieceID: UInt32): Bool? {
        // Don't force a revert if the pieceID is invalid
        return Items.pieces[pieceID]?.locked
    }

    // -----------------------------------------------------------------------
    // Items initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        self.CollectionStoragePath = /storage/ItemsEcosystemCollection
        self.ItemsAdminStoragePath = /storage/ItemsEcosystemAdmin
        self.CollectionPublicPath = /public/ItemsEcosystemCollection

        self.artistDatas = {}
        self.pieceDatas = {}
        self.pieces <- {}
        self.nextArtistID = 1
        self.nextPieceID = 1
        self.totalSupply = 0 // will be itemID

        // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: self.CollectionStoragePath)

        // Create a public capability for the Collection
        self.account.link<&{ItemsCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath)

        // Put the Admin in storage
        self.account.save<@Admin>(<- create Admin(), to: self.ItemsAdminStoragePath)

        emit ContractInitialized()
    }
}
 
