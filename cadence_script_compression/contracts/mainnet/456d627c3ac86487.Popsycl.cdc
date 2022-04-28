// Popsycl NFT Marketplace
// NFT smart contract
// Version         : 0.0.1
// Blockchain      : Flow www.onFlow.org
// Owner           : Popsycl.com
// Developer       : RubiconFinTech.com

import NonFungibleToken from 0x1d7e57aa55817448

pub contract Popsycl: NonFungibleToken {
   
    // Total number of token supply
    pub var totalSupply: UInt64
  
    // NFT No of Editions(Multiple copies) limit
    pub var editionLimit: UInt

    /// Path where the `Collection` is stored
    pub let PopsyclStoragePath: StoragePath

    /// Path where the public capability for the `Collection` is
    pub let PopsyclPublicPath: PublicPath

    /// NFT Minter
    pub let PopsyclMinterPath: StoragePath
  
    // Contract Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Mint(id: UInt64, content:String, royality:UFix64, owner: Address?, influencer: Address?)
    pub event GroupMint(id: UInt64, content:String, royality:UFix64, owner: Address?, influencer: Address?, tokenGroupId: UInt64 )

    // TOKEN RESOURCE
    pub resource NFT: NonFungibleToken.INFT {

        // Unique identifier for NFT Token
        pub let id :UInt64

        // Meta data to store token data (use dict for data)
        access(self) let metaData: {String : String}

        pub fun getMetadata():{String: String} {
            return self.metaData
        }

        pub let royality:UFix64
        // NFT token creator address
        pub let creator:Address?

        pub let influencer:Address?

        // In current store static dict in meta data
        init( id : UInt64, content : String, royality:UFix64, creator:Address?, influencer:Address) {
            self.id = id
            self.metaData = {"content" : content}
            self.royality = royality
            self.creator = creator
            self.influencer = influencer
        }

    }

    // Account's public collection
    pub resource interface PopsyclCollectionPublic {

        pub fun deposit(token:@NonFungibleToken.NFT)

        pub fun getIDs(): [UInt64]

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT

        pub fun borrowPopsycl(id: UInt64): &Popsycl.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow CaaPass reference: The ID of the returned reference is incorrect"
            }
        }

    } 

    // NFT Collection resource
    pub resource Collection : PopsyclCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        
        // Contains caller's list of NFTs
        pub var ownedNFTs: @{UInt64 : NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {

            let token <- token as! @Popsycl.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // function returns token keys of owner
        pub fun getIDs():[UInt64] {
            return self.ownedNFTs.keys
        }

        // function returns token data of token id
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // function to check wether the owner have token or not
        pub fun tokenExists(id:UInt64) : Bool {
            return self.ownedNFTs[id] != nil
        }

        pub fun withdraw(withdrawID:UInt64) : @NonFungibleToken.NFT {
            
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token    

        }

        // exposing all of its fields.
        pub fun borrowPopsycl(id: UInt64): &Popsycl.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Popsycl.NFT
            } else {
                return nil
            }
        }

        destroy(){
            destroy self.ownedNFTs
        }
    }

    // NFT MINTER
    pub resource NFTMinter {

        // Function to mint group of tokens
        pub fun GroupMint(recipient: &{PopsyclCollectionPublic}, influencerRecipient: Address, content:String, edition:UInt, tokenGroupId: UInt64, royality:UFix64) {
            pre {
                Popsycl.editionLimit >= edition : "Edition count exceeds the limit"
                edition >=2 : "Edition count should be greater than or equal to 2"
            }
            var count = 0 as UInt
            while count < edition {
                let token <- create NFT(id: Popsycl.totalSupply, content:content, royality:royality, creator: recipient.owner?.address, influencer: influencerRecipient )
                emit GroupMint(id:Popsycl.totalSupply,content:content, royality:royality, owner: recipient.owner?.address, influencer: influencerRecipient, tokenGroupId:tokenGroupId)
                recipient.deposit(token: <- token)
                Popsycl.totalSupply = Popsycl.totalSupply + 1 as UInt64
                count = count + 1
            }
        }

        pub fun Mint(recipient: &{PopsyclCollectionPublic}, influencerRecipient: Address, content:String, royality:UFix64) {
            let token <- create NFT(id: Popsycl.totalSupply, content:content, royality:royality, creator: recipient.owner?.address, influencer: influencerRecipient)
            emit Mint(id:Popsycl.totalSupply,content:content, royality:royality, owner: recipient.owner?.address, influencer: influencerRecipient)
            recipient.deposit(token: <- token)
            Popsycl.totalSupply = Popsycl.totalSupply + 1 as UInt64
        } 
    }

    // This is used to create the empty collection. without this address cannot access our NFT token
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Popsycl.Collection()
    }

    // Admin can change the maximum supported group minting count limit for the platform. Currently it is 50
    pub resource Admin {
        pub fun changeLimit(limit:UInt) {
            Popsycl.editionLimit = limit
        }
    }

    // Contract init
    init() {

        // total supply is zero at the time of contract deployment
        self.totalSupply = 0

        self.editionLimit = 1000
        
        self.PopsyclStoragePath = /storage/PopsyclNFTCollection

        self.PopsyclPublicPath = /public/PopsyclNFTPublicCollection

        self.PopsyclMinterPath = /storage/PopsyclNFTMinter

        self.account.save(<-self.createEmptyCollection(), to: self.PopsyclStoragePath)

        self.account.link<&{PopsyclCollectionPublic}>(self.PopsyclPublicPath, target:self.PopsyclStoragePath)

        self.account.save(<-create self.Admin(), to: /storage/PopsyclAdmin)

        // store a minter resource in account storage
        self.account.save(<-create NFTMinter(), to: self.PopsyclMinterPath)

        emit ContractInitialized()

    }

}
