import NonFungibleToken from 0x1d7e57aa55817448
import SoulMadeComponent from 0x543606e9393a64a6
import SoulMadeMain from 0x543606e9393a64a6

pub contract SoulMadePack: NonFungibleToken {

    pub var totalSupply: UInt64

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    pub event SoulMadePackOpened(id: UInt64, packDetail: PackDetail, to: Address?)

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let CollectionPrivatePath: PrivatePath

    pub struct PackDetail{
        pub let id: UInt64
        pub let scarcity: String
        pub let series: String
        pub let ipfsHash: String

        init(id: UInt64,
                scarcity: String,
                series: String,
                ipfsHash: String){
            self.id = id
            self.scarcity = scarcity
            self.series = series
            self.ipfsHash = ipfsHash
        }
    }

    pub struct MainComponentNftIds{
      pub let mainNftIds: [UInt64]
      pub let componentNftIds: [UInt64]

      init(mainNftIds: [UInt64], componentNftIds: [UInt64]){
        self.mainNftIds = mainNftIds
        self.componentNftIds = componentNftIds
      }

    }

    pub resource interface PackPublic {
        pub let id: UInt64
        pub let packDetail: PackDetail
    }

    pub resource NFT: NonFungibleToken.INFT, PackPublic {
        pub let id: UInt64
        pub let packDetail: PackDetail

        pub var mainNft: @{UInt64: SoulMadeMain.NFT}
        pub var componentNft: @{UInt64: SoulMadeComponent.NFT}

        pub fun getMainComponentIds(): MainComponentNftIds {
          return MainComponentNftIds(mainNftIds: self.mainNft.keys, componentNftIds: self.componentNft.keys)
        }

        pub fun depositMain(mainNft: @SoulMadeMain.NFT) {
          var old <- self.mainNft[mainNft.id] <- mainNft
          destroy old
        }

        pub fun depositComponent(componentNft: @SoulMadeComponent.NFT) {
          var old <- self.componentNft[componentNft.id] <- componentNft
          destroy old
        }

        pub fun withdrawMain(mainNftId: UInt64): @SoulMadeMain.NFT? {
          return <- self.mainNft.remove(key: mainNftId)
        }

        pub fun withdrawComponent(componentNftId: UInt64): @SoulMadeComponent.NFT? {
          return <- self.componentNft.remove(key: componentNftId)
        }

        init(initID: UInt64, packDetail: PackDetail) {
            self.id = initID
            self.packDetail = packDetail
            self.mainNft <- {}
            self.componentNft <- {}
        }

        destroy() {
            destroy self.mainNft
            destroy self.componentNft
        }        
    }

    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowPack(id: UInt64): &{SoulMadePack.PackPublic}
    }

    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        pub fun openPack(id: UInt64, mainNftCollectionRef: &{SoulMadeMain.CollectionPublic}?, componentNftCollectionRef: &{SoulMadeComponent.CollectionPublic}?) {     

          let pack <- self.withdraw(withdrawID: id) as! @SoulMadePack.NFT
          
          emit SoulMadePackOpened(id: pack.id, packDetail: pack.packDetail , to: self.owner?.address)

          let mainComponentIds = pack.getMainComponentIds()

          let mainNftIds = mainComponentIds.mainNftIds
          let componentNftIds = mainComponentIds.componentNftIds

          if(mainNftIds.length > 0 && mainNftCollectionRef != nil){
            for mainNftId in mainNftIds{
              var nft <- pack.withdrawMain(mainNftId: mainNftId)! as @NonFungibleToken.NFT
              mainNftCollectionRef!.deposit(token: <- nft)
            }
          } else if mainNftIds.length > 0 && mainNftCollectionRef == nil {
            panic("reference is null")
          }

          if(componentNftIds.length > 0 && componentNftIds != nil){
            for componentNftId in componentNftIds{
              var nft <- pack.withdrawComponent(componentNftId: componentNftId)! as @NonFungibleToken.NFT
              componentNftCollectionRef!.deposit(token: <- nft)
            }
          } else if componentNftIds.length > 0 && componentNftCollectionRef == nil {
            panic("reference is null")
          }
  
          destroy pack
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
          let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing Pack NFT")

          emit Withdraw(id: token.id, from: self.owner?.address)

          return <- token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @SoulMadePack.NFT
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

        pub fun borrowPack(id: UInt64): &{SoulMadePack.PackPublic} {    
            pre {
                self.ownedNFTs[id] != nil: "Main NFT doesn't exist"
            }
            let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return ref as! &SoulMadePack.NFT
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }
    
    access(account) fun mintPack(scarcity: String, series: String, ipfsHash: String, mainNfts: @[SoulMadeMain.NFT], componentNfts: @[SoulMadeComponent.NFT]) : @NFT {

      let packDetail = PackDetail(
          id: SoulMadePack.totalSupply,
          scarcity: scarcity,
          series: series,
          ipfsHash: ipfsHash        
      )

      var pack <- create NFT(initID: SoulMadePack.totalSupply, packDetail: packDetail)
      SoulMadePack.totalSupply = SoulMadePack.totalSupply + UInt64(1)
      while mainNfts.length > 0{
        pack.depositMain(mainNft: <- mainNfts.removeFirst())
      }

      while componentNfts.length > 0{
        pack.depositComponent(componentNft: <- componentNfts.removeFirst())
      }

      destroy mainNfts
      destroy componentNfts
      return <- pack
    }

    init() {
        self.totalSupply = 0

        self.CollectionPublicPath = /public/SoulMadePackCollection
        self.CollectionStoragePath = /storage/SoulMadePackCollection
        self.CollectionPrivatePath = /private/SoulMadePackCollection

        emit ContractInitialized()
    }
}