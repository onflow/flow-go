import NonFungibleToken from 0x1d7e57aa55817448

pub contract DigitalContentAsset: NonFungibleToken {
    pub var totalSupply: UInt64

    pub event ContractInitialized()
    pub event Deposit(id: UInt64, to: Address?)
    pub event Withdraw(id: UInt64, from: Address?)

    pub let collectionStoragePath: StoragePath
    pub let collectionPublicPath: PublicPath

    pub event TokenCreated(
        id: UInt64,
        refId: String,
        serialNumber: UInt32,
        itemId: String,
        itemVersion: UInt32,
    )

    pub event TokenDestroyed(
        id: UInt64,
        refId: String,
        serialNumber: UInt32,
        itemId: String,
        itemVersion: UInt32,
    )

    pub struct Item {
        pub let itemId: String
        pub var version: UInt32
        pub var mintedCount: UInt32
        pub var limit: UInt32
        pub var active: Bool
        access(self) let versions: { UInt32: ItemData }

        init(itemId: String, version: UInt32, metadata: { String: String }, limit: UInt32, active: Bool) {
            self.itemId = itemId

            let data = ItemData(version: version, metadata: metadata, originSerialNumber: 1)
            self.versions = { data.version: data }
            self.version = data.version
            self.mintedCount = 0
            self.limit = limit
            self.active = active
        }

        access(contract) fun setMetadata(version: UInt32, metadata: { String: String }) {
            pre {
                version >= self.version: "Version must be greater than or equal to the current version"
                version > self.version || (version == self.version && !self.isVersionLocked()): "Locked version cannot be overwritten"
            }
            post {
                self.version == version: "Must be the specified version"
                self.versions[version] != nil: "ItemData must be in the specified version"
            }
            let data = ItemData(version: version, metadata: metadata, originSerialNumber: self.mintedCount + 1)
            self.versions.insert(key: version, data)
            self.version = version
        }

        access(contract) fun setLimit(limit: UInt32) {
            pre {
                self.mintedCount == 0: "Limit can be changed only if it has never been mint"
            }
            self.limit = limit
        }

        access(contract) fun setActive(active: Bool) {
            self.active = active
        }

        access(contract) fun countUp(): UInt32 {
            pre {
                self.mintedCount < self.limit: "Item cannot be minted beyond the limit"
            }
            self.mintedCount = self.mintedCount + 1
            return self.mintedCount
        }

        pub fun getData(): ItemData {
            return self.versions[self.version]!
        }

        pub fun getVersions():  { UInt32: ItemData } {
            return self.versions
        }

        pub fun isVersionLocked(): Bool {
            return self.mintedCount >= self.versions[self.version]!.originSerialNumber
        }

        pub fun isLimitLocked(): Bool {
            return self.mintedCount > 0
        }

        pub fun isFulfilled(): Bool {
            return self.mintedCount >= self.limit
        }
    }

    pub struct ItemData {
        pub let version: UInt32
        pub let originSerialNumber: UInt32
        access(self) let metadata: { String: String }

        init(version: UInt32, metadata: { String: String }, originSerialNumber: UInt32) {
            self.version = version
            self.metadata = metadata
            self.originSerialNumber = originSerialNumber
        }

        pub fun getMetadata(): { String: String } {
            return self.metadata
        }
    }

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let refId: String
        access(self) let data: NFTData

        init(refId: String, data: NFTData) {
            DigitalContentAsset.totalSupply = DigitalContentAsset.totalSupply + (1 as UInt64)

            self.id = DigitalContentAsset.totalSupply
            self.refId = refId
            self.data = data

            emit DigitalContentAsset.TokenCreated(
                id: self.id,
                refId: refId,
                serialNumber: data.serialNumber,
                itemId: data.itemId,
                itemVersion: data.itemVersion
            )
        }

        destroy() {
            emit DigitalContentAsset.TokenDestroyed(
                id: self.id,
                refId: self.refId,
                serialNumber: self.data.serialNumber,
                itemId: self.data.itemId,
                itemVersion: self.data.itemVersion
            )
        }

        pub fun getData(): NFTData {
            return self.data
        }
    }

    pub struct NFTData {
        pub let serialNumber: UInt32
        pub let itemId: String
        pub let itemVersion: UInt32
        access(self) let metadata: { String: String }

        init(
            serialNumber: UInt32,
            itemId: String,
            itemVersion: UInt32,
            metadata: { String: String }
        ) {
            self.serialNumber = serialNumber
            self.itemId = itemId
            self.itemVersion = itemVersion
            self.metadata = metadata
        }

        pub fun getMetadata(): { String: String } {
            return self.metadata
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <-create DigitalContentAsset.Collection()
    }

    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowDCAToken(id: UInt64): &DigitalContentAsset.NFT? {
            post {
                (result == nil) || (result?.id == id): "Invalid id"
            }
        }
    }

    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var ownedNFTs: @{ UInt64: NonFungibleToken.NFT }

        init() {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            pre {
                self.ownedNFTs.containsKey(withdrawID): "That withdrawID does not exist"
            }
            let token <- self.ownedNFTs.remove(key: withdrawID)! as! @NFT

            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- create Collection()

            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }

            return <-batchCollection
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            pre {
                !self.ownedNFTs.containsKey(token.id): "That id already exists"
            }
            let token <- token as! @DigitalContentAsset.NFT
            let id = token.id
            let refId = token.refId
            self.ownedNFTs[id] <-! token

            emit Deposit(id: id, to: self.owner?.address)
        }

        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {
            let keys = tokens.getIDs()
            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }
            destroy tokens
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowDCAToken(id: UInt64): &DigitalContentAsset.NFT? {
            let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return ref as? &DigitalContentAsset.NFT
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    access(account) fun createItem(itemId: String, version: UInt32, limit: UInt32, metadata: { String: String }, active: Bool): Item {
        pre {
            !DigitalContentAsset.items.containsKey(itemId): "Admin cannot create existing items"
        }
        post {
            DigitalContentAsset.items.containsKey(itemId): "items contains the created item"
        }

        let item = Item(itemId: itemId, version: version, metadata: metadata, limit: limit, active: active)
        DigitalContentAsset.items.insert(key: itemId, item)

        return item
    }

    access(account) fun updateMetadata(itemId: String, version: UInt32, metadata: { String: String }) {
        pre {
            DigitalContentAsset.items.containsKey(itemId): "Metadata of non-existent item cannot be updated"
        }
        DigitalContentAsset.items[itemId]!.setMetadata(version: version, metadata: metadata)
    }

    access(account) fun updateLimit(itemId: String, limit: UInt32) {
        pre {
            DigitalContentAsset.items.containsKey(itemId): "Limit of non-existent item cannot be updated"
        }
        DigitalContentAsset.items[itemId]!.setLimit(limit: limit)
    }

    access(account) fun updateActive(itemId: String, active: Bool) {
        pre {
            DigitalContentAsset.items.containsKey(itemId): "Limit of non-existent item cannot be updated"
            DigitalContentAsset.items[itemId]!.active != active: "Item cannot be updated with the same value"
        }
        DigitalContentAsset.items[itemId]!.setActive(active: active)
    }

    access(account) fun mintToken(
        refId: String,
        itemId: String,
        itemVersion: UInt32,
        metadata: { String: String }
    ): @NFT {
        pre {
            DigitalContentAsset.items.containsKey(itemId) != nil: "That itemId does not exist"
            itemVersion == DigitalContentAsset.items[itemId]!.version : "That itemVersion did not match the latest version"
            !DigitalContentAsset.items[itemId]!.isFulfilled(): "Fulfilled items cannot be mint"
            DigitalContentAsset.items[itemId]!.active: "Only active items can be mint"
        }
        post {
            DigitalContentAsset.totalSupply == before(DigitalContentAsset.totalSupply) + 1: "totalSupply must be incremented"
            DigitalContentAsset.items[itemId]!.mintedCount == before(DigitalContentAsset.items[itemId])!.mintedCount + 1: "mintedCount must be incremented"
            DigitalContentAsset.items[itemId]!.isVersionLocked(): "item must be locked once mint"
        }

        let serialNumber = DigitalContentAsset.items[itemId]!.countUp()

        let data = NFTData(
            serialNumber: serialNumber,
            itemId: itemId,
            itemVersion: itemVersion,
            metadata: metadata
        )

        return <- create NFT(refId: refId, data: data)
    }

    access(self) let items: { String: Item }

    // Public

    pub fun getItemIds(): [String] {
        return self.items.keys
    }

    pub fun getItem(_ itemId: String): Item? {
        return self.items[itemId]
    }

    init() {
        self.collectionStoragePath = /storage/DCACollection
        self.collectionPublicPath = /public/DCACollection

        self.totalSupply = 0
        self.items = {}
    }
}
