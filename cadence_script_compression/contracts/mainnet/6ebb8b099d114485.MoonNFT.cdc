import NonFungibleToken from 0x1d7e57aa55817448

pub contract MoonNFT: NonFungibleToken {

    pub var totalSupply: UInt64
    // Path for the receiver capability
    pub let MOON_PUBLIC_COLLECTION_PUBLIC_PATH: PublicPath
    pub let PUBLIC_COLLECTION_PUBLIC_PATH: PublicPath

    pub let COLLECTION_PROVIDER_PRIVATE_PATH: PrivatePath
    pub let COLLECTION_RECEIVER_PUBLIC_PATH: PublicPath
    // Path for the QueryMintedCollection
    pub let QUERY_MINTED_COLLECTION_PATH: PublicPath
    // Path for seller catalog capability
    pub let SELLER_CATALOG_PATH: PublicPath

    // Path for the nft collection resource
    pub let ASSET_COLLECTION_STORAGE_PATH: StoragePath
    // Path for minter resource
    pub let MINTER_STORAGE_PATH: StoragePath
    // Path for platform seller resource
    pub let SINGLE_PLATFORM_SELLER_PATH: StoragePath
    // Path for AdminMintCollection resource
    pub let ADMIN_MINT_COLLECTION_PATH: StoragePath

    pub resource NFT : NonFungibleToken.INFT {
        pub let id: UInt64
        pub let data: MoonNftData

        access(contract) init(
            _ initID: UInt64,
            _ mediaUrl: String ,
            creator originalContentCreator: String ,
            creatorId cId: Int32,
            metadata: { String : String }
        ) {
            pre {
                originalContentCreator != "" : "Nft pack data must have a valid creator"
                cId != 0 : "Creator id for pack cannot be 0"
                initID != 0 : "PackId cannot be 0"
                mediaUrl != "" : "preview media url cannot be an empty string"
            }

            self.id = initID
            self.data = MoonNftData(
                initID,
                mediaUrl,
                creator: originalContentCreator,
                creatorId: cId,
                metadata: metadata
            )
        }
    }

    pub struct MoonNftData {
        pub let id: UInt64?
        pub let originalContentCreator: String
        pub let creatorId: Int32
        pub let mediaUrl : String
        pub let metadata: { String : String }

        init (
            _ id: UInt64?,
            _ mediaUrl: String,
            creator originalContentCreator: String,
            creatorId cId: Int32,
            metadata: { String : String }
        ){
            self.id = id
            self.originalContentCreator = originalContentCreator
            self.mediaUrl = mediaUrl
            self.metadata = metadata
            self.creatorId = cId
        }
    }

    pub resource MoonNftPack {
        pub let id: UInt64
        pub let previewMediaUrl: String
        access(contract) let pack : @{UInt64 : NFT}
        pub let title: String
        // made this optional in case we decide to mix nfts from different creators
        // into one pack
        pub let originalContentCreator: String?
        pub let creatorId: Int32?


        access(contract) init (
            _ packId: UInt64,
            _ initialPack : @[NFT],
            _ previewMediaUrl: String,
            title: String,
            creator originalContentCreator : String?,
            creatorId : Int32?
        ) {
            pre {
                originalContentCreator != "" : "Nft pack data must have a valid creator"
                creatorId != 0 : "Creator id for pack cannot be 0"
                title != "" : "title is not allowed to be an empty string"
                packId != 0 : "PackId cannot be 0"
                previewMediaUrl != "" : "preview media url cannot be an empty string"
                initialPack.length != 0 : "initial pack cannot be empty"
            }

            self.originalContentCreator = originalContentCreator
            self.creatorId = creatorId
            self.id = packId
            self.previewMediaUrl = previewMediaUrl
            self.title = title

            self.pack <- {}

            while (initialPack.length > 0) {
                let nft <- initialPack.removeLast()
                let nullNft <- self.pack.insert(key: nft.id, <- nft)
                destroy nullNft
            }

            destroy initialPack
        }

        // TODO improve by generating the metadata in init, then storing as variable,
        // then simply return reference to metadata
        pub fun getData(): MoonNftPackData {
            let nftIds = self.pack.keys

            return MoonNftPackData(
                self.id,
                nftIds,
                self.previewMediaUrl,
                title: self.title,
                creator: self.originalContentCreator,
                creatorId: self.creatorId)
        }

        pub fun exportNftsFromPack() : @[NFT] {
            let exportPack: @[NFT] <- []

            for key in self.pack.keys {
                exportPack.append(<- self.pack.remove(key: key)!)
            }

            return <- exportPack
        }

        destroy () {
            destroy self.pack
        }
    }

    pub struct MoonNftPackData {
        pub let id: UInt64
        pub let previewMediaUrl: String
        pub let collectionNftIds : [UInt64]
        pub let title: String
        pub let creator: String?
        pub let creatorId: Int32?

        init(
            _ id: UInt64,
            _ nftIds: [UInt64],
            _ previewMediaUrl: String,
            title: String,
            creator: String?,
            creatorId: Int32?,
        ) {
            self.id = id
            self.creator = creator
            self.collectionNftIds = nftIds
            self.creatorId = creatorId
            self.previewMediaUrl = previewMediaUrl
            self.title = title
        }
    }

    pub resource MoonNftRelease {
        pub let id : String
        access(self) let packGroupings: @{ String : MoonNftPack}
        pub let packData: MoonNftPackData
        access(self) var price: Int

        access(account) init(
            id: String?
            _ packGroupings: @{ String : MoonNftPack},
            _ packData: MoonNftPackData,
            price : Int
        ) {
            pre {
                id != "" : "release id cannot be an empty string"
                packGroupings.keys.length != 0 : "Pack grouping must contain at least one grouping"
                price > 0 : "Price for release must be greater than 0"
                packData.creator != "" : "release data must have a valid creator"
                packData.creatorId != 0 : "Creator id for pack cannot be 0"
                packData.title != "" : "title is not allowed to be an empty string"
                packData.previewMediaUrl != "" : "preview media url cannot be an empty string"
            }

            self.id = id != nil ? id! : self.uuid.toString()
            self.packGroupings <- packGroupings
            self.packData = packData
            self.price = price
        }

        pub fun getData(): MoonNftReleaseData {
            return MoonNftReleaseData(
                self.id,
                self.packData.previewMediaUrl,
                title: self.packData.title,
                creator: self.packData.creator,
                creatorId: self.packData.creatorId,
                count: self.packGroupings.keys.length,
                packIds: self.getPackIds(),
                price: self.price
            )
        }

        pub fun getPackIds(): [String] {
            return self.packGroupings.keys
        }

        pub fun withdrawPack(packId: String): @MoonNftPack {
            pre {
                self.packGroupings.containsKey(packId) : "Release does not contain pack with Id"
            }

            let pack <- self.packGroupings.remove(key: packId)!

            return <- pack
        }

        pub fun appendPackToRelease(packUUID: String, pack: @MoonNftPack) {
            pre {
                self.packGroupings.containsKey(packUUID) : "Already have a pack within grouping with that UUID. Use a different UUID"
            }

            let nullPack <- self.packGroupings[packUUID] <- pack

            destroy nullPack
        }

        pub fun updatePrice(_ newPrice: Int) {
            self.price = newPrice
        }

        destroy() {
            destroy self.packGroupings
        }
    }

    pub struct MoonNftReleaseData {
        pub let id : String
        pub let previewMediaUrl: String
        pub let title: String
        pub let creator: String?
        pub let creatorId: Int32?
        pub let count : Int
        pub let price : Int
        pub let packIds : [String]

        init(
            _ id: String,
            _ previewMediaUrl: String,
            title: String,
            creator: String?,
            creatorId: Int32?,
            count: Int,
            packIds: [String]
            price: Int
        ) {
            self.id = id
            self.previewMediaUrl = previewMediaUrl
            self.title = title
            self.creator = creator
            self.creatorId = creatorId
            self.count = count
            self.price = price
            self.packIds = packIds
        }
    }

    pub resource interface MoonCollectionPublic {
        pub fun bulkDepositNfts(tokens: @[NFT])
        pub fun depositNftPack(pack: @MoonNftPack)
        pub fun nftIdExists(_ id: UInt64): Bool
        pub fun packIdExists(_ id: UInt64): Bool

        pub fun borrowMoonNft(id: UInt64) : &MoonNFT.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow MoonNft reference: The ID of the returned reference is incorrect"
            }
        }
        pub fun getDataForAllNfts() : [MoonNftData]

        pub fun getNftPackIds(): [UInt64]
        pub fun getNftPackData(id: UInt64) : MoonNftPackData
        pub fun getDataForAllPacks(): [MoonNftPackData]
    }

    pub resource Collection: MoonCollectionPublic,NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
        access(contract) let ownedPacks: @{UInt64: MoonNftPack}

        init () {
            self.ownedNFTs <- {}
            self.ownedPacks <- {}
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let moonToken <- token as! @NFT

            emit Deposit(id: moonToken.id, to: self.owner?.address)
            emit AssetCollection_MoonNftDeposit(data: moonToken.data)
            self.ownedNFTs[moonToken.id] <-! moonToken
        }

        pub fun bulkDepositNfts(tokens: @[MoonNFT.NFT]) {
            while tokens.length > 0 {
                let token <- tokens.removeFirst()
                self.deposit(token: <- token)
            }

            destroy tokens
        }

        pub fun depositNftPack(pack: @MoonNftPack) {
            let packData = pack.getData()
            self.ownedPacks[pack.id] <-! pack

            emit AssetCollection_NftPackDeposit(data: packData)
        }

        pub fun nftIdExists(_ id: UInt64): Bool {
            return self.ownedNFTs.containsKey(id)
        }

        pub fun packIdExists(_ id: UInt64): Bool {
            return self.ownedPacks.containsKey(id)
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

       pub fun borrowMoonNft(id: UInt64) : &MoonNFT.NFT? {
           if (self.ownedNFTs[id] != nil) {
               let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
               return ref as! &MoonNFT.NFT
           } else {
               return nil
           }
        }

        pub fun getDataForAllNfts() : [MoonNftData] {
            let nftData : [MoonNftData] = []

            for key in self.ownedNFTs.keys {
                let nft = self.borrowMoonNft(id: key)
                if (nft != nil) {
                    nftData.append(nft!.data)
                }
            }

            return nftData
        }

        pub fun getNftPackIds(): [UInt64] {
            return self.ownedPacks.keys
        }
        pub fun getNftPackData(id: UInt64) : MoonNftPackData {
            pre {
                self.packIdExists(id) : "Pack does not exist"
            }

            let ownedPacks : &{UInt64: MoonNftPack} = &self.ownedPacks as &{UInt64: MoonNftPack}
            let pack : &MoonNftPack = &self.ownedPacks[id] as &MoonNftPack
            return pack.getData()
        }

        pub fun getDataForAllPacks(): [MoonNftPackData] {
            let packData : [MoonNftPackData] = []

            for key in self.ownedPacks.keys {
                packData.append(self.getNftPackData(id: key))
            }

            return packData
        }

        pub fun withdraw(withdrawID: UInt64) : @NonFungibleToken.NFT {
            pre {
                self.nftIdExists(withdrawID) : "Cannot withdraw NFT that doesn't exist in your collection"
            }

            let moonNft = self.borrowMoonNft(id: withdrawID)
            emit AssetCollection_NftWithdrawn(data: moonNft!.data)

            let nft <- self.ownedNFTs.remove(key: withdrawID)!


            emit Withdraw(id: withdrawID, from: self.owner?.address)

            return <- nft
        }

        pub fun withdrawPack(packId: UInt64) : @MoonNftPack {
            pre {
                self.packIdExists(packId) : "Cannot withdraw pack that doesn't exist in your collection"
            }

            let nftPack <- self.ownedPacks.remove(key: packId)!

            emit AssetCollection_NftPackWithdrawn(data: nftPack.getData())
            return <- nftPack
        }

        pub fun openPackAndDepositNfts (packId: UInt64) : [MoonNftData]{
            pre {
                self.packIdExists(packId) : "Cannot open pack that doesnt exist in your collection"
            }

            let pack <- self.withdrawPack(packId: packId)
            let packData = pack.getData()
            let packNfts <- pack.exportNftsFromPack()

            let nftData : [MoonNftData] = []

            while packNfts.length > 0 {
                let nft <- packNfts.removeFirst()
                nftData.append(nft.data)
                self.deposit(token: <- nft)
            }

            destroy pack
            destroy packNfts

            emit AssetCollection_MoonNftPackOpened(data: packData)
            return nftData
        }

        destroy() {
            destroy self.ownedNFTs
            destroy self.ownedPacks
        }
    }

    pub resource interface SellerCatalog {
        pub fun getNftsByCreatorId(_ creatorId: Int32) : [MoonNftData]
        pub fun getNftsByCreator(_ creator: String) : [MoonNftData]

        pub fun getPackReleasesByCreatorId(_ creatorId: Int32): [MoonNftReleaseData]
        pub fun getPackReleasesByCreator(_ creator: String): [MoonNftReleaseData]


        pub fun getDataForAllReleases(): [MoonNftReleaseData]
        pub fun getTotalPackReleaseCount(): Int
        pub fun packReleaseExists(id: String): Bool

        pub fun getPackReleaseData(id: String): MoonNftReleaseData
        pub fun getNftData(id: UInt64): MoonNftData

        pub fun getCurrentPackIdsAvailableWithinRelease(id: String): [String]

        pub fun getDataForAllNfts(): [MoonNftData]
        pub fun getTotalNFTCount(): Int
        pub fun nftExists(id: UInt64): Bool
    }

    pub resource SinglePlatformSeller: SellerCatalog {
        access(self) let nftsForSale: @{UInt64 : NFT}
        access(self) let packReleasesForSale: @{String : MoonNftRelease}

        access(self) let nftsByCreatorId: {Int32: {UInt64 : MoonNftData}}
        access(self) let packReleasesByCreatorId: {Int32: {String : MoonNftReleaseData}}

        access(self) let nftsByCreator: {String: {UInt64 : MoonNftData}}
        access(self) let packReleasesByCreator: {String: {String : MoonNftReleaseData}}

        access(account) init() {
            self.packReleasesForSale <- {}
            self.nftsForSale <- {}

            self.nftsByCreatorId = {}
            self.packReleasesByCreatorId = {}

            self.nftsByCreator = {}
            self.packReleasesByCreator = {}
        }

        pub fun getDataForAllReleases(): [MoonNftReleaseData] {
            let packs: [MoonNftReleaseData] = []

            for packId in self.packReleasesForSale.keys {
                let packsForSale: &{String : MoonNftRelease} = &self.packReleasesForSale as &{String : MoonNftRelease}
                let release: &MoonNftRelease = &packsForSale[packId] as &MoonNftRelease
                let packMetadata = release.getData()
                packs.append(packMetadata)
            }

            return packs
        }

        pub fun getCurrentPackIdsAvailableWithinRelease(id: String): [String] {
            pre {
                self.packReleaseExists(id: id) : "Release does not exist"
            }

            let packsForSale: &{String : MoonNftRelease} = &self.packReleasesForSale as &{String : MoonNftRelease}
            let release: &MoonNftRelease = &packsForSale[id] as &MoonNftRelease

            return release.getPackIds()
        }

        pub fun getTotalPackReleaseCount(): Int {
            return self.packReleasesForSale.keys.length
        }

        pub fun packReleaseExists(id: String ): Bool {
            return self.packReleasesForSale.containsKey(id)
        }

        pub fun getDataForAllNfts(): [MoonNftData] {
            let nfts: [MoonNftData] = []
            let nftsForSale: &{UInt64 : NFT} = &self.nftsForSale as &{UInt64 : NFT}

            for nftId in nftsForSale.keys {
                let nft: &NFT = &self.nftsForSale[nftId] as &NFT
                let nftData = nft.data
                nfts.append(nftData)
            }

            return nfts
        }

        pub fun getTotalNFTCount(): Int {
            return self.nftsForSale.keys.length
        }

        pub fun nftExists(id: UInt64 ): Bool {
            return self.nftsForSale.containsKey(id)
        }

        pub fun withdrawRelease(id packId: String): @MoonNftRelease {
            pre {
                self.packReleaseExists(id: packId) : "Release does not exist"
            }

            let release <- self.packReleasesForSale.remove(key: packId)!

            // remove pack from creatorIds map
            if (release.packData.creatorId != nil) {
                let creatorIdCollection = self.packReleasesByCreatorId[release.packData.creatorId!]
                creatorIdCollection?.remove(key: release.id)
            }

            // remove pack from creator map
            if (release.packData.creator != nil) {
                let creatorCollection = self.packReleasesByCreator[release.packData.creator!]
                creatorCollection?.remove(key: release.id)
            }

            emit SellerCatalog_ReleaseWithdrawn(data: release.getData())
            return <- release
        }

        pub fun withdrawNft (id nftId: UInt64): @NFT {
            pre {
                self.nftExists(id: nftId) : "Nft you are trying to withdraw does not exist"
            }

            let nft <- self.nftsForSale.remove(key: nftId)!

            let creatorDic = self.nftsByCreator[nft.data.originalContentCreator] ?? panic("Nft Data does not exist for creator of Nft")
            let creatorIdDic = self.nftsByCreatorId[nft.data.creatorId] ?? panic("Nft Data does not exist for Nft associated with creatorId")

            creatorDic.remove(key: nftId)
            creatorIdDic.remove(key: nftId)

            emit SellerCatalog_NftWithdrawn(data: nft.data)
            return <- nft
        }

        pub fun depositNft(_ nft: @NFT) {
            let nftData = nft.data

            if (!self.nftsByCreator.containsKey(nft.data.originalContentCreator)) {
                self.nftsByCreator[nft.data.originalContentCreator] = {}
            }

            if (!self.nftsByCreatorId.containsKey(nft.data.creatorId)) {
                self.nftsByCreatorId[nft.data.creatorId] = {}
            }

            let creatorDic: &{UInt64 : MoonNftData} = &self.nftsByCreator[nft.data.originalContentCreator] as &{UInt64 : MoonNftData}
            let creatorIdDic: &{UInt64 : MoonNftData} = &self.nftsByCreatorId[nft.data.creatorId] as &{UInt64 : MoonNftData}

            creatorDic[nft.id] = nft.data
            creatorIdDic[nft.id] = nft.data

            let nullNft <- self.nftsForSale[nft.id] <- nft

            emit SellerCatalog_NftDeposited(data : nftData)
            destroy  nullNft
        }

        pub fun depositRelease(_ release: @MoonNftRelease) {
            pre {
                !self.packReleasesForSale.containsKey(release.id) : "There is already a release with Id ".concat(release.id).concat(" Present")
            }

            let releaseData = release.getData()

            if (release.packData.creator != nil) {
                if (!self.packReleasesByCreator.containsKey(release.packData.creator !)) {
                    self.packReleasesByCreator.insert(key: release.packData.creator !, {})
                }

                let creatorCollection = &self.packReleasesByCreator[release.packData.creator!] as &{String : MoonNftReleaseData}
                creatorCollection.insert(key: release.id, releaseData)
            }

            if (release.packData.creatorId != nil) {
                if (!self.packReleasesByCreatorId.containsKey(release.packData.creatorId!)) {
                    self.packReleasesByCreatorId.insert(key: release.packData.creatorId!, {})
                }

                let creatorIdCollection = &self.packReleasesByCreatorId[release.packData.creatorId!] as &{String : MoonNftReleaseData}
                creatorIdCollection.insert(key: release.id, releaseData)
            }

            let old <- self.packReleasesForSale.insert(key: release.id, <- release)

            emit SellerCatalog_ReleaseDeposited(data: releaseData)
            // pack Ids are unique so we are guaranteed
            // not to be destroying an actual pack here
            destroy old
        }

        pub fun borrowRelease(releaseId: String): &MoonNftRelease {
            pre {
                self.packReleaseExists(id: releaseId) : "Release does not exist"
            }

            let packReleasesForSale: &{String : MoonNftRelease} = &self.packReleasesForSale as &{String : MoonNftRelease}
            let release : &MoonNftRelease = &packReleasesForSale[releaseId] as &MoonNftRelease

            return release
        }

        pub fun getPackReleaseData(id: String): MoonNftReleaseData {
            pre {
                self.packReleaseExists(id: id) : "Release does not exist"
            }

            let packsForSale: &{String : MoonNftRelease} = &self.packReleasesForSale as &{String : MoonNftRelease}

            let release : &MoonNftRelease = &packsForSale[id] as &MoonNftRelease
            let data = release.getData()

            return data
        }

        pub fun getNftData(id: UInt64): MoonNftData {
            pre {
                self.nftExists(id: id) : "Pack does not exist"
            }

            let nftsForSale: &{UInt64 : NFT} = &self.nftsForSale as &{UInt64 : NFT}

            let nft : &NFT = &nftsForSale[id] as &NFT
            let data = nft.data

            return data
        }

        pub fun bulkDepositRelease(_ packs: @[MoonNftRelease]) {
            pre {
                packs.length > 0 : "Cannot bulk deposit an empty collection of releases"
            }

            var i = 0
            while packs.length > 0  {
                let pack <- packs.removeFirst()
                self.depositRelease(<- pack)

                i = i + 1
            }

            destroy packs
        }

        pub fun bulkDepositNft(_ nfts: @[NFT]) {
            pre {
                nfts.length > 0 : "Cannot bulk deposit an empty collection of nfts"
            }

            var i = 0
            while nfts.length > 0  {
                let nft <- nfts.removeFirst()
                self.depositNft(<- nft)

                i = i + 1
            }
            // TODO
            destroy nfts
        }

        pub fun getNftsByCreatorId(_ creatorId: Int32) : [MoonNftData] {
            pre {
                self.nftsByCreatorId.containsKey(creatorId) : "Creator does not have any Nfts within Catalog"
            }

            let creatorIdDic = self.nftsByCreatorId[creatorId]!

            return creatorIdDic.values
        }
        pub fun getNftsByCreator(_ creator: String) : [MoonNftData] {
            pre {
                self.nftsByCreator.containsKey(creator) : "Creator does not have any Nfts within Catalog"
            }

            let creatorDic = self.nftsByCreator[creator]!

            return creatorDic.values
        }


        pub fun getPackReleasesByCreatorId(_ creatorId: Int32): [MoonNftReleaseData] {
            pre {
                self.packReleasesByCreatorId.containsKey(creatorId) : "CreatorId does not have any Releases associated with them within Catalog"
            }

            let creatorIdDic = self.packReleasesByCreatorId[creatorId]!

            return creatorIdDic.values
        }
        pub fun getPackReleasesByCreator(_ creator: String): [MoonNftReleaseData] {
            pre {
                self.packReleasesByCreator.containsKey(creator) : "Creator Id does not have any Releases associated with them within Catalog"
            }

            let creatorDic = self.packReleasesByCreator[creator]!

            return creatorDic.values
        }

        pub fun withdrawPackFromWithinRelease (_ releaseId: String,  packUUID: String? ) : @MoonNftPack  {
            pre {
                self.packReleaseExists(id: releaseId) : "release you are trying to withdraw from does not exist"
            }

            let releaseCollection = &self.packReleasesForSale as &{String : MoonNftRelease}
            let release : &MoonNFT.MoonNftRelease = &releaseCollection[releaseId] as &MoonNftRelease

            if (packUUID != nil) {
                return <- release.withdrawPack(packId: packUUID!)
            }
            else {
                let packIds = self.getCurrentPackIdsAvailableWithinRelease(id: releaseId)
                if (packIds.length == 0) {
                    panic("There are no more packs to withdraw from this release")
                }

                let packId = packIds.removeFirst()
                return <- release.withdrawPack(packId: packId)
            }
        }

        destroy() {
            destroy self.packReleasesForSale
            destroy self.nftsForSale
        }
    }

    pub resource NftMinter {
        access(self) var nftIdCount: UInt64
        access(self) var packIdCount: UInt64

        access(account) init() {
            self.packIdCount = 0
            self.nftIdCount = 0
        }

        access(self) fun incrementNftId() {
            self.nftIdCount = self.nftIdCount + 1 as UInt64
            // Can't rely on totalSupply to mint NFTs and create unique Id's
            // as this is a publicly accessable and modifiable variable
            // so we have to account for shortcomings of NonFungibleToken standard
            MoonNFT.totalSupply = self.nftIdCount
        }

        access(self) fun incrementPackIdCount() {
            self.packIdCount = self.packIdCount + 1 as UInt64
        }

        pub fun getTrueTotalSupply () : UInt64 {
            return self.nftIdCount
        }

        pub fun mintNFT (_ nftData: MoonNftData ): @MoonNFT.NFT {
            self.incrementNftId()
            let newNFT <- create NFT(
                self.nftIdCount,
                nftData.mediaUrl,
                creator: nftData.originalContentCreator,
                creatorId: nftData.creatorId,
                metadata: nftData.metadata
            )

            emit NftMinter_MoonNftMinted(data: newNFT.data)
            return <-newNFT
        }

        pub fun bulkMintNfts (_ nftsToMint: [MoonNftData]): @[MoonNFT.NFT] {
            pre {
                nftsToMint.length > 0 : "[NftMinter] No NFT's that we can mint"
            }

            let mintedNfts : @[NFT] <- []

            for nftData in nftsToMint {
                let newNFT <- self.mintNFT(nftData)

                mintedNfts.append(<- newNFT)

            }

            return <- mintedNfts
        }

        pub fun createNftPack (_ packOfNfts: @[MoonNFT.NFT], _ data: MoonNftPackData) : @MoonNftPack {
            pre {
                packOfNfts.length != 0 : "Cannot create a pack without nft's in it"
            }

            self.incrementPackIdCount()
            let nftPack <- create MoonNftPack(
                self.packIdCount,
                <- packOfNfts,
                data.previewMediaUrl,
                title: data.title,
                creator: data.creator,
                creatorId: data.creatorId
            )

            emit NftMinter_MoonNftPackCreated(data: nftPack.getData())
            return <- nftPack
        }

        pub fun createNftPackRelease (id: String, _ packOfNfts: @{ String : [MoonNFT.NFT]}, _ data: MoonNftPackData, price: Int) :@MoonNftRelease {
            pre {
                id != "" : "Pack Release Id cannot be empty"
                packOfNfts.keys.length != 0 : "pack of nfts must contain at lease one nft grouping"
                price > 0 : "price cannot be less than 0"
            }

            let packGroupingMap : @{ String : MoonNftPack } <- {}

            for key in packOfNfts.keys {
                let nfts <- packOfNfts.remove(key: key)!

                if (nfts.length == 0) {
                    panic("An nft grouping must have at least one nft in the collection")
                }

                let pack <- self.createNftPack(<- nfts, data)
                let nullNft <- packGroupingMap.insert(key: key, <- pack)
                destroy nullNft
            }

            destroy packOfNfts

            let release <- create MoonNftRelease(id: id, <- packGroupingMap, data, price: price)
            emit NftMinter_MoonNftPackReleaseCreated(data: release.getData())
            return <- release
        }
    }

    pub resource interface QueryMintedCollection {
        pub fun getAllGroups() : [NftGroupData]
        pub fun getGroupInfo(_ groupId: String) : NftGroupData
        pub fun getAllNftIds() : [UInt64]

        pub fun groupIdExists(groupId: String): Bool

        pub fun getGroupInfoByCreator (_ creator: String) : [NftGroupData]
        pub fun getGroupInfoByCreatorId (_ creatorId: Int32) : [NftGroupData]
    }

    pub resource AdminMintedCollection : QueryMintedCollection {
        access(self) let groupMetadata : {String : MoonNftData}
        access(self) let groupNftIds : { String : {UInt64 : UInt64} }
        access(self) let nfts : @{ UInt64 : NFT }

        // mapping creator to groupIds
        access(self) let creatorToGroupMap : { String : { String : Bool }}
        // mapping creatorId to groupIds
        access(self) let creatorIdToGroupMap : { Int32 : { String : Bool }}

        access(account) init () {
            self.groupMetadata = {}
            self.groupNftIds = {}
            self.nfts <- {}

            self.creatorToGroupMap = {}
            self.creatorIdToGroupMap = {}
        }

        pub fun depositGroup(_ groupId: String, _ groupMetadata: MoonNftData, _ nfts: @[NFT]){
            pre {
                nfts.length > 0 : "No NFT's to deposit"
                groupId != "" : "Invalid groupId"
                self.groupIdExists(groupId: groupId) == false : "Cannot deposit an Nft grouping that already exists : ".concat(groupId)
            }

            self.groupMetadata[groupId] = groupMetadata
            var nftIds : {UInt64 : UInt64} = {}

            while nfts.length > 0 {
                let nft <- nfts.removeLast()
                nftIds.insert(key: nft.id, nft.id);

                let nullNft <- self.nfts[nft.id] <- nft

                destroy nullNft
            }


            if (!self.creatorToGroupMap.containsKey(groupMetadata.originalContentCreator)) {
                self.creatorToGroupMap[groupMetadata.originalContentCreator] = {}
            }

            if (!self.creatorIdToGroupMap.containsKey(groupMetadata.creatorId)) {
                self.creatorIdToGroupMap[groupMetadata.creatorId] = {}
            }

            let creatorGroupMap : { String : Bool } = self.creatorToGroupMap[groupMetadata.originalContentCreator]!
            let creatorIdToGroupMap : { String : Bool } = self.creatorIdToGroupMap[groupMetadata.creatorId]!

            creatorGroupMap[groupId] = true
            creatorIdToGroupMap[groupId] = true

            self.creatorToGroupMap[groupMetadata.originalContentCreator] = creatorGroupMap
            self.creatorIdToGroupMap[groupMetadata.creatorId] = creatorIdToGroupMap

            self.groupNftIds[groupId] = nftIds

            emit AdminMintedCollection_NftGroupDeposited(data: self.getGroupInfo(groupId))

            destroy nfts
        }

        pub fun addMoreNftsToDepositedGroup (_ groupId : String, nfts: @[NFT]) {
            pre {
                self.groupIdExists(groupId: groupId) : "cannot append to group that does not exist"
            }

            let nftIds = self.groupNftIds[groupId]!

            while nfts.length > 0 {
                let nft <- nfts.removeLast()
                nftIds.insert(key: nft.id, nft.id);

                let nullNft <- self.nfts[nft.id] <- nft

                destroy nullNft
            }

            self.groupNftIds[groupId] = nftIds

            emit AdminMintedCollection_NftGroupUpdated(data: self.getGroupInfo(groupId))
            destroy nfts
        }

        pub fun getAllNftIds() : [UInt64] {
            return self.nfts.keys
        }

        pub fun groupIdExists(groupId: String): Bool {
            return self.groupNftIds.containsKey(groupId)
        }

        pub fun getAllGroups() : [NftGroupData]{
            let groupData: [NftGroupData] = []

            for groupId in self.groupNftIds.keys {
                groupData.append(self.getGroupInfo(groupId))
            }

            return groupData
        }

        pub fun getGroupInfoByCreator (_ creator: String) : [NftGroupData] {
            pre {
                self.creatorToGroupMap.containsKey(creator) : "Creator grouping does not exist"
            }

            let groupIdMap = self.creatorToGroupMap[creator]!
            let groupInfoCollection : [NftGroupData] = []

            for groupId in groupIdMap.keys {
                groupInfoCollection.append(self.getGroupInfo(groupId))
            }

            return groupInfoCollection
        }

        pub fun getGroupInfoByCreatorId (_ creatorId: Int32) : [NftGroupData] {
            pre {
                self.creatorIdToGroupMap.containsKey(creatorId) : "Creator grouping does not exist"
            }

            let groupIdMap = self.creatorIdToGroupMap[creatorId]!
            let groupInfoCollection : [NftGroupData] = []

            for groupId in groupIdMap.keys {
                groupInfoCollection.append(self.getGroupInfo(groupId))
            }

            return groupInfoCollection
        }

        pub fun getGroupInfo(_ groupId: String) : NftGroupData {
            pre {
                self.groupNftIds[groupId] != nil : "No Nfts associated with group"
                self.groupMetadata[groupId] != nil : "No Metadata associated with group"
            }

            let nftIdGroup = self.groupNftIds[groupId]!
            let nftIds = nftIdGroup.keys

            return NftGroupData(
                groupId,
                nftIds,
                self.groupMetadata[groupId]!
            )
        }

        pub fun pickNfts(_ groupIds: [String]) : @[NFT]{
            pre {
                groupIds.length > 0 : "groupIds are empty. No Nft's to pick "
            }

            let pickedNfts : @[NFT] <- []
            let pickedNftData: [MoonNftData] = []

            while groupIds.length > 0 {
                let groupId = groupIds.removeFirst()
                let nftIds = self.groupNftIds[groupId] ?? panic("Group with Id ".concat(groupId).concat(" does not exist"))

                if (nftIds.keys.length == 0) {
                    panic("No more Nft's in this grouping")
                }
                // remove the nft Id from the group so that we know it no longer exists in this grouping
                let nftId = nftIds.remove(key: nftIds.keys[0])!

                let nft <- self.nfts.remove(key: nftId)!

                if (nftIds.keys.length != 0) {
                    // set groupIds to the newly updated array
                    self.groupNftIds[groupId] = nftIds
                }
                else {
                    // cleanup nft grouping once Id's have been exhausted
                    self.cleanupEmptyGrouping(groupId, creator: nft.data.originalContentCreator, creatorId: nft.data.creatorId)
                }

                pickedNftData.append(nft.data)
                pickedNfts.append(<- nft)
            }

            emit AdminMintedCollection_MoonNftsPicked(data : pickedNftData)
            return <- pickedNfts
        }

        pub fun withdrawAllNftsForGroup (_ groupId: String) : @[NFT] {
            pre {
                self.groupIdExists(groupId: groupId) : "grouping you are trying to withdraw does not exist"
            }

            let withdrawnNftData : [MoonNftData] = []
            let withdrawnNfts : @[NFT] <- []
            let nftIds = self.groupNftIds[groupId]!

            var creator : String? = nil
            var creatorId : Int32? = nil

            if (nftIds.keys.length == 0) {
                panic("Fatal! Cannot withdraw from groupNftIds that are already empty")
            }

            while nftIds.keys.length > 0 {
                let nftId = nftIds.remove(key: nftIds.keys[0])!

                let nft <- self.nfts.remove(key: nftId) ?? panic("Nft within groupNftIds does not exist")

                // all Nft's within this grouping will have the same originalContentCreator and creatorId
                creator = nft.data.originalContentCreator
                creatorId = nft.data.creatorId

                withdrawnNftData.append(nft.data)
                withdrawnNfts.append(<- nft)
            }

            self.cleanupEmptyGrouping(groupId, creator: creator!, creatorId: creatorId!)

            emit AdminMintedCollection_MoonNftsPicked(data: withdrawnNftData)
            return <- withdrawnNfts
        }

        access(self) fun cleanupEmptyGrouping (_ groupId: String, creator: String, creatorId: Int32) {
            pre {
                self.groupIdExists(groupId: groupId) : "groupId you are trying to cleanup doesn't exist or was just exhausted"
                self.creatorToGroupMap.containsKey(creator) : "No creator collection present for groupId".concat(groupId)
                self.creatorIdToGroupMap.containsKey(creatorId) : "No creator Id collection present for groupId".concat(groupId)
            }

            self.groupNftIds.remove(key: groupId)

            let creatorGroupCollection : { String : Bool } = self.creatorToGroupMap[creator]!
            let creatorIdToGroupCollection : { String : Bool } = self.creatorIdToGroupMap[creatorId]!

            creatorGroupCollection.remove(key: groupId)
            creatorIdToGroupCollection.remove(key: groupId)
        }

        destroy () {
            destroy self.nfts
        }
    }

    pub struct NftGroupData {
        pub let groupId : String
        pub let nftIds : [UInt64]
        pub let metadata : MoonNftData

        init (_ groupId: String, _ nftIds: [UInt64], _ metadata: MoonNftData) {
            self.groupId = groupId
            self.nftIds = nftIds
            self.metadata = metadata
        }
    }

    pub fun createEmptyCollection(): @Collection {
        return <- create Collection()
    }

    init() {
        self.totalSupply = 0

        self.MOON_PUBLIC_COLLECTION_PUBLIC_PATH = /public/MoonNftPlatform_CrimsonZone_v1_MoonCollectionPublic
        self.PUBLIC_COLLECTION_PUBLIC_PATH = /public/MoonNftPlatform_CrimsonZone_v1_CollectionPublic

        self.COLLECTION_PROVIDER_PRIVATE_PATH = /private/MoonNftPlatform_CrimsonZone_v1_Provider
        self.COLLECTION_RECEIVER_PUBLIC_PATH = /public/MoonNftPlatform_CrimsonZone_v1_Receiver

        self.SELLER_CATALOG_PATH = /public/MoonNftPlatform_CrimsonZone_v1_SellerCatalog
        self.QUERY_MINTED_COLLECTION_PATH = /public/MoonNftPlatform_CrimsonZone_v1_QueryMintedCollection

        self.ASSET_COLLECTION_STORAGE_PATH = /storage/MoonNftPlatform_CrimsonZone_v1_MoonNFTCollection
        self.MINTER_STORAGE_PATH = /storage/MoonNftPlatform_CrimsonZone_v1_NftMinter
        self.SINGLE_PLATFORM_SELLER_PATH = /storage/MoonNftPlatform_CrimsonZone_v1_PlatformSeller
        self.ADMIN_MINT_COLLECTION_PATH = /storage/MoonNftPlatform_CrimsonZone_v1_AdminMintedCollection

        // setup Collection infrastructure
        self.account.save(<- self.createEmptyCollection(), to: self.ASSET_COLLECTION_STORAGE_PATH)
        self.account.link<&{MoonNFT.MoonCollectionPublic}>(self.MOON_PUBLIC_COLLECTION_PUBLIC_PATH, target: self.ASSET_COLLECTION_STORAGE_PATH)

        // setup NonFungibleToken Infrastructure
        self.account.link<&{NonFungibleToken.CollectionPublic}>(self.PUBLIC_COLLECTION_PUBLIC_PATH, target: self.ASSET_COLLECTION_STORAGE_PATH)
        self.account.link<&{NonFungibleToken.Provider}>(self.COLLECTION_PROVIDER_PRIVATE_PATH, target: self.ASSET_COLLECTION_STORAGE_PATH)
        self.account.link<&{NonFungibleToken.Receiver}>(self.COLLECTION_RECEIVER_PUBLIC_PATH, target: self.ASSET_COLLECTION_STORAGE_PATH)

        // setup minting infrastructure
        self.account.save(<-create NftMinter(), to: self.MINTER_STORAGE_PATH)

        // setup seller infrastructure
        self.account.save(<- create SinglePlatformSeller(), to: self.SINGLE_PLATFORM_SELLER_PATH)
        self.account.link<&{SellerCatalog}>(self.SELLER_CATALOG_PATH, target: self.SINGLE_PLATFORM_SELLER_PATH)

        // setup admin mint collection resource
        self.account.save(<- create AdminMintedCollection(), to: self.ADMIN_MINT_COLLECTION_PATH)
        self.account.link<&{QueryMintedCollection}>(self.QUERY_MINTED_COLLECTION_PATH, target: self.ADMIN_MINT_COLLECTION_PATH)

        emit ContractInitialized()
    }

    pub event NftMinter_MoonNftMinted(data: MoonNftData)

    pub event AdminMintedCollection_MoonNftsPicked(data: [MoonNftData])

    pub event NftMinter_MoonNftPackCreated(data: MoonNftPackData)

    pub event NftMinter_MoonNftPackReleaseCreated(data: MoonNftReleaseData)

    pub event SellerCatalog_ReleaseDeposited (data: MoonNftReleaseData)

    pub event SellerCatalog_NftDeposited (data: MoonNftData)

    pub event SellerCatalog_ReleaseWithdrawn (data: MoonNftReleaseData)

    pub event SellerCatalog_NftWithdrawn (data: MoonNftData)

    pub event AssetCollection_NftPackDeposit(data: MoonNftPackData)

    pub event AssetCollection_NftWithdrawn(data: MoonNftData)

    pub event AssetCollection_NftPackWithdrawn(data: MoonNftPackData)

    pub event AdminMintedCollection_NftGroupDeposited(data: NftGroupData)

    pub event AdminMintedCollection_NftGroupUpdated(data: NftGroupData)

    pub event AssetCollection_MoonNftPackOpened(data: MoonNftPackData)

    pub event AssetCollection_MoonNftDeposit(data: MoonNftData)

    // Event that emitted when the NFT contract is initialized
    //
    pub event ContractInitialized()

    // Event that is emitted when a token is withdrawn,
    // indicating the owner of the collection that it was withdrawn from.
    //
    // If the collection is not in an account's storage, `from` will be `nil`.
    //
    pub event Withdraw(id: UInt64, from: Address?)

    // Event that emitted when a token is deposited to a collection.
    //
    // It indicates the owner of the collection that it was deposited to.
    //
    pub event Deposit(id: UInt64, to: Address?)
}

