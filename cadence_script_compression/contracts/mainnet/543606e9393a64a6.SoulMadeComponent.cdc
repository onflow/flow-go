import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract SoulMadeComponent: NonFungibleToken {

    pub var totalSupply: UInt64
    
    pub event ContractInitialized()

    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    pub event SoulMadeComponentCollectionCreated()
    pub event SoulMadeComponentCreated(componentDetail: ComponentDetail)

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let CollectionPrivatePath: PrivatePath

    pub struct ComponentDetail{
        pub let id: UInt64
        pub let series: String
        pub let name: String
        pub let description: String
        pub let category: String
        pub let layer: UInt64
        pub let edition: UInt64
        pub let maxEdition: UInt64
        pub let ipfsHash: String

        init(id: UInt64,
            series: String,
            name: String,
            description: String,
            category: String,
            layer: UInt64,
            edition: UInt64,
            maxEdition: UInt64,
            ipfsHash: String) {
                self.id=id
                self.series=series
                self.name=name
                self.description=description
                self.category=category
                self.layer=layer
                self.edition=edition
                self.maxEdition=maxEdition
                self.ipfsHash=ipfsHash
        }
    }

    pub resource interface ComponentPublic {
        pub let id: UInt64
        pub let componentDetail: ComponentDetail
    }

    pub resource NFT: NonFungibleToken.INFT, ComponentPublic, MetadataViews.Resolver{
        pub let id: UInt64
        pub let componentDetail: ComponentDetail

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: self.componentDetail.name,
                        description: self.componentDetail.description,
                        thumbnail: MetadataViews.IPFSFile(
                            cid: self.componentDetail.ipfsHash,
                            path: nil
                        )
                    )
            }

            return nil
        }

        init(id: UInt64,
            componentDetail: ComponentDetail) {
                self.id=id
                self.componentDetail=componentDetail
        }
    }

    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowComponent(id: UInt64): &{SoulMadeComponent.ComponentPublic}
    }


    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing Component NFT")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <- token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @SoulMadeComponent.NFT
            let id: UInt64 = token.id
            let oldToken <- self.ownedNFTs[id] <- token
            emit Deposit(id: id, to: self.owner?.address)
            destroy oldToken
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowComponent(id: UInt64): &{SoulMadeComponent.ComponentPublic} {
            pre {
                self.ownedNFTs[id] != nil: "Component NFT doesn't exist"
            }
            let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return ref as! &SoulMadeComponent.NFT
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let componentNFT = nft as! &SoulMadeComponent.NFT
            return componentNFT as &AnyResource{MetadataViews.Resolver}
        }        

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        emit SoulMadeComponentCollectionCreated()
        return <- create Collection()
    }

    access(account) fun makeEdition(series: String,
                                    name: String,
                                    description: String,
                                    category: String,
                                    layer: UInt64,
                                    currentEdition: UInt64,
                                    maxEdition: UInt64,
                                    ipfsHash: String) : @NFT {
        let componentDetail = ComponentDetail(
            id: SoulMadeComponent.totalSupply,
            series: series,
            name: name,
            description: description,
            category: category,
            layer: layer,
            edition: currentEdition,
            maxEdition: maxEdition,
            ipfsHash: ipfsHash            
        )

        var newNFT <- create NFT(
            id: SoulMadeComponent.totalSupply,
            componentDetail: componentDetail
        )

        emit SoulMadeComponentCreated(componentDetail: componentDetail)
        
        SoulMadeComponent.totalSupply = SoulMadeComponent.totalSupply + UInt64(1)

        return <- newNFT
    }

    init() {
        self.totalSupply = 0
        
        self.CollectionPublicPath = /public/SoulMadeComponentCollection
        self.CollectionStoragePath = /storage/SoulMadeComponentCollection
        self.CollectionPrivatePath = /private/SoulMadeComponentCollection

        emit ContractInitialized()
    }
}
 