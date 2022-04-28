import NonFungibleToken from 0x1d7e57aa55817448
import SoulMadeComponent from 0x543606e9393a64a6
import SoulMadeMain from 0x543606e9393a64a6
import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61

/*
 This contract is based on the Flovatar Marketplace contract
 https://github.com/crash13override/flovatar/blob/main/contracts/FlovatarMarketplace.cdc
*/

pub contract SoulMadeMarketplace {

    pub let CollectionPublicPath: PublicPath
    pub let CollectionStoragePath: StoragePath

    pub let SoulMadePlatformCut: UFix64

    // The Vault of the Marketplace where it will receive the cuts on each sale
    pub let marketplaceWallet: Capability<&FlowToken.Vault{FungibleToken.Receiver}>

    pub event SoulMadeMarketplaceSaleCollectionCreated()

    // Event that is emitted when a new NFT is put up for sale
    pub event SoulMadeMainForSale(id: UInt64, price: UFix64, address: Address)
    pub event SoulMadeComponentForSale(id: UInt64, price: UFix64, address: Address)

    pub event SoulMadeForSale(id: UInt64, nftType: String, address: Address, saleData: SoulMadeSaleData)

    // Event that is emitted when the price of an NFT changes
    pub event SoulMadeMainPriceChanged(id: UInt64, newPrice: UFix64, address: Address)
    pub event SoulMadeComponentPriceChanged(id: UInt64, newPrice: UFix64, address: Address)

    // Event that is emitted when a token is purchased
    pub event SoulMadeMainPurchased(id: UInt64, price: UFix64, from: Address, to: Address)
    pub event SoulMadeComponentPurchased(id: UInt64, price: UFix64, from: Address, to: Address)

    // Event that is emitted when a seller withdraws their NFT from the sale
    pub event SoulMadeMainSaleWithdrawn(tokenId: UInt64, address: Address)
    pub event SoulMadeComponentSaleWithdrawn(tokenId: UInt64, address: Address)

    // Interface that users will publish for their Sale collection
    // that only exposes the methods that are supposed to be public
    pub resource interface SalePublic {
        pub fun purchaseSoulMadeMain(tokenId: UInt64, recipientCap: Capability<&{SoulMadeMain.CollectionPublic}>, buyTokens: @FungibleToken.Vault)
        pub fun purchaseSoulMadeComponent(tokenId: UInt64, recipientCap: Capability<&{SoulMadeComponent.CollectionPublic}>, buyTokens: @FungibleToken.Vault)
        pub fun getSoulMadeMainPrice(tokenId: UInt64): UFix64?
        pub fun getSoulMadeComponentPrice(tokenId: UInt64): UFix64?
        pub fun getSoulMadeMainIDs(): [UInt64]
        pub fun getSoulMadeComponentIDs(): [UInt64]
        pub fun getSoulMadeMain(tokenId: UInt64): &{SoulMadeMain.MainPublic}
        pub fun getSoulMadeComponent(tokenId: UInt64): &{SoulMadeComponent.ComponentPublic}
    }

    // NFT Collection object that allows a user to put their NFT up for sale
    // where others can send fungible tokens to purchase it
    pub resource SaleCollection: SalePublic {

        // Dictionary of the NFTs that the user is putting up for sale
        access(contract) let SoulMadeMainForSale: @{UInt64: SoulMadeMain.NFT}
        access(contract) let SoulMadeComponentForSale: @{UInt64: SoulMadeComponent.NFT}

        // Dictionary of the prices for each NFT by ID
        access(contract) let SoulMadeMainPrices: {UInt64: UFix64}
        access(contract) let SoulMadeComponentPrices: {UInt64: UFix64}

        // The fungible token vault of the owner of this sale.
        // When someone buys a token, this resource can deposit
        // tokens into their account.
        access(account) let ownerVault: Capability<&{FungibleToken.Receiver}>

        init (ownerVault: Capability<&{FungibleToken.Receiver}>) {
            self.SoulMadeMainForSale <- {}
            self.SoulMadeComponentForSale <- {}
            self.ownerVault = ownerVault
            self.SoulMadeMainPrices = {}
            self.SoulMadeComponentPrices = {}
        }

        // Gives the owner the opportunity to remove a SoulMadeMain sale from the collection
        pub fun withdrawSoulMadeMain(tokenId: UInt64): @SoulMadeMain.NFT {
            // remove the price
            self.SoulMadeMainPrices.remove(key: tokenId)
            // remove and return the token
            let token <- self.SoulMadeMainForSale.remove(key: tokenId) ?? panic("missing NFT")

            let vaultRef = self.ownerVault.borrow()
                ?? panic("Could not borrow reference to owner token vault")
            emit SoulMadeMainSaleWithdrawn(tokenId: tokenId, address: vaultRef.owner!.address)
            return <- token
        }

        // Gives the owner the opportunity to remove a Component sale from the collection
        pub fun withdrawSoulMadeComponent(tokenId: UInt64): @SoulMadeComponent.NFT {
            // remove the price
            self.SoulMadeComponentPrices.remove(key: tokenId)
            // remove and return the token
            let token <- self.SoulMadeComponentForSale.remove(key: tokenId) ?? panic("missing NFT")

            let vaultRef = self.ownerVault.borrow()
                ?? panic("Could not borrow reference to owner token vault")
            emit SoulMadeComponentSaleWithdrawn(tokenId: tokenId, address: vaultRef.owner!.address)
            return <- token
        }

        // Lists a SoulMadeMain NFT for sale in this collection
        pub fun listSoulMadeMainForSale(token: @SoulMadeMain.NFT, price: UFix64) {
            let id = token.id

            // store the price in the price array
            self.SoulMadeMainPrices[id] = price

            let saleData: SoulMadeSaleData = SoulMadeSaleData(id: id, price: price, nftType: "SoulMadeMain", mainDetail: token.mainDetail, componentDetail: nil)

            // put the NFT into the the forSale dictionary
            let oldToken <- self.SoulMadeMainForSale[id] <- token
            destroy oldToken

            let vaultRef = self.ownerVault.borrow()
                ?? panic("Could not borrow reference to owner token vault")

            emit SoulMadeMainForSale(id: id, price: price, address: vaultRef.owner!.address)
            emit SoulMadeForSale(id: id, nftType: "SoulMadeMain", address: vaultRef.owner!.address, saleData: saleData)

        }

        // Lists a Component NFT for sale in this collection
        pub fun listSoulMadeComponentForSale(token: @SoulMadeComponent.NFT, price: UFix64) {
            let id = token.id

            // store the price in the price array
            self.SoulMadeComponentPrices[id] = price

            let saleData: SoulMadeSaleData = SoulMadeSaleData(id: id, price: price, nftType: "SoulMadeComponent", mainDetail: nil, componentDetail: token.componentDetail)

            // put the NFT into the the forSale dictionary
            let oldToken <- self.SoulMadeComponentForSale[id] <- token
            destroy oldToken

            let vaultRef = self.ownerVault.borrow()
                ?? panic("Could not borrow reference to owner token vault")
            emit SoulMadeComponentForSale(id: id, price: price, address: vaultRef.owner!.address)
            emit SoulMadeForSale(id: id, nftType: "SoulMadeComponent", address: vaultRef.owner!.address, saleData: saleData)
        }

        // Changes the price of a SoulMadeMain that is currently for sale
        pub fun changeSoulMadeMainPrice(tokenId: UInt64, newPrice: UFix64) {
            self.SoulMadeMainPrices[tokenId] = newPrice

            let vaultRef = self.ownerVault.borrow()
                ?? panic("Could not borrow reference to owner token vault")
            emit SoulMadeMainPriceChanged(id: tokenId, newPrice: newPrice, address: vaultRef.owner!.address)
        }
        // Changes the price of a Component that is currently for sale
        pub fun changeSoulMadeComponentPrice(tokenId: UInt64, newPrice: UFix64) {
            self.SoulMadeComponentPrices[tokenId] = newPrice

            let vaultRef = self.ownerVault.borrow()
                ?? panic("Could not borrow reference to owner token vault")
            emit SoulMadeComponentPriceChanged(id: tokenId, newPrice: newPrice, address: vaultRef.owner!.address)
        }

        // Lets a user send tokens to purchase a SoulMadeMain that is for sale
        pub fun purchaseSoulMadeMain(tokenId: UInt64, recipientCap: Capability<&{SoulMadeMain.CollectionPublic}>, buyTokens: @FungibleToken.Vault) {
            pre {
                self.SoulMadeMainForSale[tokenId] != nil && self.SoulMadeMainPrices[tokenId] != nil:
                    "No token matching this ID for sale!"
                buyTokens.balance >= (self.SoulMadeMainPrices[tokenId] ?? 0.0):
                    "Not enough tokens to buy the NFT!"
            }

            let recipient = recipientCap.borrow()!

            // get the value out of the optional
            let price = self.SoulMadeMainPrices[tokenId]!

            self.SoulMadeMainPrices[tokenId] = nil

            let vaultRef = self.ownerVault.borrow() ?? panic("Could not borrow reference to owner token vault")

            let nft <- self.withdrawSoulMadeMain(tokenId: tokenId)
            
            let marketplaceWallet = SoulMadeMarketplace.marketplaceWallet.borrow()!
            let marketplaceAmount = price * SoulMadeMarketplace.SoulMadePlatformCut
            let tempMarketplaceWallet <- buyTokens.withdraw(amount: marketplaceAmount)
            marketplaceWallet.deposit(from: <- tempMarketplaceWallet)

            // deposit the purchasing tokens into the owners vault
            vaultRef.deposit(from: <- buyTokens)

            // deposit the NFT into the buyers collection
            recipient.deposit(token: <- nft)

            emit SoulMadeMainPurchased(id: tokenId, price: price, from: vaultRef.owner!.address, to: recipient.owner!.address)
        }

        // Lets a user send tokens to purchase a Component that is for sale
        pub fun purchaseSoulMadeComponent(tokenId: UInt64, recipientCap: Capability<&{SoulMadeComponent.CollectionPublic}>, buyTokens: @FungibleToken.Vault) {
            pre {
                self.SoulMadeComponentForSale[tokenId] != nil && self.SoulMadeComponentPrices[tokenId] != nil:
                    "No token matching this ID for sale!"
                buyTokens.balance >= (self.SoulMadeComponentPrices[tokenId] ?? 0.0):
                    "Not enough tokens to buy the NFT!"
            }

            let recipient = recipientCap.borrow()!

            // get the value out of the optional
            let price = self.SoulMadeComponentPrices[tokenId]!

            self.SoulMadeComponentPrices[tokenId] = nil

            let vaultRef = self.ownerVault.borrow() ?? panic("Could not borrow reference to owner token vault")

            let nft <-self.withdrawSoulMadeComponent(tokenId: tokenId)

            let marketplaceWallet = SoulMadeMarketplace.marketplaceWallet.borrow()!
            let marketplaceAmount = price * SoulMadeMarketplace.SoulMadePlatformCut
            let tempMarketplaceWallet <- buyTokens.withdraw(amount: marketplaceAmount)
            marketplaceWallet.deposit(from: <-tempMarketplaceWallet)

            // deposit the purchasing tokens into the owners vault
            vaultRef.deposit(from: <- buyTokens)

            // deposit the NFT into the buyers collection
            recipient.deposit(token: <- nft)

            emit SoulMadeComponentPurchased(id: tokenId, price: price, from: vaultRef.owner!.address, to: recipient.owner!.address)
        }

        // Returns the price of a specific SoulMadeMain in the sale
        pub fun getSoulMadeMainPrice(tokenId: UInt64): UFix64? {
            return self.SoulMadeMainPrices[tokenId]
        }
        // Returns the price of a specific Component in the sale
        pub fun getSoulMadeComponentPrice(tokenId: UInt64): UFix64? {
            return self.SoulMadeComponentPrices[tokenId]
        }

        // Returns an array of SoulMadeMain IDs that are for sale
        pub fun getSoulMadeMainIDs(): [UInt64] {
            return self.SoulMadeMainForSale.keys
        }
        // Returns an array of Component IDs that are for sale
        pub fun getSoulMadeComponentIDs(): [UInt64] {
            return self.SoulMadeComponentForSale.keys
        }

        // Returns a borrowed reference to a SoulMadeMain Sale
        // so that the caller can read data and call methods from it.
        pub fun getSoulMadeMain(tokenId: UInt64): &{SoulMadeMain.MainPublic} {
            pre {
                self.SoulMadeMainForSale[tokenId] != nil: "Main NFT doesn't exist"
            }
            let ref = &self.SoulMadeMainForSale[tokenId] as auth &NonFungibleToken.NFT
            return ref as! &SoulMadeMain.NFT

        }
        // Returns a borrowed reference to a Component Sale
        // so that the caller can read data and call methods from it.
        pub fun getSoulMadeComponent(tokenId: UInt64): &{SoulMadeComponent.ComponentPublic} {
            pre {
                self.SoulMadeComponentForSale[tokenId] != nil: "Component NFT doesn't exist"
            }
            let ref = &self.SoulMadeComponentForSale[tokenId] as auth &NonFungibleToken.NFT
            return ref as! &SoulMadeComponent.NFT
        }

        destroy() {
            destroy self.SoulMadeMainForSale
            destroy self.SoulMadeComponentForSale
        }
    }

    pub struct SoulMadeMainSaleData {
        pub let id: UInt64
        pub let price: UFix64
        pub let mainDetail: SoulMadeMain.MainDetail

        init(
            id: UInt64,
            price: UFix64,
            mainDetail: SoulMadeMain.MainDetail){
            self.id = id
            self.price = price
            self.mainDetail = mainDetail
        }
    }

    // Get a specific SoulMadeMain Sale offers for an account
    pub fun getSoulMadeMainSale(address: Address, id: UInt64) : SoulMadeMainSaleData {
        let account = getAccount(address)

        let saleCollection = account.getCapability(self.CollectionPublicPath).borrow<&{SoulMadeMarketplace.SalePublic}>()!
        let soulMadeMain = saleCollection.getSoulMadeMain(tokenId: id)
        let price = saleCollection.getSoulMadeMainPrice(tokenId: id)
        return SoulMadeMainSaleData(
                    id: id,
                    price: price!,
                    mainDetail: soulMadeMain.mainDetail
                    )
    }

    // Get all the SoulMadeMain Sale offers for a specific account
    pub fun getSoulMadeMainSales(address: Address) : [SoulMadeMainSaleData] {
        var saleData: [SoulMadeMainSaleData] = []
        let account = getAccount(address)

        let saleCollection = account.getCapability(self.CollectionPublicPath).borrow<&{SoulMadeMarketplace.SalePublic}>()!
        for id in saleCollection.getSoulMadeMainIDs() {
            let price = saleCollection.getSoulMadeMainPrice(tokenId: id)
            let soulMadeMain = saleCollection.getSoulMadeMain(tokenId: id)
            saleData.append(SoulMadeMainSaleData(
                id: id,
                price: price!,
                mainDetail: soulMadeMain.mainDetail
                ))
        }
        return saleData
    }

    // This struct is used to send a data representation of the Component Sales 
    pub struct SoulMadeComponentSaleData {
        pub let id: UInt64
        pub let price: UFix64
        pub let componentDetail: SoulMadeComponent.ComponentDetail

        init(
            id: UInt64,
            price: UFix64,
            componentDetail: SoulMadeComponent.ComponentDetail){
            self.id = id
            self.price = price
            self.componentDetail = componentDetail
        }
    }

    // Get a specific Component Sale offers for an account
    pub fun getSoulMadeComponentSale(address: Address, id: UInt64) : SoulMadeComponentSaleData {
        let account = getAccount(address)

        let saleCollection = account.getCapability(self.CollectionPublicPath).borrow<&{SoulMadeMarketplace.SalePublic}>()!
        let soulMadeComponent = saleCollection.getSoulMadeComponent(tokenId: id)
        let price = saleCollection.getSoulMadeComponentPrice(tokenId: id)
        return SoulMadeComponentSaleData(
                    id: id,
                    price: price!,
                    componentDetail: soulMadeComponent.componentDetail
                    )
    }

    // Get all the Component Sale offers for a specific account
    pub fun getSoulMadeComponentSales(address: Address) : [SoulMadeComponentSaleData] {
        var saleData: [SoulMadeComponentSaleData] = []
        let account = getAccount(address)

        let saleCollection = account.getCapability(self.CollectionPublicPath).borrow<&{SoulMadeMarketplace.SalePublic}>()!
        for id in saleCollection.getSoulMadeComponentIDs() {
            let price = saleCollection.getSoulMadeComponentPrice(tokenId: id)
            let soulMadeComponent = saleCollection.getSoulMadeComponent(tokenId: id)
            saleData.append(SoulMadeComponentSaleData(
                id: id,
                price: price!,
                componentDetail: soulMadeComponent.componentDetail
                ))
        }
        return saleData
    }

    pub struct SoulMadeSaleData {
        pub let id: UInt64
        pub let price: UFix64
        pub let nftType: String
        pub let mainDetail: SoulMadeMain.MainDetail?
        pub let componentDetail: SoulMadeComponent.ComponentDetail?

        init(
            id: UInt64,
            price: UFix64,
            nftType: String,
            mainDetail: SoulMadeMain.MainDetail?,
            componentDetail: SoulMadeComponent.ComponentDetail?){

            self.id = id
            self.price = price
            self.nftType = nftType
            self.mainDetail = mainDetail
            self.componentDetail = componentDetail
        }
    }

    pub fun getSoulMadeSales(address: Address) : [SoulMadeSaleData] {
        var saleData: [SoulMadeSaleData] = []
        let account = getAccount(address)

        let saleCollection = account.getCapability(self.CollectionPublicPath).borrow<&{SoulMadeMarketplace.SalePublic}>()!

        for id in saleCollection.getSoulMadeMainIDs() {
            let price = saleCollection.getSoulMadeMainPrice(tokenId: id)
            let soulMadeMain = saleCollection.getSoulMadeMain(tokenId: id)
            saleData.append(SoulMadeSaleData(
                id: id,
                price: price!,
                nftType: "SoulMadeMain",
                mainDetail: soulMadeMain.mainDetail,
                componentDetail: nil
                ))
        }

        for id in saleCollection.getSoulMadeComponentIDs() {
            let price = saleCollection.getSoulMadeComponentPrice(tokenId: id)
            let soulMadeComponent = saleCollection.getSoulMadeComponent(tokenId: id)
            saleData.append(SoulMadeSaleData(
                id: id,
                price: price!,
                nftType: "SoulMadeComponent",
                mainDetail: nil,
                componentDetail: soulMadeComponent.componentDetail
                ))
        }

        return saleData
    }

    pub fun convertSoulMadeMainSaleToSoulMadeSale(mainSale: SoulMadeMainSaleData): SoulMadeSaleData{
        return SoulMadeSaleData(
                    id: mainSale.id,
                    price: mainSale.price,
                    nftType: "SoulMadeMain",
                    mainDetail: mainSale.mainDetail,
                    componentDetail: nil
                )
    }

    pub fun convertSoulMadeComponentSaleToSoulMadeSale(componentSale: SoulMadeComponentSaleData): SoulMadeSaleData{
        return SoulMadeSaleData(
                    id: componentSale.id,
                    price: componentSale.price,
                    nftType: "SoulMadeComponent",
                    mainDetail: nil,
                    componentDetail: componentSale.componentDetail
                )
    }    

    // Returns a new collection resource to the caller
    pub fun createSaleCollection(ownerVault: Capability<&{FungibleToken.Receiver}>): @SaleCollection {
        emit SoulMadeMarketplaceSaleCollectionCreated()
        return <- create SaleCollection(ownerVault: ownerVault)
    }
    

    pub init() {
        self.CollectionPublicPath = /public/SoulMadeMarketplace
        self.CollectionStoragePath = /storage/SoulMadeMarketplace
        self.SoulMadePlatformCut = 0.15

        self.marketplaceWallet = self.account.getCapability<&FlowToken.Vault{FungibleToken.Receiver}>(/public/flowTokenReceiver)
    }
}