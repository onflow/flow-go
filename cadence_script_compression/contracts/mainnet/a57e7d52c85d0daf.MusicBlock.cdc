// This is an example implementation of a Flow Non-Fungible Token
// It is not part of the official standard but it assumed to be
// very similar to how many NFTs would implement the core functionality.

import NonFungibleToken from 0x1d7e57aa55817448

pub contract MusicBlock: NonFungibleToken {
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64)
        
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    pub var totalSupply: UInt64
    pub let name: String
    pub let symbol: String
    pub let baseMetadataUri: String


    pub struct MusicBlockData {
        pub let creator: Address //creator 
        pub let cpower: UInt64 //computing power
        pub let cid: String //content id refers to ipfs's hash or general URI
        priv let precedences: [UInt64] // cocreated based on which tokens 
        pub let generation: UInt64 //generation, defered for the cocreated tokens
        pub let allowCocreate: Bool //false


        init(creator: Address, cid: String, cp: UInt64, precedences: [UInt64], allowCocreate: Bool){
            self.creator = creator;
            self.cpower = cp;
            self.cid = cid;
            self.precedences = precedences;
            self.allowCocreate = allowCocreate;
            self.generation  = 1; // TOOD: update according to the level of the token
        }


        pub fun getPrecedences() : [UInt64] {
            return self.precedences
        }
    }


    /**
    * We split metadata into two categories: those that are essential and immutable through life time and those that can be 
    * stored on an external storage. Metadata like desc., image, etc. will be stored off chain and publicly accessible via metadata uri.
    * For the first category, we explicitly define them as NFT fields and get accessed via public getters.
    */
    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        priv let data: MusicBlockData
        // priv let supply: UInt64 // cap removed. make a single NFT unique by the standard interface.


        init(initID: UInt64, initCreator: Address, initCpower: UInt64, initCid: String, initPrecedences: [UInt64], initAllowCocreate: Bool) {
            self.id = initID
            self.data = MusicBlockData(creator: initCreator, cid: initCid, cp: initCpower, precedences: initPrecedences, allowCocreate: initAllowCocreate);
            // self.supply = initSupply            
        }


        pub fun getMusicBlockData() : MusicBlockData {
            return self.data
        }
    }


    pub resource interface MusicBlockCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun getMusicBlockData(id: UInt64) : MusicBlockData
        pub fun getUri(id: UInt64) : String
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowMusicBlock(id: UInt64): &MusicBlock.NFT {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result.id != id):
                    "Cannot borrow MusicBlock reference: The ID of the returned reference not exists or incorrect"
            }
        }

    }


    pub resource Collection: MusicBlockCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
        // pub var metadata: {UInt64: { String : String }}


        init () {
            self.ownedNFTs <- {}
            // self.metadata = {}
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
            let token <- token as! @MusicBlock.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            // let oldToken <- self.ownedNFTs[token.id] <-! token
            self.ownedNFTs[id] <-! token
            // self.metadata[id] = metadata
            emit Deposit(id: id, to: self.owner?.address)

            // destroy oldToken
        }


        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }
        
        
        pub fun idExists(id: UInt64): Bool {
            return self.ownedNFTs[id] != nil
        }


        pub fun getMusicBlockData(id: UInt64) : MusicBlockData {
            return self.borrowMusicBlock(id: id).getMusicBlockData()
        }


        pub fun getUri(id: UInt64): String {
            return MusicBlock.baseMetadataUri.concat("/").concat(id.toString()) ;
        }


        // borrowNFT gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }
        

        // borrowMusicBlock
        // Gets a reference to an NFT in the collection as a MusicBlock,
        // exposing all of its fields (including the typeID).
        // This is safe as there are no functions that can be called on the MusicBlock.
        //
        pub fun borrowMusicBlock(id: UInt64): &MusicBlock.NFT {
            let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return ref as! &MusicBlock.NFT
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

        pub fun mintNFT(recipient: &{NonFungibleToken.CollectionPublic}, creator: Address , cpower: UInt64, cid: String, precedences: [UInt64], allowCocreate: Bool) {
            emit Minted(id: MusicBlock.totalSupply)
            // create a new NFT
            var newNFT <- create MusicBlock.NFT(
                initID: MusicBlock.totalSupply, 
                initCreator: creator, 
                initCpower:cpower, initCid:cid, 
                initPrecedences:precedences, 
                initAllowCocreate: allowCocreate
            )

            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-newNFT)

            MusicBlock.totalSupply = MusicBlock.totalSupply + 1
        } 
    }


    init() {
        // Initialize the total supply
        self.totalSupply = 0

        self.name = "MELOS Music Token"
        self.symbol = "MELOSNFT"
        self.baseMetadataUri = "https://meta.melos.finance/melosnft/"

        self.CollectionStoragePath = /storage/MusicBlockCollection
        self.CollectionPublicPath = /public/MusicBlockCollection
        self.MinterStoragePath = /storage/MusicBlockMinter
        
        self.account.save(<-create NFTMinter(), to: self.MinterStoragePath)

        emit ContractInitialized()
    }

    
}