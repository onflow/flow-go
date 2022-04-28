import NonFungibleToken from 0x1d7e57aa55817448

pub contract MotoGPPack: NonFungibleToken {

    pub fun getVersion(): String {
        return "0.7.8"
    }

    // The total number of Packs that have been minted
    pub var totalSupply: UInt64

    // An dictionary of all the pack types that have been
    // created using addPackType method defined 
    // in the MotoGPAdmin contract
    //
    // It maps the packType # to the 
    // information of that pack type
    access(account) var packTypes: {UInt64: PackType}

    pub event Mint(id: UInt64)

    pub event Burn(id: UInt64)

    // Event that emitted when the MotoGPPack contract is initialized
    //
    pub event ContractInitialized()

    // Event that is emitted when a Pack is withdrawn,
    // indicating the owner of the collection that it was withdrawn from.
    //
    // If the collection is not in an account's storage, `from` will be `nil`.
    //
    pub event Withdraw(id: UInt64, from: Address?)

    // Event that emitted when a Pack is deposited to a collection.
    //
    // It indicates the owner of the collection that it was deposited to.
    //
    pub event Deposit(id: UInt64, to: Address?)

    // PackInfo
    // a struct that represents the info of a Pack.  
    // It is conveniant to have a struct because we 
    // can easily read from it without having to move
    // resources around. Each PackInfo holds a 
    // packNumber and packType
    //
    pub struct PackInfo {
        // The pack # of this pack type
        pub var packNumber: UInt64

        // An ID that represents that type of pack this is
        pub var packType: UInt64

        init(_packNumber: UInt64, _packType: UInt64) {
            self.packNumber = _packNumber
            self.packType = _packType
        }
    }

    // NFT
    // This resource defines a Pack with a PackInfo
    // struct.
    // After a Pack is opened, the Pack is destroyed and its id
    // can never be created again. 
    //
    // The NFT (Pack) resource is arbitrary in the sense that it doesn't
    // actually hold Cards inside. However, by looking at a Pack's
    // packType and mapping that to the # of Cards in a Pack,
    // we will use that info when opening the Pack.
    //
    pub resource NFT: NonFungibleToken.INFT {
        // The Pack's id (completely sequential)
        pub let id: UInt64

        pub var packInfo: PackInfo

        init(_packNumber: UInt64, _packType: UInt64) {
            let packTypeStruct: &PackType = &MotoGPPack.packTypes[_packType] as &PackType
            // updates the number of packs minted for this packType
            packTypeStruct.updateNumberOfPacksMinted()
            
            // will panic if this packNumber already exists in this packType
            packTypeStruct.addPackNumber(packNumber: _packNumber)

            self.id = MotoGPPack.totalSupply
            self.packInfo = PackInfo(_packNumber: _packNumber, _packType: _packType)

            MotoGPPack.totalSupply = MotoGPPack.totalSupply + (1 as UInt64)

            emit Mint(id: self.id)
        }

        destroy(){
            emit Burn(id: self.id)
        }
    }

    // createPack
    // allows us to create Packs from another contract
    // in this account. This is helpful for 
    // allowing MotoGPAdmin to mint Packs.
    //
    access(account) fun createPack(packNumber: UInt64, packType: UInt64): @NFT {
        return <- create NFT(_packNumber: packNumber, _packType: packType)
    }

    // PackType
    // holds all the information for
    // a specific pack type
    //
    pub struct PackType {

        pub let packType: UInt64

        // the number of packs of this pack type
        // that have been minted
        pub var numberOfPacksMinted: UInt64

        // the number of cards that will be minted
        // when a pack of this pack type is opened
        pub let numberOfCards: UInt64

        // the pack numbers that already exist of this
        // pack type.
        // This is primarily used so we don't create duplicate
        // Packs with the same packNumber and packType
        pub var assignedPackNumbers: {UInt64: Bool}

        // updateNumberOfPacksMinted
        // updates the number of Packs that have
        // been minted for this specific pack type
        //
        access(contract) fun updateNumberOfPacksMinted() {
            self.numberOfPacksMinted = self.numberOfPacksMinted + (1 as UInt64)
        }

        // addPackNumber
        // updates the amount of packNumbers that
        // belong to this PackType so we do not
        // make duplicates when minting a Pack
        //
        access(contract) fun addPackNumber(packNumber: UInt64) {
            if let assignedPackNumbers = self.assignedPackNumbers[packNumber] {
                panic("The following pack number already exists: ".concat(packNumber.toString()))
            } else {
                self.assignedPackNumbers[packNumber] = true
            }
        }

        init(_packType: UInt64, _numberOfCards: UInt64) {
            self.packType = _packType
            self.numberOfCards = _numberOfCards
            self.numberOfPacksMinted = 0
            self.assignedPackNumbers = {}
        }
    }

    // IPackCollectionPublic
    // This defines an interface to only expose a
    // Collection of Packs to the public
    //
    // Allows users to deposit Packs, read all the Pack IDs,
    // borrow a NFT reference to access a Pack's ID,
    // and borrow a Pack reference
    // to access all of the Pack's fields.
    //
    pub resource interface IPackCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT

        pub fun borrowPack(id: UInt64): &MotoGPPack.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Pack reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // IPackCollectionAdminAccessible
    // Exposes the openPack which allows an admin to
    // open a pack in this collection.
    //
    pub resource interface IPackCollectionAdminAccessible {
        //Keep for now for compatibility purposes.
    }

    // Collection
    // a collection of Pack resources so that users can
    // own Packs in a collection and trade them back and forth.
    //
    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, IPackCollectionPublic, IPackCollectionAdminAccessible  {
        // A dictionary that maps ids to the pack with that id
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        // deposit
        // deposits a Pack into the users Collection
        //
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @MotoGPPack.NFT

            let id: UInt64 = token.id

            // add the new Pack to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            // Only emit a deposit event if the Collection 
            // is in an account's storage
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            destroy oldToken
        }

        // withdraw
        // withdraws a Pack from the collection
        //
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // getIDs
        // returns the ids of all the Packs this
        // collection has
        // 
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT
        // borrowNFT gets a reference to an NFT in the collection
        // so that the caller can read its id
        //
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowPack
        // borrowPack returns a borrowed reference to a Pack
        // so that the caller can read data from it.
        // They can use this to read its PackInfo and id
        //
        pub fun borrowPack(id: UInt64): &MotoGPPack.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &MotoGPPack.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }

        init() {   
            self.ownedNFTs <- {}
        }
    }

    // createEmptyCollection
    // creates an empty Collection so that users
    // can be able to receive and deal with Pack resources.
    //
    pub fun createEmptyCollection(): @Collection {
        return <- create Collection()
    }

    // getPackTypeInfo
    // returns a PackType struct, which represents the info
    // of a specific PackType that is passed in
    //
    pub fun getPackTypeInfo(packType: UInt64): PackType {
        return self.packTypes[packType] ?? panic("This pack type does not exist")
    }

    init() {
        self.totalSupply = 0
        self.packTypes = {}

        emit ContractInitialized()
    }
}
