// Popsycl NFT Marketplace
// PopsyclPack smart contract
// Version         : 0.0.1
// Blockchain      : Flow www.onFlow.org
// Developer :  RubiconFinTech.com

import NonFungibleToken from 0x1d7e57aa55817448
import Popsycl from 0x456d627c3ac86487

pub contract PopsyclPack: NonFungibleToken {
   
    // Total number of pack token supply
    pub var totalSupply: UInt64
    
    // Path where the pack `Collection` is stored
    pub let PopsyclPackStoragePath: StoragePath

    // Path where the pack public capability for the `Collection` is
    pub let PopsyclPackPublicPath: PublicPath

    // Pack NFT Minter
    pub let PopsyclPackMinterPath: StoragePath

    // pack Contract Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event PopsyclNFTDeposit(id: UInt64)
    pub event PopsyclNFTWithdaw(id: UInt64)
    pub event PackMint(packTokenId: UInt64, packId: UInt64, name: String, royalty:UFix64, owner: Address?, influencer: Address?, tokens: [UInt64])
  
    // TOKEN RESOURCE
    pub resource NFT: NonFungibleToken.INFT {

        // pack Unique identifier for NFT Token
        pub let id :UInt64

        // pack name
        pub let name: String

        // NFTs store 
        access(self) let packs: @{UInt64: Popsycl.NFT}

        // pack royalty
        pub let royalty:UFix64

        // pack NFT token creator address
        pub let creator:Address?

        //  pack nft influencer
        pub let influencer:Address?

        // In current store static dict in meta data
        init( id : UInt64, name: String, royalty:UFix64, creator:Address?, influencer:Address) {
            self.id = id
            self.name = name
            self.packs <- {}
            self.royalty = royalty
            self.creator = creator
            self.influencer = influencer
        }

        // old NFTS
        pub fun addPopsycNfts(token: @Popsycl.NFT){
          emit PopsyclNFTDeposit(id: token.id)
          let oldToken <- self.packs[token.id] <- token
          destroy oldToken 
        }

        pub fun packMint(sellerRef: &Popsycl.Collection, packId: UInt64, tokens: [UInt64], name: String, recipient: &{PopsyclPackCollectionPublic}, influencerRecipient: Address, royalty:UFix64) {
            pre {
                tokens.length > 1 : "please provide atleat two NFTS for pack's"
            }
            let token <- create NFT(id: PopsyclPack.totalSupply, name:name, royalty:royalty, creator: recipient.owner?.address, influencer: influencerRecipient)
                emit PackMint(packTokenId:PopsyclPack.totalSupply, packId: packId, name:name, royalty:royalty, owner: recipient.owner?.address, influencer: influencerRecipient, tokens: tokens)
                recipient.deposit(token: <- token)
                PopsyclPack.totalSupply = PopsyclPack.totalSupply + 1 as UInt64

            for id in tokens {
                let token <- sellerRef.withdraw(withdrawID: id) as! @Popsycl.NFT
                self.addPopsycNfts(token: <- token)
            }
        }

        // old NFTS
        pub fun withdraw(id: UInt64): @Popsycl.NFT {
            // remove and return the token
            emit PopsyclNFTWithdaw(id: id)
            let token <- self.packs.remove(key: id) ?? panic("missing NFT")
            return <-token
        }

        destroy() {
          destroy self.packs
        }
    }

    // Account's pack public collection
    pub resource interface PopsyclPackCollectionPublic {
        pub fun deposit(token:@NonFungibleToken.NFT)

        pub fun getIDs(): [UInt64]

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
    } 

    // pack NFT Collection resource
    pub resource Collection : NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, PopsyclPackCollectionPublic {
        
        // Contains caller's list of pack NFTs
        pub var ownedNFTs: @{UInt64 : NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {

            let token <- token as! @PopsyclPack.NFT

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

        destroy(){
            destroy self.ownedNFTs
        }
    }

    // This is used to create the empty pack collection. without this address cannot access our NFT token
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create PopsyclPack.Collection()
    }

    // Contract init
    init() {
        // total supply is zero at the time of contract deployment
        self.totalSupply = 0

        self.PopsyclPackStoragePath = /storage/PopsyclPackNFTCollection

        self.PopsyclPackPublicPath = /public/PopsyclPackNFTPublicCollection

        self.PopsyclPackMinterPath = /storage/PopsyclPackNFTMinter

        self.account.save(<-self.createEmptyCollection(), to: self.PopsyclPackStoragePath)

        self.account.link<&{PopsyclPackCollectionPublic}>(self.PopsyclPackPublicPath, target:self.PopsyclPackStoragePath)

        self.account.save(<-create NFT(id: 1, name: "pack name", royalty: 10.0, creator: 0x456d627c3ac86487, influencer: 0x5dc999f0bb011052 ), to: self.PopsyclPackMinterPath)

        emit ContractInitialized()
    }

}
