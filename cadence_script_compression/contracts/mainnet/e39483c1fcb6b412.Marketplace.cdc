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
    access(contract) let listingIDsByTime: [UInt64]

    // listingID order by price, listingID asc
    access(contract) let listingIDsByPrice: [UInt64]

    // collection identifier => listingIDs order by time, listingID asc
    access(contract) let collectionListingIDsByTime: {String: [UInt64]}

    // collection identifier => listingIDs order by price, listingID asc
    access(contract) let collectionListingIDsByPrice: {String: [UInt64]}

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

    pub fun getListingIDsByTime(): [UInt64] {
        return self.listingIDsByTime
    }

    pub fun getListingIDsByPrice(): [UInt64] {
        return self.listingIDsByPrice
    }

    pub fun getCollectionListingIDsByTime(nftType: Type): [UInt64] {
        return self.collectionListingIDsByTime[nftType.identifier] ?? []
    }

    pub fun getCollectionListingIDsByPrice(nftType: Type): [UInt64] {
        return self.collectionListingIDsByPrice[nftType.identifier] ?? []
    }

    pub fun getListingIDItem(listingID: UInt64): Item? {
        return self.listingIDItems[listingID]
    }

    pub fun getListingID(nftType: Type, nftID: UInt64): UInt64? {
        let nftListingIDs = self.collectionNFTListingIDs[nftType.identifier] ?? {}
        return nftListingIDs[nftID]
    }

    pub fun getSaleCutRequirements(nftType: Type): [SaleCutRequirement] {
        return self.saleCutRequirements[nftType.identifier] ?? []
    }

    pub fun addListing(id: UInt64, storefrontPublicCapability: Capability<&{NFTStorefront.StorefrontPublic}>) {
        let item = Item(storefrontPublicCapability: storefrontPublicCapability, listingID: id)

        // all by time
        let indexToInsertListingIDByTime = self.getIndexToAddListingIDByTime(item: item, items: self.listingIDsByTime)

        // all by price
        let indexToInsertListingIDByPrice = self.getIndexToAddListingIDByPrice(item: item, items: self.listingIDsByPrice)
            ?? panic("could not add duplicate listing")

        // collection by time
        var items = self.collectionListingIDsByTime[item.listingDetails.nftType.identifier] ?? []
        let indexToInsertCollectionListingIDByTime = self.getIndexToAddListingIDByTime(item: item, items: items)

        // collection by price
        items = self.collectionListingIDsByPrice[item.listingDetails.nftType.identifier] ?? []
        let indexToInsertCollectionListingIDByPrice = self.getIndexToAddListingIDByPrice(item: item, items: items)
            ?? panic("could not add duplicate listing")

        self.addItem(
            item,
            storefrontPublicCapability: storefrontPublicCapability,
            indexToInsertListingIDByTime: indexToInsertListingIDByTime,
            indexToInsertListingIDByPrice: indexToInsertListingIDByPrice,
            indexToInsertCollectionListingIDByTime: indexToInsertCollectionListingIDByTime,
            indexToInsertCollectionListingIDByPrice: indexToInsertCollectionListingIDByPrice)
    }

    pub fun addListingWithIndexes(
        id: UInt64,
        storefrontPublicCapability: Capability<&{NFTStorefront.StorefrontPublic}>,
        indexToInsertListingIDByTime: Int,
        indexToInsertListingIDByPrice: Int,
        indexToInsertCollectionListingIDByTime: Int,
        indexToInsertCollectionListingIDByPrice: Int,
    ) {
        let item = Item(storefrontPublicCapability: storefrontPublicCapability, listingID: id)

        // check indexes
        self.checkValidIndexToInsertByTime(
            item: item,
            index: indexToInsertListingIDByTime,
            items: self.listingIDsByTime)
        self.checkValidIndexToInsertByPrice(
            item: item,
            index: indexToInsertListingIDByPrice,
            items: self.listingIDsByPrice)
        self.checkValidIndexToInsertByTime(
            item: item,
            index: indexToInsertCollectionListingIDByTime,
            items: self.collectionListingIDsByTime[item.listingDetails.nftType.identifier] ?? [])
        self.checkValidIndexToInsertByPrice(
            item: item,
            index: indexToInsertCollectionListingIDByPrice,
            items: self.collectionListingIDsByPrice[item.listingDetails.nftType.identifier] ?? [])

        self.addItem(
            item,
            storefrontPublicCapability: storefrontPublicCapability,
            indexToInsertListingIDByTime: indexToInsertListingIDByTime,
            indexToInsertListingIDByPrice: indexToInsertListingIDByPrice,
            indexToInsertCollectionListingIDByTime: indexToInsertCollectionListingIDByTime,
            indexToInsertCollectionListingIDByPrice: indexToInsertCollectionListingIDByPrice)
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
    pub fun removeListingWithIndexes(
        id: UInt64,
        indexToRemoveListingIDByTime: Int,
        indexToRemoveListingIDByPrice: Int,
        indexToRemoveCollectionListingIDByTime: Int,
        indexToRemoveCollectionListingIDByPrice: Int,
    ) {
        pre {
            self.listingIDsByTime[indexToRemoveListingIDByTime] == id: "invalid indexToRemoveListingIDByTime"
            self.listingIDsByPrice[indexToRemoveListingIDByPrice] == id: "invalid indexToRemoveListingIDByPrice"
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

            var items = self.collectionListingIDsByTime[item.listingDetails.nftType.identifier] ?? []
            assert(items[indexToRemoveCollectionListingIDByTime] == id, message: "invalid indexToRemoveCollectionListingIDByTime")
            items = self.collectionListingIDsByPrice[item.listingDetails.nftType.identifier] ?? []
            assert(items[indexToRemoveCollectionListingIDByPrice] == id, message: "invalid indexToRemoveCollectionListingIDByPrice")

            self.removeItemWithIndexes(
                item,
                indexToRemoveListingIDByTime: indexToRemoveListingIDByTime,
                indexToRemoveListingIDByPrice: indexToRemoveListingIDByPrice,
                indexToRemoveCollectionListingIDByTime: indexToRemoveCollectionListingIDByTime,
                indexToRemoveCollectionListingIDByPrice: indexToRemoveCollectionListingIDByPrice)
        }
    }

    // Add item and indexes.
    access(contract) fun addItem(
        _ item: Item,
        storefrontPublicCapability: Capability<&{NFTStorefront.StorefrontPublic}>,
        indexToInsertListingIDByTime: Int,
        indexToInsertListingIDByPrice: Int,
        indexToInsertCollectionListingIDByTime: Int,
        indexToInsertCollectionListingIDByPrice: Int
    ) {
        pre {
            self.listingIDItems[item.listingID] == nil: "could not add duplicate listing"
        }

        assert(item.listingDetails.purchased == false, message: "the item has been purchased")

        // check duplicate NFT
        let nftListingIDs = self.collectionNFTListingIDs[item.listingDetails.nftType.identifier]
        if let nftListingIDs = nftListingIDs {
            assert(nftListingIDs[item.listingDetails.nftID] == nil, message: "could not add duplicate NFT")
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
        self.listingIDsByTime.insert(at: indexToInsertListingIDByTime, item.listingID)

        // all by price
        self.listingIDsByPrice.insert(at: indexToInsertListingIDByPrice, item.listingID)

        // collection by time
        if let items = self.collectionListingIDsByTime[item.listingDetails.nftType.identifier] {
            items.insert(at: indexToInsertCollectionListingIDByTime, item.listingID)
            self.collectionListingIDsByTime[item.listingDetails.nftType.identifier] = items
        } else {
            self.collectionListingIDsByTime[item.listingDetails.nftType.identifier] = [item.listingID]
        }

        // collection by price
        if let items = self.collectionListingIDsByPrice[item.listingDetails.nftType.identifier] {
            items.insert(at: indexToInsertCollectionListingIDByPrice, item.listingID)
            self.collectionListingIDsByPrice[item.listingDetails.nftType.identifier] = items
        } else {
            self.collectionListingIDsByPrice[item.listingDetails.nftType.identifier] = [item.listingID]
        }

        // update index data
        self.listingIDItems[item.listingID] = item
        if let nftListingIDs = nftListingIDs {
            nftListingIDs[item.listingDetails.nftID] = item.listingID
            self.collectionNFTListingIDs[item.listingDetails.nftType.identifier] = nftListingIDs
        } else {
            self.collectionNFTListingIDs[item.listingDetails.nftType.identifier] = {item.listingDetails.nftID: item.listingID}
        }
    }

    // Remove item and indexes. The indexes will be found automatically.
    access(contract) fun removeItem(_ item: Item) {
        let indexToRemoveListingIDByTime = self.getIndexToRemoveListingIDByTime(
            item: item,
            items: self.listingIDsByTime)
        let indexToRemoveListingIDByPrice = self.getIndexToRemoveListingIDByPrice(
            item: item,
            items: self.listingIDsByPrice)
        let indexToRemoveCollectionListingIDByTime = self.getIndexToRemoveListingIDByTime(
            item: item,
            items: self.collectionListingIDsByTime[item.listingDetails.nftType.identifier] ?? [])
        let indexToRemoveCollectionListingIDByPrice = self.getIndexToRemoveListingIDByPrice(
            item: item,
            items: self.collectionListingIDsByPrice[item.listingDetails.nftType.identifier] ?? [])

        self.removeItemWithIndexes(
            item,
            indexToRemoveListingIDByTime: indexToRemoveListingIDByTime,
            indexToRemoveListingIDByPrice: indexToRemoveListingIDByPrice,
            indexToRemoveCollectionListingIDByTime: indexToRemoveCollectionListingIDByTime,
            indexToRemoveCollectionListingIDByPrice: indexToRemoveCollectionListingIDByPrice)
    }

    // Remove item and indexes. The indexes should be checked before calling this func.
    access(contract) fun removeItemWithIndexes(
        _ item: Item,
        indexToRemoveListingIDByTime: Int?,
        indexToRemoveListingIDByPrice: Int?,
        indexToRemoveCollectionListingIDByTime: Int?,
        indexToRemoveCollectionListingIDByPrice: Int?
    ) {
        // remove from listingIDsByTime
        if let indexToRemoveListingIDByTime = indexToRemoveListingIDByTime {
            self.listingIDsByTime.remove(at: indexToRemoveListingIDByTime)
        }

        // remove from listingIDsByPrice
        if let indexToRemoveListingIDByPrice = indexToRemoveListingIDByPrice {
            self.listingIDsByPrice.remove(at: indexToRemoveListingIDByPrice)
        }

        // remove from collectionListingIDsByTime
        if let indexToRemoveCollectionListingIDByTime = indexToRemoveCollectionListingIDByTime {
            var items = self.collectionListingIDsByTime[item.listingDetails.nftType.identifier] ?? []
            items.remove(at: indexToRemoveCollectionListingIDByTime)
            self.collectionListingIDsByTime[item.listingDetails.nftType.identifier] = items
        }

        // remove from collectionListingIDsByPrice
        if let indexToRemoveCollectionListingIDByPrice = indexToRemoveCollectionListingIDByPrice {
            var items = self.collectionListingIDsByPrice[item.listingDetails.nftType.identifier] ?? []
            items.remove(at: indexToRemoveCollectionListingIDByPrice)
            self.collectionListingIDsByPrice[item.listingDetails.nftType.identifier] = items
        }

        // update index data
        self.listingIDItems.remove(key: item.listingID)
        let nftListingIDs = self.collectionNFTListingIDs[item.listingDetails.nftType.identifier] ?? {}
        nftListingIDs.remove(key: item.listingDetails.nftID)
        self.collectionNFTListingIDs[item.listingDetails.nftType.identifier] = nftListingIDs
    }

    // Run binary search to find out the index to insert
    access(contract) fun getIndexToAddListingIDByPrice(item: Item, items: [UInt64]): Int? {
        var startIndex = 0
        var endIndex = items.length

        while startIndex < endIndex {
            var midIndex = startIndex + (endIndex - startIndex) / 2
            var midListingID = items[midIndex]!
            var midItem = self.listingIDItems[midListingID]!

            if item.listingDetails.salePrice > midItem.listingDetails.salePrice {
                startIndex = midIndex + 1
            } else if item.listingDetails.salePrice < midItem.listingDetails.salePrice {
                endIndex = midIndex
            } else {
                if item.listingID > midListingID {
                    startIndex = midIndex + 1
                }  else if item.listingID < midListingID {
                    endIndex = midIndex
                } else {
                    return nil
                }
            }
        }
        return startIndex
    }

    // Run reverse for loop to find out the index to insert
    access(contract) fun getIndexToAddListingIDByTime(item: Item, items: [UInt64]): Int {
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
    access(contract) fun getIndexToRemoveListingIDByTime(item: Item, items: [UInt64]): Int? {
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

    // Run binary search to find the listing ID
    access(contract) fun getIndexToRemoveListingIDByPrice(item: Item, items: [UInt64]): Int? {
        var startIndex = 0
        var endIndex = items.length

        while startIndex < endIndex {
            var midIndex = startIndex + (endIndex - startIndex) / 2
            var midListingID = items[midIndex]!
            var midItem = self.listingIDItems[midListingID]!

            if item.listingDetails.salePrice > midItem.listingDetails.salePrice {
                startIndex = midIndex + 1
            } else if item.listingDetails.salePrice < midItem.listingDetails.salePrice {
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

    access(contract) fun checkValidIndexToInsertByTime(item: Item, index: Int, items: [UInt64]) {
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

    access(contract) fun checkValidIndexToInsertByPrice(item: Item, index: Int, items: [UInt64]) {
        if index > 0 {
            let previousListingID = items[index - 1]
            let previousItem = self.listingIDItems[previousListingID]!
            if previousItem.listingDetails.salePrice > item.listingDetails.salePrice {
                panic("invalid index (salePrice)")
            } else if previousItem.listingDetails.salePrice == item.listingDetails.salePrice && previousItem.listingID >= item.listingID {
                panic("invalid index (listingID)")
            }
        }
        if index < items.length {
            let nextListingID = items[index]
            let nextItem = self.listingIDItems[nextListingID]!
            if item.listingDetails.salePrice > nextItem.listingDetails.salePrice {
                panic("invalid index (salePrice)")
            } else if item.listingDetails.salePrice == nextItem.listingDetails.salePrice && item.listingID >= nextItem.listingID {
                panic("invalid index (listingID)")
            }
        }
    }

    init () {
        self.MarketplaceAdminStoragePath = /storage/BloctoBayMarketplaceAdmin

        self.listingIDsByTime = []
        self.listingIDsByPrice = []
        self.collectionListingIDsByTime = {}
        self.collectionListingIDsByPrice = {}
        self.listingIDItems = {}
        self.collectionNFTListingIDs = {}
        self.saleCutRequirements = {}

        let admin <- create Administrator()
        self.account.save(<-admin, to: self.MarketplaceAdminStoragePath)
    }
}
