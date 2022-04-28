import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393
import CheezeNFT from 0x5a8fb12692f5a446
import NonFungibleToken from 0x1d7e57aa55817448


pub contract CheezeVouchers {
    pub var totalVoucherSupply: UInt64

    access(self) var cutPercent: UFix64
    access(self) var cutReceiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>


    access(self) var listings: {UInt64: ListingDetails}
    access(self) var storedNFTs: @{UInt64: [CheezeNFT.NFT]}
    access(self) var vouchersAvailable: {UInt64: UInt64}
    access(self) var voucherUsed: {UInt64: Bool}

    pub event ListingCreated(listingId: UInt64)
    pub event VoucherBought(voucherId: UInt64)
    pub event VoucherFulfilled(
        nftId: UInt64, voucherId: UInt64, listingId: UInt64
    )
    pub event DepositVoucher(voucherId: UInt64)

    pub resource Voucher {
        pub let id: UInt64
        pub let listingId: UInt64

        init(
            id: UInt64, listingId: UInt64
        ) {
            self.id = id
            self.listingId = listingId
        }
    }

    pub resource VoucherCollection {
        access(contract) var ownedVouchers: @{UInt64: Voucher}

        pub fun deposit(voucher: @Voucher) {
            let id: UInt64 = voucher.id
            // add the new token to the dictionary which removes the old one
            let oldVoucher <- self.ownedVouchers[id] <- voucher
            emit DepositVoucher(voucherId: id)
            destroy oldVoucher
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedVouchers.keys
        }

        pub fun listingIdForVoucherWithId(voucherId: UInt64): UInt64{
            pre {
                self.ownedVouchers.keys.contains(voucherId)
            }
            post {
                self.ownedVouchers.keys.contains(voucherId)
            }
            let voucherRef = &self.ownedVouchers[voucherId] as &Voucher
            return voucherRef.listingId
        }

        destroy() {
            destroy self.ownedVouchers
        }

        init () {
            self.ownedVouchers <- {}
        }
    }

    pub resource Administrator {
        pub fun fulfillVoucher(address: Address, voucherId: UInt64, random: UInt64) {
            pre {
                getAccount(address).getCapability<&CheezeVouchers.VoucherCollection>(
                    /public/VoucherCollection
                ).borrow()!.getIDs().contains(voucherId)
                !CheezeVouchers.voucherUsed[voucherId]!
            }
            post {
                CheezeVouchers.voucherUsed[voucherId]!
            }
            let voucherCollection = getAccount(
                address
                ).getCapability<&CheezeVouchers.VoucherCollection>(
                    /public/VoucherCollection
                ).borrow()!
            let listingId = voucherCollection.listingIdForVoucherWithId(voucherId: voucherId)

            let stored <- CheezeVouchers.storedNFTs.remove(key: listingId)!

            let randomNftIndex = random % UInt64(
                stored.length
            )
            let token <- stored.remove(at: randomNftIndex)
            let nftId = token.id

            let receiver = getAccount (
                address
                ).getCapability<&CheezeNFT.Collection{NonFungibleToken.Receiver}>(
                    /public/CheezeNFTReceiver
                ).borrow()!
            receiver.deposit(token: <-token)

            CheezeVouchers.voucherUsed[voucherId] = true
            emit VoucherFulfilled(
                nftId: nftId, voucherId: voucherId, listingId: listingId
            )

            CheezeVouchers.storedNFTs[listingId] <-! stored
        }
    }

    pub resource ListingCreator {
        pub let listingId: UInt64
        pub var nfts: @[CheezeNFT.NFT]

        init(price: UFix64, sellerPaymentReceiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>){
            self.listingId = UInt64(CheezeVouchers.listings.length) + 1
            self.nfts <- []
            CheezeVouchers.listings[self.listingId] = ListingDetails(
                listingId: self.listingId,
                price: price, sellerPaymentReceiver: sellerPaymentReceiver,
            )
        }

        pub fun putOnSale(
            tokens: @[CheezeNFT.NFT],
        ){
            var i = 0 as UInt64
            let len = tokens.length
            while i < UInt64(len) {
                let nft <- tokens.removeFirst()
                self.nfts.append(<-nft)
                i = i + 1
            }
            destroy tokens
        }

        destroy(){
            CheezeVouchers.vouchersAvailable[self.listingId] = UInt64(
                self.nfts.length
            )
            CheezeVouchers.storedNFTs[self.listingId] <-! self.nfts
            emit ListingCreated(listingId: self.listingId)
        }
    }

    pub struct ListingDetails {
        pub var listingId: UInt64
        pub var price: UFix64
        pub var sellerPaymentReceiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>

        init(
            listingId: UInt64,
            price: UFix64,
            sellerPaymentReceiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
        ) {
            self.listingId = listingId
            self.price = price
            self.sellerPaymentReceiver = sellerPaymentReceiver
        }
    }

    pub fun priceFor(listingId: UInt64): UFix64 {
        return self.listings[listingId]!.price
    }
    pub fun availableVouchers(listingId: UInt64): UInt64 {
        return self.vouchersAvailable[listingId]!
    }

    pub fun buyVoucher(listingId: UInt64, payment: @FUSD.Vault): @CheezeVouchers.Voucher{
        pre {
            payment.balance == self.priceFor(listingId: listingId)
            self.availableVouchers(listingId: listingId) > 0
        }
        post {
        }

        self.vouchersAvailable[listingId] = (
            self.vouchersAvailable[listingId]! - 1
        )

        let price = self.priceFor(listingId: listingId)
        let listing = self.listings[listingId]!
        let receiver = listing.sellerPaymentReceiver.borrow()!

        // Move money
        let beneficiaryCut <- payment.withdraw(
            amount: price * self.cutPercent
        )
        self.cutReceiver.borrow()!.deposit(from: <- beneficiaryCut)
        receiver.deposit(from: <- payment)

        // Actually create Voucher
        self.totalVoucherSupply = self.totalVoucherSupply + 1
        let id = self.totalVoucherSupply
        emit VoucherBought(voucherId: id)
        self.voucherUsed[id] = false
        return <-create Voucher(id: id, listingId: listingId)
    }

    pub fun createEmptyCollection(): @VoucherCollection {
        return <- create VoucherCollection()
    }

    pub fun createListingCreator(price: UFix64, sellerPaymentReceiver: Capability<&FUSD.Vault{FungibleToken.Receiver}>): @ListingCreator {
        return <- create ListingCreator(price: price, sellerPaymentReceiver: sellerPaymentReceiver)
    }

    init() {
        self.totalVoucherSupply = 0
        self.listings = {}
        self.storedNFTs <- {}
        self.vouchersAvailable = {}
        self.voucherUsed = {}


        let admin <- create Administrator()
        self.account.save(<-admin, to: /storage/cheezeVouchersAdmin)

        self.cutReceiver = self.account.getCapability<&FUSD.Vault{FungibleToken.Receiver}>(
            /public/fusdReceiver
        )!
        self.cutPercent = 0.25
    }
}
