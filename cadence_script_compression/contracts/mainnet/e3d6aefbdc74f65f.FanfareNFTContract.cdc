import NonFungibleToken from 0x1d7e57aa55817448

pub contract FanfareNFTContract: NonFungibleToken {

    pub var totalSupply: UInt64

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, mediaURI: String)

    // Event that is emitted when a new minter resource is created
    pub event MinterCreated()

    // Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    
    // The storage Path for minters' MinterProxy
    pub let MinterProxyStoragePath: StoragePath

    // The public path for minters' MinterProxy capability
    pub let MinterProxyPublicPath: PublicPath


    // The storage path for the admin resource
    pub let AdminStoragePath: StoragePath


    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub var mediaURI: String
        pub var creatorAddress: Address

        init(initID: UInt64, mediaURI: String, creatorAddress: Address) {
            self.id = initID
            self.mediaURI = mediaURI
            self.creatorAddress = creatorAddress
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

    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    pub resource NFTMinter {

        pub fun mintNFT(creator: Capability<&{NonFungibleToken.Receiver}>, mediaURI: String, creatorAddress: Address): &NonFungibleToken.NFT {
            let token <- create NFT(
                initID: FanfareNFTContract.totalSupply,
                mediaURI: mediaURI,
                creatorAddress: creatorAddress,
            )
            FanfareNFTContract.totalSupply = FanfareNFTContract.totalSupply + 1
            let tokenRef = &token as &NonFungibleToken.NFT
            emit Minted(id: token.id, mediaURI: mediaURI)
            creator.borrow()!.deposit(token: <- token)
            return tokenRef
        }
    }

    pub resource interface MinterProxyPublic {
        pub fun setMinterCapability(cap: Capability<&NFTMinter>)
    }

    // MinterProxy
    //
    // Resource object holding a capability that can be used to mint new tokens.
    // The resource that this capability represents can be deleted by the admin
    // in order to unilaterally revoke minting capability if needed.

    pub resource MinterProxy: MinterProxyPublic {


        access(self) var minterCapability: Capability<&NFTMinter>?

        pub fun setMinterCapability(cap: Capability<&NFTMinter>) {
            self.minterCapability = cap
        }

        pub fun mintNFT(creator: Capability<&{NonFungibleToken.Receiver}>, mediaURI: String, creatorAddress: Address): &NonFungibleToken.NFT {
          return self.minterCapability!
            .borrow()!
            .mintNFT(creator: creator, mediaURI: mediaURI, creatorAddress: creatorAddress
            )
        }

        init() {
            self.minterCapability = nil
        }

    }

    pub fun createMinterProxy(): @MinterProxy {
        return <- create MinterProxy()
    }


    pub resource Administrator {

        pub fun createNewMinter(): @NFTMinter {
            emit MinterCreated()
            return <- create NFTMinter()
        }

    }




    init() {
        self.CollectionStoragePath = /storage/FanfareNFTCollection
        self.CollectionPublicPath = /public/FanfareNFTCollection

        self.AdminStoragePath = /storage/FanfareAdmin

        self.MinterProxyPublicPath= /public/FanfareNFTMinterProxy
        self.MinterProxyStoragePath= /storage/FanfareNFTMinterProxy

        // Initialize the total supply
        self.totalSupply = 0

        let admin <- create Administrator()
        self.account.save(<-admin, to: self.AdminStoragePath)

        self.account.link<&{NonFungibleToken.Receiver}>(/public/FanfareNFTReceiver, target: /storage/FanfareNFTCollection)

        emit ContractInitialized()        
    }
}
