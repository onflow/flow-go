import NonFungibleToken from 0x1d7e57aa55817448

pub contract FanfareNFTContract: NonFungibleToken {

    pub var totalSupply: UInt64

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, templateID: UInt64, creatorAddress: Address, recipient: Address, metadata: String)

    // Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    
    //Contract Owner ContentCreator Resouce 
    pub let ContentCreatorStoragePath: StoragePath
    pub let ContentCreatorPrivatePath: PrivatePath
    pub let ContentCreatorPublicPath: PublicPath 

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let templateID: UInt64
        pub var creatorAddress: Address
        pub var metadata: String

        init(initID: UInt64, templateID: UInt64, creatorAddress: Address, metadata: String) {
            self.id = initID
            self.templateID = templateID
            self.creatorAddress = creatorAddress
            self.metadata = metadata
        }
    }

    pub resource interface FanfareNFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT) 
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowNFTMetadata(id: UInt64): &FanfareNFTContract.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Card reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, FanfareNFTCollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @FanfareNFTContract.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
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

        // borrowNFTMetadata gets a reference to an NFT in the collection
        // so that the caller can read its id and metadata
        pub fun borrowNFTMetadata(id: UInt64): &FanfareNFTContract.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &FanfareNFTContract.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // public function that anyone can call to create a new empty collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    pub resource ContentCreator {
        pub var idCount: UInt64

        init(){
            self.idCount = 1
        }
      
        pub fun mintNFT(creatorAddress: Address, recipient: Address, templateID: UInt64, metadata: String): UInt64 {
            let token: @NFT <- create NFT(
                initID: self.idCount,
                templateID: templateID,
                creatorAddress: creatorAddress,
                metadata: metadata
            )
            let id: UInt64 = self.idCount
            self.idCount = self.idCount + 1
            FanfareNFTContract.totalSupply = FanfareNFTContract.totalSupply + 1

            var receiver = getAccount(recipient).getCapability<&{FanfareNFTCollectionPublic}>(FanfareNFTContract.CollectionPublicPath)
            let account = receiver.borrow()!
            account.deposit(token: <- token)

            emit Minted(id: id, templateID: templateID, creatorAddress: creatorAddress, recipient: recipient, metadata: metadata)
            return id
        }

    }

    init() {
        self.CollectionStoragePath = /storage/FanfareNFTCollection
        self.CollectionPublicPath = /public/FanfareNFTCollection

        self.ContentCreatorStoragePath = /storage/FanfareContentStorage
        self.ContentCreatorPrivatePath = /private/FanfareContentStorage
        self.ContentCreatorPublicPath = /public/FanfareContentStorage

        // Initialize the total supply
        self.totalSupply = 0

        self.account.save(<-create ContentCreator(), to: self.ContentCreatorStoragePath)
        
        self.account.link<&{FanfareNFTContract.FanfareNFTCollectionPublic}>(/public/FanfareNFTCollection, target: /storage/FanfareNFTCollection)

        emit ContractInitialized()        
    }
}