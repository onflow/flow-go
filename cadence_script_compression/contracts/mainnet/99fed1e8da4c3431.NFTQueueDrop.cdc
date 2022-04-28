import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe

pub contract NFTQueueDrop {

    pub event Claimed(nftType: Type, nftID: UInt64)

    pub let DropStoragePath: StoragePath
    pub let DropPublicPath: PublicPath

    pub enum DropStatus: UInt8 {
        pub case open
        pub case paused
        pub case closed
    }

    pub resource interface DropPublic {
        pub let price: UFix64
        pub let size: Int
        pub var status: DropStatus
        pub fun supply(): Int

        pub fun claim(payment: @FungibleToken.Vault): @NonFungibleToken.NFT
    }

    pub resource Drop: DropPublic {

        access(self) let nftType: Type
        access(self) let collection: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>
        access(self) let paymentReceiver: Capability<&{FungibleToken.Receiver}>

        pub let price: UFix64
        pub let size: Int
        pub var status: DropStatus

        pub fun pause() {
            self.status = DropStatus.paused
        }

        pub fun resume() {
            pre {
                self.status != DropStatus.closed : "Cannot resume drop that is closed"
            }

            self.status = DropStatus.open
        }

        pub fun close() {
            self.status = DropStatus.closed
        }

        pub fun supply(): Int {
            return self.collection.borrow()!.getIDs().length
        }

        pub fun complete(): Bool {
            return self.supply() == 0
        }

        access(self) fun pop(): @NonFungibleToken.NFT {
            let collection = self.collection.borrow()!
            let ids = collection.getIDs()
            let nextID = ids[0]

            return <- collection.withdraw(withdrawID: nextID)
        }

        pub fun claim(payment: @FungibleToken.Vault): @NonFungibleToken.NFT {
            pre {
                payment.balance == self.price: "payment vault does not contain requested price"
            }

            let collection = self.collection.borrow()!
            let receiver = self.paymentReceiver.borrow()!

            receiver.deposit(from: <- payment)

            let nft <- self.pop()

            if self.supply() == 0 {
                self.close()
            }

            emit Claimed(nftType: self.nftType, nftID: nft.id)

            return <- nft
        }

        init(
            nftType: Type,
            collection: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>, 
            paymentReceiver: Capability<&{FungibleToken.Receiver}>,
            paymentPrice: UFix64
        ) {
            self.nftType = nftType
            self.collection = collection
            self.paymentReceiver = paymentReceiver

            self.price = paymentPrice
            self.size = collection.borrow()!.getIDs().length
            self.status = DropStatus.open
        }
    }

    pub fun createDrop(
        nftType: Type,
        collection: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>, 
        paymentReceiver: Capability<&{FungibleToken.Receiver}>,
        paymentPrice: UFix64
    ): @Drop {
        return <- create Drop(
            nftType: nftType,
            collection: collection,
            paymentReceiver: paymentReceiver,
            paymentPrice: paymentPrice
        )
    }

    init() {
        self.DropStoragePath = /storage/NFTQueueDrop
        self.DropPublicPath = /public/NFTQueueDrop
    }
}
