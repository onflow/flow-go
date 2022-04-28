import FungibleToken from 0xf233dcee88fe0abe
import NFTStorefront from 0x4eb8a10cb9f87357

pub contract Marketplace {

    pub struct Item {
        pub let storefrontPublicCapability: Capability<&{NFTStorefront.StorefrontPublic}>

        // NFTStoreFront.Listing resource uuid
        pub let listingID: UInt64

        // Store listingDetails to prevent vanishing from storefrontPublicCapability
        pub let listingDetails: NFTStorefront.ListingDetails

        // When to add this item to marketplace
        pub let timestamp: UFix64

        init(storefrontPublicCapability: Capability<&{NFTStorefront.StorefrontPublic}>, listingID: UInt64) {
            self.storefrontPublicCapability = storefrontPublicCapability
            self.listingID = listingID
            let storefrontPublic = storefrontPublicCapability.borrow() ?? panic("Could not borrow public storefront from capability")
            let listingPublic = storefrontPublic.borrowListing(listingResourceID: listingID) ?? panic("no listing id")
            // check if owner is correct
            assert(listingPublic.borrowNFT() != nil, message: "could not borrow NFT")

            self.listingDetails = listingPublic.getDetails()
            self.timestamp = getCurrentBlock().timestamp
        }
    }

    pub struct SaleCutRequirement {
        pub let receiver: Capability<&{FungibleToken.Receiver}>

        pub let ratio: UFix64

        init(receiver: Capability<&{FungibleToken.Receiver}>, ratio: UFix64) {
            pre {
                ratio <= 1.0: "ratio must be less than or equal to 1.0"
            }
            self.receiver = receiver
            self.ratio = ratio
        }
    }

    pub let MarketplaceAdminStoragePath: StoragePath

    // listingID order by time, listingID asc
    access(contract) let listingIDs: [UInt64]

    // listingID => item
    access(contract) let listingIDItems: {UInt64: Item}

    // collection identifier => (NFT id => listingID)
    access(contract) let collectionNFTListingIDs: {String: {UInt64: UInt64}}

    // collection identifier => SaleCutRequirements
    access(contract) let saleCutRequirements: {String: [SaleCutRequirement]}

    // Administrator
    //
    pub resource Administrator {

        pub fun updateSaleCutRequirements(_ requirements: [SaleCutRequirement], nftType: Type) {
            var totalRatio: UFix64 = 0.0
            for requirement in requirements {
                totalRatio = totalRatio + requirement.ratio
            }
            assert(totalRatio <= 1.0, message: "total ratio must be less than or equal to 1.0")
            Marketplace.saleCutRequirements[nftType.identifier] = requirements
        }

        pub fun forceRemoveListing(id: UInt64) {
            if let item = Marketplace.listingIDItems[id] {
                Marketplace.removeItem(item)
            }
        }
    }

    pub fun getListingIDs(): [UInt64] {
        return self.listingIDs
    }

    pub fun getListingIDItem(listingID: UInt64): Item? {
        return self.listingIDItems[listingID]
    }

    pub fun getListingID(nftType: Type, nftID: UInt64): UInt64? {
        let nftListingIDs = self.collectionNFTListingIDs[nftType.identifier] ?? {}
        return nftListingIDs[nftID]
    }

    pub fun getAllSaleCutRequirements(): {String: [SaleCutRequirement]} {
        return self.saleCutRequirements
    }

    pub fun getSaleCutRequirements(nftType: Type): [SaleCutRequirement] {
        return self.saleCutRequirements[nftType.identifier] ?? []
    }

    pub fun addListing(id: UInt64, storefrontPublicCapability: Capability<&{NFTStorefront.StorefrontPublic}>) {
        let item = Item(storefrontPublicCapability: storefrontPublicCapability, listingID: id)

        let indexToInsertListingID = self.getIndexToAddListingID(item: item, items: self.listingIDs)

        self.addItem(
            item,
            storefrontPublicCapability: storefrontPublicCapability,
            indexToInsertListingID: indexToInsertListingID)
    }

    pub fun addListingWithIndex(
        id: UInt64,
        storefrontPublicCapability: Capability<&{NFTStorefront.StorefrontPublic}>,
        indexToInsertListingID: Int
    ) {
        let item = Item(storefrontPublicCapability: storefrontPublicCapability, listingID: id)

        self.checkValidIndexToInsert(
            item: item,
            index: indexToInsertListingID,
            items: self.listingIDs)

        self.addItem(
            item,
            storefrontPublicCapability: storefrontPublicCapability,
            indexToInsertListingID: indexToInsertListingID)
    }

    // Anyone can remove it if the listing item has been removed or purchased.
    pub fun removeListing(id: UInt64) {
        if let item = self.listingIDItems[id] {
            // Skip if the listing item hasn't been purchased
            if let storefrontPublic = item.storefrontPublicCapability.borrow() {
                if let listingItem = storefrontPublic.borrowListing(listingResourceID: id) {
                    let listingDetails = listingItem.getDetails()
                    if listingDetails.purchased == false {
                        return
                    }
                }
            }

            self.removeItem(item)
        }
    }

    // Anyone can remove it if the listing item has been removed or purchased.
    pub fun removeListingWithIndex(id: UInt64, indexToRemoveListingID: Int) {
        pre {
            self.listingIDs[indexToRemoveListingID] == id: "invalid indexToRemoveListingID"
        }

        if let item = self.listingIDItems[id] {
            // Skip if the listing item hasn't been purchased
            if let storefrontPublic = item.storefrontPublicCapability.borrow() {
                if let listingItem = storefrontPublic.borrowListing(listingResourceID: id) {
                    let listingDetails = listingItem.getDetails()
                    if listingDetails.purchased == false {
                        return
                    }
                }
            }

            self.removeItemWithIndexes(
                item,
                indexToRemoveListingID: indexToRemoveListingID)
        }
    }

    // Add item and indexes.
    access(contract) fun addItem(
        _ item: Item,
        storefrontPublicCapability: Capability<&{NFTStorefront.StorefrontPublic}>,
        indexToInsertListingID: Int
    ) {
        pre {
            self.listingIDItems[item.listingID] == nil: "could not add duplicate listing"
        }

        assert(item.listingDetails.purchased == false, message: "the item has been purchased")

        // find previous duplicate NFT
        let nftListingIDs = self.collectionNFTListingIDs[item.listingDetails.nftType.identifier]
        var previousItem: Item? = nil
        if let nftListingIDs = nftListingIDs {
            if let listingID = nftListingIDs[item.listingDetails.nftID] {
                previousItem = self.listingIDItems[listingID]!

                // panic only if they're same address
                if previousItem!.storefrontPublicCapability.address == item.storefrontPublicCapability.address {
                    panic("could not add duplicate NFT")
                }
            }
        }

        // check sale cut
        let requirements = self.saleCutRequirements[item.listingDetails.nftType.identifier]
            ?? panic("no SaleCutRequirements")
        for requirement in requirements {
            let saleCutAmount = item.listingDetails.salePrice * requirement.ratio

            var match = false
            for saleCut in item.listingDetails.saleCuts {
                if saleCut.receiver.address == requirement.receiver.address &&
                   saleCut.receiver.borrow()! == requirement.receiver.borrow()! {
                    if saleCut.amount >= saleCutAmount {
                        match = true
                    }
                    break
                }
            }

            assert(match == true, message: "saleCut must follow SaleCutRequirements")
        }

        // all by time
        self.listingIDs.insert(at: indexToInsertListingID, item.listingID)

        // update index data
        self.listingIDItems[item.listingID] = item
        if let nftListingIDs = nftListingIDs {
            nftListingIDs[item.listingDetails.nftID] = item.listingID
            self.collectionNFTListingIDs[item.listingDetails.nftType.identifier] = nftListingIDs
        } else {
            self.collectionNFTListingIDs[item.listingDetails.nftType.identifier] = {item.listingDetails.nftID: item.listingID}
        }

        // remove previous item
        if let previousItem = previousItem {
            self.removeItem(previousItem)
        }
    }

    // Remove item and indexes. The indexes will be found automatically.
    access(contract) fun removeItem(_ item: Item) {
        let indexToRemoveListingID = self.getIndexToRemoveListingID(
            item: item,
            items: self.listingIDs)

        self.removeItemWithIndexes(
            item,
            indexToRemoveListingID: indexToRemoveListingID)
    }

    // Remove item and indexes. The indexes should be checked before calling this func.
    access(contract) fun removeItemWithIndexes(_ item: Item, indexToRemoveListingID: Int?) {
        // remove from listingIDs
        if let indexToRemoveListingID = indexToRemoveListingID {
            self.listingIDs.remove(at: indexToRemoveListingID)
        }

        // update index data
        self.listingIDItems.remove(key: item.listingID)
        let nftListingIDs = self.collectionNFTListingIDs[item.listingDetails.nftType.identifier] ?? {}
        nftListingIDs.remove(key: item.listingDetails.nftID)
        self.collectionNFTListingIDs[item.listingDetails.nftType.identifier] = nftListingIDs
    }

    // Run reverse for loop to find out the index to insert
    access(contract) fun getIndexToAddListingID(item: Item, items: [UInt64]): Int {
        var index = items.length - 1
        while index >= 0 {
            let currentListingID = items[index]
            let currentItem = self.listingIDItems[currentListingID]!

            if item.timestamp == currentItem.timestamp {
                if item.listingID > currentListingID {
                    break
                }
                index = index - 1
            } else {
                break
            }
        }
        return index + 1
    }

    // Run binary search to find the listing ID
    access(contract) fun getIndexToRemoveListingID(item: Item, items: [UInt64]): Int? {
        var startIndex = 0
        var endIndex = items.length

        while startIndex < endIndex {
            var midIndex = startIndex + (endIndex - startIndex) / 2
            var midListingID = items[midIndex]!
            var midItem = self.listingIDItems[midListingID]!

            if item.timestamp > midItem.timestamp {
                startIndex = midIndex + 1
            } else if item.timestamp < midItem.timestamp {
                endIndex = midIndex
            } else {
                if item.listingID > midListingID {
                    startIndex = midIndex + 1
                }  else if item.listingID < midListingID {
                    endIndex = midIndex
                } else {
                    return midIndex
                }
            }
        }
        return nil
    }

    access(contract) fun checkValidIndexToInsert(item: Item, index: Int, items: [UInt64]) {
        if index > 0 {
            let previousListingID = items[index - 1]
            let previousItem = self.listingIDItems[previousListingID]!
            if previousItem.timestamp > item.timestamp {
                panic("invalid index (timestamp)")
            } else if previousItem.timestamp == item.timestamp && previousItem.listingID >= item.listingID {
                panic("invalid index (listingID)")
            }
        }
        if index < items.length {
            let nextListingID = items[index]
            let nextItem = self.listingIDItems[nextListingID]!
            if item.timestamp > nextItem.timestamp {
                panic("invalid index (timestamp)")
            } else if item.timestamp == nextItem.timestamp && item.listingID >= nextItem.listingID {
                panic("invalid index (listingID)")
            }
        }
    }

    init () {
        self.MarketplaceAdminStoragePath = /storage/BloctoBayMarketplaceAdmin

        self.listingIDs = []
        self.listingIDItems = {}
        self.collectionNFTListingIDs = {}
        self.saleCutRequirements = {}

        let admin <- create Administrator()
        self.account.save(<-admin, to: self.MarketplaceAdminStoragePath)
    }
}
