import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract ExperiaNFTContract: NonFungibleToken {

    pub event ContractInitialized()

    pub var ExperiaCollectionPublic: PublicPath
    // Initialize the total supply
    pub var totalSupply: UInt64
    //total collection created
    pub var totalCollection: UInt64
    /* withdraw event */
    pub event Withdraw(id: UInt64, from: Address?)
    /* Event that is issued when an NFT is deposited */
    pub event Deposit(id: UInt64, to: Address?)
    /* event that is emitted when a new collection is created */
    pub event NewCollection(collectionName: String, collectionID:UInt64)
    /* Event that is emitted when new NFTs are cradled*/
    pub event NewNFTsminted(amount: UInt64)
    /* Event that is emitted when an NFT collection is created */
    pub event CreateNFTCollection(amount: UInt64, maxNFTs: UInt64)
    /* Event that returns how many IDs a collection has */ 
    pub event TotalsIDs(ids:[UInt64])

    pub let MinterStoragePath: StoragePath
    /* ## ~~This is the contract where we manage the flow of our collections and NFTs~~  ## */

    /* 
    Through the contract you can find variables such as Metadata,
    which are no longer a name to refer to the attributes of our NFTs. 
    which could be the url where our images live
    */

    //In this section you will find our variables and fields for our NFTs and Collections
    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
    // The unique ID that each NFT has
        pub let id: UInt64

        access(self) var metadata : {String: AnyStruct}
        pub let name: String
        pub let description: String
        pub let thumbnail: String

        init(id : UInt64, name: String, metadata: {String:AnyStruct}, thumbnail: String, description: String) {
            self.id = id
            self.metadata = metadata
            self.name = name
            self.thumbnail = thumbnail
            self.description = description
        }
        
        pub fun getMetadata(): {String: AnyStruct} {
        return self.metadata
        }

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>()
            ]
        }

         pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: self.name,
                        description: self.description,
                        thumbnail: MetadataViews.IPFSFile(
                            cid: self.thumbnail, 
                            path: "sm.png"
                        )
                    )
            }
            return nil
        }

    }

    // We define this interface purely as a way to allow users
    // They would use this to only expose getIDs
    // borrowGMDYNFT
    // and idExists fields in their Collection
    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun idExists(id: UInt64): Bool
        pub fun getRefNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowExperiaNFT(id: UInt64): &ExperiaNFTContract.NFT? {
            post {
                (result == nil) || (result?.id == id):
                "Cannot borrow NFT reference: the ID of the returned reference is incorrect"
            }
        }
    }

    // We define this interface simply as a way to allow users to
    // to create a banner of the collections with their Name and Metadata
    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
        pub var metadata: {String: AnyStruct}

        pub var name: String

        init (name: String, metadata: {String: AnyStruct}) {
            self.ownedNFTs <- {}
            self.name = name
            self.metadata = metadata
        }

         /* Function to remove the NFt from the Collection */
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            // If the NFT isn't found, the transaction panics and reverts
            let exist = self.idExists(id: withdrawID)
            if exist == false {
                    panic("id NFT Not exist")
            }
           let token <- self.ownedNFTs.remove(key: withdrawID)!

             /* Emit event when a common user withdraws an NFT*/
            emit Withdraw(id:withdrawID, from: self.owner?.address)

           return <-token
        }

        /*Function to deposit a  NFT in the collection*/
        pub fun deposit(token: @NonFungibleToken.NFT) {

            let token <- token as! @ExperiaNFTContract.NFT

            let id: UInt64 = token.id
            
            self.ownedNFTs[token.id] <-! token

            emit Deposit(id: id, to: self.owner?.address )
        }

        //fun get IDs nft
        pub fun getIDs(): [UInt64] {

            emit TotalsIDs(ids: self.ownedNFTs.keys)
            
            return self.ownedNFTs.keys
        }

        /*Function get Ref NFT*/
        pub fun getRefNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        /*Function borrow NFT*/
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            if self.ownedNFTs[id] != nil {
                return &self.ownedNFTs[id] as &NonFungibleToken.NFT     
            }
            panic("not found NFT")
        }

        pub fun borrowExperiaNFT(id: UInt64): &ExperiaNFTContract.NFT? {
            if self.ownedNFTs[id] != nil {
                // Create an authorized reference to allow downcasting
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &ExperiaNFTContract.NFT
            }
          panic("not found NFT")
        }

        // fun to check if the NFT exists
        pub fun idExists(id: UInt64): Bool {
            return self.ownedNFTs[id] != nil
        }
        
        destroy () {
            destroy self.ownedNFTs
        }
    }


    // We define this interface simply as a way to allow users to
    // to add the first NFTs to an empty collection.
    pub resource interface NFTCollectionReceiver {
      pub fun generateNFT(amount: UInt64, collection: Capability<&ExperiaNFTContract.Collection{NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic}>)
      pub fun getQuantityAvailablesForCreate(): Int
    }

    pub resource NFTTemplate: NFTCollectionReceiver {

        priv var metadata : {String: AnyStruct}
        // array NFT
        priv var collectionNFT : [UInt64]
        priv var counteriDs: [UInt64]
        pub let name : String
        pub let thumbnail:  String
        pub let description: String
        pub let maximum : UInt64
 
        init(name: String,  metadata: {String: AnyStruct}, thumbnail: String, description: String, amountToCreate: UInt64, maximum: UInt64 collection: Capability<&ExperiaNFTContract.Collection{NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic}>) { 
            self.metadata = metadata
            self.name = name
            self.maximum = maximum
            self.thumbnail = thumbnail
            self.description = description
            self.collectionNFT = []
            self.counteriDs = []
            self.generateNFT(amount: amountToCreate, collection: collection)
        }
        
        pub fun generateNFT(amount: UInt64, collection: Capability<&ExperiaNFTContract.Collection{NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic}>) {
            if Int(amount) < 0 {
                panic("Error amount should be greather than 0")
            }
            if amount > self.maximum {
                panic("Error amount is greater than maximun")
            }
            let newTotal = Int(amount) + self.collectionNFT.length
            if newTotal > Int(self.maximum) {
                panic("The collection is already complete or The amount of nft sent exceeds the maximum amount")
            }
            
            var i = 0  
            let collectionBorrow = collection.borrow() ?? panic("cannot borrow collection")
              emit NewNFTsminted(amount: amount)
            while i < Int(amount) {
             ExperiaNFTContract.totalSupply = ExperiaNFTContract.totalSupply + 1 
                let newNFT <- create NFT(id: ExperiaNFTContract.totalSupply, name: self.name, metadata: self.metadata, thumbnail: self.thumbnail, description: self.description)
                collectionBorrow.deposit(token: <- newNFT)
                self.collectionNFT.append(ExperiaNFTContract.totalSupply)
                self.counteriDs.append(ExperiaNFTContract.totalSupply)
                i = i + 1
            }
            emit TotalsIDs(ids: self.counteriDs)
            self.counteriDs = []
        }
    
        pub fun getQuantityAvailablesForCreate(): Int {
            return Int(self.maximum) - self.collectionNFT.length
        }
    }

    pub resource NFTMinter {

         pub fun createNFTTemplate(name: String,
                                metadata: {String: AnyStruct}, thumbnail: String, 
                                description: String, amountToCreate: UInt64, 
                                maximum: UInt64,  collection: Capability<&ExperiaNFTContract.Collection{NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic}>
                                ): @NFTTemplate {
      emit  CreateNFTCollection(amount: amountToCreate, maxNFTs: maximum)
        return <- create NFTTemplate(
            name: name, 
            metadata: metadata, 
            thumbnail: thumbnail,
            description:  description,
            amountToCreate: amountToCreate, 
            maximum: maximum,
            collection: collection,
        )
    }

    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection(name: "", metadata: {})
    }

    pub fun createEmptyCollectionNFT(name: String, metadata: {String:AnyStruct}): @NonFungibleToken.Collection {
        var newID = ExperiaNFTContract.totalCollection + 1
        ExperiaNFTContract.totalCollection = newID
        emit NewCollection(collectionName: name, collectionID: ExperiaNFTContract.totalCollection)
        return <-  create Collection(name: name, metadata: metadata)
    }

    init() {
        // Initialize the total supply
        self.totalSupply = 0
        // Initialize the total collection
        self.totalCollection = 0

        self.MinterStoragePath = /storage/experiaMinterV1

        self.ExperiaCollectionPublic = /public/ExperiaCollectionPublic

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<- minter, to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}