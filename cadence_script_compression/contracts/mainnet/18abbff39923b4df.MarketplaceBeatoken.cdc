// SPDX-License-Identifier: UNLICENSED
import FungibleBeatoken from 0x18abbff39923b4df
import NonFungibleToken from 0x1d7e57aa55817448
import NonFungibleBeatoken from 0x18abbff39923b4df

pub contract MarketplaceBeatoken {

    pub let publicSale: PublicPath
    pub let storageSale: StoragePath

    pub event ForSale(id: UInt64, price: UFix64)
    pub event PriceChanged(id: UInt64, newPrice: UFix64)
    pub event TokenPurchased(id: UInt64, price: UFix64)
    pub event SaleWithdrawn(id: UInt64)

    pub resource interface SalePublic {
        pub fun purchase(tokenID: UInt64, recipient: &NonFungibleBeatoken.Collection, buyTokens: @FungibleBeatoken.Vault)
        pub fun idPrice(tokenID: UInt64): UFix64?
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT 
        pub fun getIDs(): [UInt64]
    }

    pub resource SaleCollection: SalePublic {

        access(self) let ownerCollection: Capability<&NonFungibleBeatoken.Collection>

        access(self) let ownerVault: Capability<&FungibleBeatoken.Vault>

        access(self) let prices: {UInt64: UFix64}

        pub var forSale: @{UInt64: NonFungibleToken.NFT}

        init (collection: Capability<&NonFungibleBeatoken.Collection>,
              vault: Capability<&FungibleBeatoken.Vault>) {

            pre {
                collection.check():
                    "Owner's Moment Collection Capability is invalid!"

                vault.check():
                    "Owner's Receiver Capability is invalid!"
            }

            self.forSale <- {}
            self.prices = {}

            self.ownerCollection = collection
            self.ownerVault = vault
        }

        pub fun withdraw(tokenID: UInt64): @NonFungibleToken.NFT {
            self.prices.remove(key: tokenID)
            let token <- self.forSale.remove(key: tokenID) ?? panic("missing NFT")
            emit SaleWithdrawn(id: tokenID)
            return <-token
        }

        pub fun listForSale(token: @NonFungibleToken.NFT, price: UFix64) {
            let id = token.id

            self.prices[id] = price

            let oldToken <- self.forSale[id] <- token
            destroy oldToken

            emit ForSale(id: id, price: price)
        }

        pub fun changePrice(tokenID: UInt64, newPrice: UFix64) {
            self.prices[tokenID] = newPrice

            emit PriceChanged(id: tokenID, newPrice: newPrice)
        }

        pub fun purchase(tokenID: UInt64, recipient: &NonFungibleBeatoken.Collection, buyTokens: @FungibleBeatoken.Vault) {
            pre {
                self.forSale[tokenID] != nil && self.prices[tokenID] != nil:
                    "No token matching this ID for sale!"
                buyTokens.balance >= (self.prices[tokenID] ?? 0.0):
                    "Not enough tokens to by the NFT!"
            }

            let price = self.prices[tokenID]!

            self.prices[tokenID] = nil

            let vaultRef = self.ownerVault.borrow()
                ?? panic("Could not borrow reference to owner token vault")
                
            vaultRef.deposit(from: <-buyTokens)

            recipient.deposit(token: <-self.withdraw(tokenID: tokenID))

            emit TokenPurchased(id: tokenID, price: price)
        }

        pub fun idPrice(tokenID: UInt64): UFix64? {
            return self.prices[tokenID]
        }

        pub fun getIDs(): [UInt64] {
            return self.forSale.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.forSale[id] as &NonFungibleToken.NFT
        }

        pub fun cancelSale(tokenID: UInt64, recipient: &NonFungibleBeatoken.Collection) {
            pre {
                self.prices[tokenID] != nil: "Token with the specified ID is not already for sale"
            }

            self.prices.remove(key: tokenID)
            self.prices[tokenID] = nil

            recipient.deposit(token: <-self.withdraw(tokenID: tokenID))
        }

        destroy() {
            destroy self.forSale
        }
    }

    pub fun createSaleCollection(
                ownerCollection: Capability<&NonFungibleBeatoken.Collection>,
                ownerVault: Capability<&FungibleBeatoken.Vault>
            ): @SaleCollection {

        return <- create SaleCollection(collection: ownerCollection, vault: ownerVault)
    }

    init() {
        self.publicSale = /public/beatokenNFTSale
        self.storageSale = /storage/beatokenNFTSale
    }
}
