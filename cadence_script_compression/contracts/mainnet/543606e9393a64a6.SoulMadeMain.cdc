import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448
import SoulMadeComponent from 0x543606e9393a64a6

pub contract SoulMadeMain: NonFungibleToken {

    pub var totalSupply: UInt64

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let CollectionPrivatePath: PrivatePath

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    pub event SoulMadeMainCollectionCreated()
    pub event SoulMadeMainCreated(id: UInt64, series: String)
    pub event NameSet(id: UInt64, name: String)
    pub event DescriptionSet(id: UInt64, description: String)
    pub event IpfsHashSet(id: UInt64, ipfsHash: String)
    pub event MainComponentUpdated(mainNftId: UInt64)

    pub struct MainDetail{
        pub let id: UInt64
        pub(set) var name: String
        pub let series: String
        pub(set) var description: String
        pub(set) var ipfsHash: String
        pub(set) var componentDetails: [SoulMadeComponent.ComponentDetail]

        init(id: UInt64,
                name: String,
                series: String,
                description: String,
                ipfsHash: String,
                componentDetails: [SoulMadeComponent.ComponentDetail]){

            self.id = id
            self.name = name
            self.series = series
            self.description = description
            self.ipfsHash = ipfsHash
            self.componentDetails = componentDetails
        }
    }

    pub resource interface MainPublic {
        pub let id: UInt64
        pub let mainDetail: MainDetail

        pub fun getAllComponentDetail(): {String : SoulMadeComponent.ComponentDetail} 
    }

    pub resource interface MainPrivate {
        pub fun setName(_ name: String)
        pub fun setDescription(_ description: String)
        pub fun setIpfsHash(_ ipfsHash: String)
        pub fun withdrawComponent(category: String): @SoulMadeComponent.NFT?
        pub fun depositComponent(componentNft: @SoulMadeComponent.NFT): @SoulMadeComponent.NFT?
    }

    pub resource NFT: MainPublic, MainPrivate, NonFungibleToken.INFT, MetadataViews.Resolver {
        pub let id: UInt64
        pub let mainDetail: MainDetail

        access(self) var components: @{String : SoulMadeComponent.NFT}

        init(id: UInt64, series: String) {
            self.id = id
            self.mainDetail = MainDetail(id: id, name: "", series: series, description: "", ipfsHash: "", componentDetails: [])
            self.components <- {}
        }
        
        pub fun getAllComponentDetail(): {String : SoulMadeComponent.ComponentDetail} {
            var info : {String : SoulMadeComponent.ComponentDetail} = {}
            for categoryKey in self.components.keys{
                let componentRef = &self.components[categoryKey] as! &SoulMadeComponent.NFT
                let detail = componentRef.componentDetail
                info[categoryKey] = detail
            }
            return info
        }

        pub fun withdrawComponent(category: String): @SoulMadeComponent.NFT {
            let componentNft <- self.components.remove(key: category)!
            self.mainDetail.componentDetails = self.getAllComponentDetail().values
            emit MainComponentUpdated(mainNftId: self.id)
            return <- componentNft
        }

        pub fun depositComponent(componentNft: @SoulMadeComponent.NFT): @SoulMadeComponent.NFT? {
            let category : String = componentNft.componentDetail.category
            var old <- self.components[category] <- componentNft
            self.mainDetail.componentDetails = self.getAllComponentDetail().values
            emit MainComponentUpdated(mainNftId: self.id)
            return <- old
        }

        pub fun setName(_ name: String){
            pre {
                name.length > 2 : "The name is too short"
                name.length < 100 : "The name is too long" 
            }
            self.mainDetail.name = name
            emit NameSet(id: self.id, name: name)
        }

        pub fun setDescription(_ description: String){
            pre {
                description.length > 2 : "The descripton is too short"
                description.length < 500 : "The description is too long" 
            }
            self.mainDetail.description = description
            emit DescriptionSet(id: self.id, description: description)
        }

        pub fun setIpfsHash(_ ipfsHash: String){
            self.mainDetail.ipfsHash = ipfsHash
            emit IpfsHashSet(id: self.id, ipfsHash: ipfsHash)
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
                        name: self.mainDetail.name,
                        description: self.mainDetail.description,
                        thumbnail: MetadataViews.IPFSFile(
                            cid: self.mainDetail.ipfsHash,
                            path: nil
                        )
                    )
            }

            return nil
        }

        destroy() {
            destroy self.components
        }
    }

    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowMain(id: UInt64): &{SoulMadeMain.MainPublic}
    }

    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing Main NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <- token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @SoulMadeMain.NFT
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

        pub fun borrowMain(id: UInt64): &{SoulMadeMain.MainPublic} {
            pre {
                self.ownedNFTs[id] != nil: "Main NFT doesn't exist"
            }
            let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return ref as! &SoulMadeMain.NFT
        }

        pub fun borrowMainPrivate(id: UInt64): &{SoulMadeMain.MainPrivate} {
            pre {
                self.ownedNFTs[id] != nil: "Main NFT doesn't exist"
            }
            let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return ref as! &SoulMadeMain.NFT
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let mainNFT = nft as! &SoulMadeMain.NFT
            return mainNFT as &AnyResource{MetadataViews.Resolver}
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        emit SoulMadeMainCollectionCreated()
        return <- create Collection()
    }

    pub fun mintMain(series: String) : @NFT {
        var new <- create NFT(id : SoulMadeMain.totalSupply, series: series)
        emit SoulMadeMainCreated(id: SoulMadeMain.totalSupply, series: series)
        SoulMadeMain.totalSupply = SoulMadeMain.totalSupply + 1
        return <- new
    }

    init() {
        self.totalSupply = 0

        self.CollectionPublicPath = /public/SoulMadeMainCollection
        self.CollectionStoragePath = /storage/SoulMadeMainCollection
        self.CollectionPrivatePath = /private/SoulMadeMainCollection
        
        emit ContractInitialized()
    }
}
 