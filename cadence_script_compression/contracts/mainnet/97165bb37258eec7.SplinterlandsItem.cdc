/*
    Description: Central Smart Contract for Splinterlands

    authors: Bilal Shahid bilal@zay.codes
             Amit Ishairzay amit@zay.codes
*/

import NonFungibleToken from 0x1d7e57aa55817448

pub contract SplinterlandsItem: NonFungibleToken {

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Events
    // -----------------------------------------------------------------------

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    // -----------------------------------------------------------------------
    // SplinterlandsItem Events
    // -----------------------------------------------------------------------

    pub event ItemMinted(id: UInt64, itemID: String)
    pub event AdminDeposit(id: UInt64, itemID: String, bridgeAddress: String)
    pub event ItemDestroyed(id: UInt64, itemID: String)

    // -----------------------------------------------------------------------
    // Named Paths
    // -----------------------------------------------------------------------
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminStoragePath: StoragePath
    pub let AdminPublicPath: PublicPath

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Fields
    // -----------------------------------------------------------------------

    pub var totalSupply: UInt64

    // -----------------------------------------------------------------------
    // Splinterlands Struct Fields
    // -----------------------------------------------------------------------
    pub struct SplinterlandItemData {
        pub let id: UInt64
        pub let itemID: String
        init(id: UInt64, itemID: String) {
            self.id = id
            self.itemID = itemID
        }
        pub fun getId(): UInt64 {
            return self.id
        }
        pub fun getItemID(): String {
            return self.itemID
        }
    }

    pub struct SplinterlandsItemStateData {
        access(self) let id: UInt64
        access(self) let state: SplinterlandsItemState
        init(id: UInt64, state: SplinterlandsItemState) {
            self.id = id
            self.state = state
        }
        pub fun getId(): UInt64 {
            return self.id
        }
        pub fun getState(): SplinterlandsItemState {
            return self.state
        }
    }

    // -----------------------------------------------------------------------
    // Splinterlands Standard Fields
    // -----------------------------------------------------------------------

    pub var burnedCount: UInt64

    pub enum SplinterlandsItemState: UInt8 {
        pub case InFlowCirculation
        pub case OutOfFlowCirculation
        pub case Burned
    }

    access(self) var itemIDsMinted: {String: SplinterlandsItemStateData}

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Resources
    // -----------------------------------------------------------------------
    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let itemID: String

        init(id: UInt64, itemID: String) {
            self.id = id
            self.itemID = itemID
            SplinterlandsItem.itemIDsMinted[self.itemID] = SplinterlandsItemStateData(id: self.id, state: SplinterlandsItemState.InFlowCirculation)
            emit ItemMinted(id: self.id, itemID: self.itemID)
        }

        // We don't really have much metadata to give other than the itemID which is in this already
        // Still creating this function as it may become a standard for other uses in the future
        // and provides an upgradeable location to fill in with more data if needed someday
        pub fun getMetadata(): {String: String} {
            let metadata: {String: String} = {}
            metadata["itemID"] = self.itemID
            return metadata
        }

        destroy() {
            post {
                SplinterlandsItem.itemIDsMinted[self.itemID]!.getState() == SplinterlandsItemState.Burned: "Should have state as burned for item id if NFT was destroyed"
            }
            SplinterlandsItem.itemIDsMinted[self.itemID] = SplinterlandsItemStateData(id: self.id, state: SplinterlandsItemState.Burned)
            SplinterlandsItem.burnedCount = SplinterlandsItem.burnedCount + (1 as UInt64)
            emit ItemDestroyed(id: self.id, itemID: self.itemID)
        }
    }


    // -----------------------------------------------------------------------
    // SplinterlandsItem Collection Resources
    // -----------------------------------------------------------------------

    pub resource interface ItemCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowItem(id: UInt64): &SplinterlandsItem.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Item reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, ItemCollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("Cannot withdraw: Item does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @SplinterlandsItem.NFT

            let id = token.id

            let oldToken <- self.ownedNFTs[id] <- token
            
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }
            destroy oldToken
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowItem(id: UInt64): &SplinterlandsItem.NFT? {
            if self.ownedNFTs[id] == nil {
                return nil
            } else {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &SplinterlandsItem.NFT
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }

    }

    // -----------------------------------------------------------------------
    // NonFungibleToken Standard Functions
    // -----------------------------------------------------------------------
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // -----------------------------------------------------------------------
    // SplinterlandsItem Admin Resource
    // -----------------------------------------------------------------------

    pub resource interface AdminPublic { // This should be declared immediately before the admin resource
        pub fun deposit(token: @SplinterlandsItem.NFT, bridgeAddress: String)
    }

    pub resource Admin: AdminPublic {

        access(self) var adminCollectionPublicCapability: Capability<&SplinterlandsItem.Collection{SplinterlandsItem.ItemCollectionPublic}>

        init(adminCollectionPublicCapability:  Capability<&SplinterlandsItem.Collection{SplinterlandsItem.ItemCollectionPublic}>) {
            self.adminCollectionPublicCapability = adminCollectionPublicCapability
        }
        
        pub fun mintItem(adminCollection: &SplinterlandsItem.Collection, recipient: &{SplinterlandsItem.ItemCollectionPublic}, itemID: String) {
            pre {
                SplinterlandsItem.itemIDsMinted[itemID] == nil || SplinterlandsItem.itemIDsMinted[itemID]!.getState() != SplinterlandsItemState.InFlowCirculation: "Cannot mint the item. It's already been minted."
            }
            post {
                SplinterlandsItem.itemIDsMinted[itemID]!.getState() == SplinterlandsItemState.InFlowCirculation: "Item state not set correctly"
            }
            if (SplinterlandsItem.itemIDsMinted[itemID] != nil && SplinterlandsItem.itemIDsMinted[itemID]!.getState() == SplinterlandsItemState.OutOfFlowCirculation) {
                // We've minted this item into flow before - and it's still in the admin account.
                // Instead of making a new one, send the existing one.
                let id: UInt64 = SplinterlandsItem.itemIDsMinted[itemID]!.getId()
                let existingItem <- adminCollection.withdraw(withdrawID: id)
                recipient.deposit(token: <-existingItem)
                SplinterlandsItem.itemIDsMinted[itemID] = SplinterlandsItemStateData(id: id, state: SplinterlandsItemState.InFlowCirculation)
            } else {
                let id: UInt64 = SplinterlandsItem.totalSupply
                let newItem: @SplinterlandsItem.NFT <- create SplinterlandsItem.NFT(initID: id, itemID)
                recipient.deposit(token: <-newItem)
                SplinterlandsItem.totalSupply = SplinterlandsItem.totalSupply + (1 as UInt64)
                SplinterlandsItem.itemIDsMinted[itemID] = SplinterlandsItemStateData(id: id, state: SplinterlandsItemState.InFlowCirculation)
            }
        }

        pub fun deposit(token: @SplinterlandsItem.NFT, bridgeAddress: String) {
            let itemID = token.itemID
            let id = token.id
            self.adminCollectionPublicCapability.borrow()!.deposit(token: <-token)
            SplinterlandsItem.itemIDsMinted[itemID] = SplinterlandsItemStateData(id: id, state: SplinterlandsItemState.OutOfFlowCirculation)
            emit AdminDeposit(id: id, itemID: itemID, bridgeAddress: bridgeAddress)
        }

    }

    pub fun getItemIDs(): {String: SplinterlandsItemStateData} {
        return SplinterlandsItem.itemIDsMinted
    }

    pub fun getItemIDState(itemID: String): SplinterlandsItemStateData {
        return SplinterlandsItem.itemIDsMinted[itemID]!
    }

    init() {
        self.CollectionStoragePath = /storage/SplinterlandsItemCollection
        self.CollectionPublicPath = /public/SplinterlandsItemCollection
        self.AdminStoragePath = /storage/SplinterlandsItemAdmin
        self.AdminPublicPath = /public/SplinterlandsItemAdmin
        
        self.totalSupply = 0
        self.burnedCount = 0
        self.itemIDsMinted = {}

        // Setup Admin Account
        let adminCollection <- SplinterlandsItem.createEmptyCollection() as! @SplinterlandsItem.Collection
        self.account.save(<-adminCollection, to: self.CollectionStoragePath)
        let adminCollectionPublicCapability = self.account.link<&SplinterlandsItem.Collection{NonFungibleToken.CollectionPublic, SplinterlandsItem.ItemCollectionPublic}>(self.CollectionPublicPath, target: self.CollectionStoragePath) ?? panic("Could not get a capability to the admins collection")
        let admin <- create Admin(adminCollectionPublicCapability: adminCollectionPublicCapability)
        self.account.save(<-admin, to: self.AdminStoragePath)
        self.account.link<&SplinterlandsItem.Admin{SplinterlandsItem.AdminPublic}>(self.AdminPublicPath, target: self.AdminStoragePath)

        emit ContractInitialized()
     }

}
