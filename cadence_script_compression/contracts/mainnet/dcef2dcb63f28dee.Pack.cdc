// SPDX-License-Identifier: Apache License 2.0
import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448

import Elvn from 0x6292b23b3eb3f999
import Moments from 0x6292b23b3eb3f999

pub contract Pack {
    // payment
    access(self) let vault: @Elvn.Vault
    // releaseId: [Pack]
    // Pack: [Moments]
    access(self) let salePacks: @{UInt64: [Pack.Token]}

    pub var totalSupply: UInt64
    
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    pub event BuyPack(packId: UInt64, price: UFix64)
    pub event OpenPack(packId: UInt64, momentsIds: [UInt64], address: Address?)

    pub event Deposit(id: UInt64, to: Address?)
    pub event Withdraw(id: UInt64, from: Address?)

    pub resource Token {
        pub let id: UInt64

        pub let releaseId: UInt64
        pub let price: UFix64

        access(self) let momentsMap: @[Moments.NFT]

        pub fun openPacks(): @[Moments.NFT] {
            pre {
                self.momentsMap.length > 0: "There are no moments in the pack"
            }

            let map: @[Moments.NFT] <- []
            let momentsIds: [UInt64] = []
            while self.momentsMap.length > 0 {
                let moment <- self.momentsMap.removeFirst()
                momentsIds.append(moment.id)
                map.append(<- moment)
            }

            emit OpenPack(packId: self.id, momentsIds: momentsIds, address: self.owner?.address)
            return <- map
        }

        destroy () {
            destroy self.momentsMap
        }

        init(tokenId: UInt64, releaseId: UInt64, price: UFix64, momentsMap: @[Moments.NFT]) {
            self.id = tokenId
            self.releaseId = releaseId
            self.price = price
            self.momentsMap <- momentsMap 
        }
    }

    pub resource interface MomentsCollectionPublic {
        pub fun getIds(): [UInt64]
    }
    
    pub resource Collection: MomentsCollectionPublic {
        pub var ownedPacks: @{UInt64: Pack.Token}

        pub fun getIds(): [UInt64] {
            return self.ownedPacks.keys
        }

        pub fun withdraw(withdrawID: UInt64): @Pack.Token {
            let token <- self.ownedPacks.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)
            return <- token
        }

        pub fun deposit(token: @Pack.Token) {
            let id: UInt64 = token.id

            self.ownedPacks[id] <-! token

            emit Deposit(id: id, to: self.owner?.address)
        }

        destroy() {
            destroy self.ownedPacks
        }

        init() {
            self.ownedPacks <- {}
        }
    }

    pub fun isNonExists(releaseId: UInt64): Bool {
        return self.salePacks[releaseId] == nil
    }

    pub fun buyPack(releaseId: UInt64, vault: @FungibleToken.Vault): @Pack.Token { 
        pre {
            !self.isNonExists(releaseId: releaseId): "Not found releaseId: ".concat(releaseId.toString())
        }

        let balance = vault.balance
        self.vault.deposit(from: <- vault)

        let packsRef = &self.salePacks[releaseId] as &[Pack.Token]
        if packsRef.length == 0 {
            return panic("Sold out pack")
        }
        let packRef = &packsRef[0] as &Pack.Token

        if packRef.price > balance {
            return panic("Not enough vault balance")
        }

        let salePacks <- self.salePacks.remove(key: releaseId) ?? panic("unreachable")
        let randomIndex = unsafeRandom() % UInt64(packsRef.length)
        let pack <- salePacks.remove(at: randomIndex)
        self.salePacks[releaseId] <-! salePacks

        emit BuyPack(packId: pack.id, price: pack.price)
        return <- pack
    }

    pub fun getPackRemainingCount(releaseId: UInt64): Int {
        pre {
            !self.isNonExists(releaseId: releaseId): "Not found releaseId: ".concat(releaseId.toString())
        }

        let packsRef = &self.salePacks[releaseId] as? &[Pack.Token]
        return packsRef.length
    }

    pub fun getPackPrice(releaseId: UInt64): UFix64 {
        pre {
            !self.isNonExists(releaseId: releaseId): "Not found releaseId: ".concat(releaseId.toString())
        }

        let packsRef = &self.salePacks[releaseId] as? &[Pack.Token]
        if packsRef.length == 0 {
            return panic("Sold out pack")
        }

        let packRef = &packsRef[0] as? &Pack.Token
        return packRef.price
    }

    pub resource Administrator {
        pub fun addPack(token: @Pack.Token) {
            let releaseId = token.releaseId

            if Pack.salePacks[releaseId] == nil {
                let packs: @[Pack.Token] <- [<- token]                
                Pack.salePacks[releaseId] <-! packs
                return
            }

            let packPrice = Pack.getPackPrice(releaseId: releaseId)
            if packPrice != token.price {
                destroy token
                return panic("Pack price is not equal")
            }

            let packs <- Pack.salePacks.remove(key: releaseId) ?? panic("unreachable")
            packs.append(<- token)
            Pack.salePacks[releaseId] <-! packs
        }

        pub fun createPackToken(releaseId: UInt64, price: UFix64, momentsMap: @[Moments.NFT]): @Pack.Token {
            let pack <- create Pack.Token(tokenId: Pack.totalSupply, releaseId: releaseId, price: price, momentsMap: <- momentsMap)
            Pack.totalSupply = Pack.totalSupply + 1
            return <- pack
        }

        pub fun withdraw(amount: UFix64?): @FungibleToken.Vault {
            if let amount = amount {    
                return <- Pack.vault.withdraw(amount: amount)
            } else {
                let balance = Pack.vault.balance
                return <- Pack.vault.withdraw(amount: balance)
            }
        }
    }

    pub fun createEmptyCollection(): @Pack.Collection {
        return <- create Collection()
    }

    init() {
        self.CollectionStoragePath = /storage/sportiumPackCollection
        self.CollectionPublicPath = /public/sportiumPackCollection

        self.salePacks <- {}
        self.vault <- Elvn.createEmptyVault() as! @Elvn.Vault
        self.totalSupply = 0

        self.account.save(<- create Administrator(), to: /storage/sportiumPackAdministrator)
    }
}
 