import MotoGPAdmin from 0xa49cc0ee46c54bfb
import MotoGPPack from 0xa49cc0ee46c54bfb
import MotoGPCard from 0xa49cc0ee46c54bfb
import NonFungibleToken from 0x1d7e57aa55817448
import ContractVersion from 0xa49cc0ee46c54bfb

// Contract for managing pack opening.
//
// A PackOpener Collection is created when a user authorizes a tx that saves a PackOpener Collection in the user's storage.
// The user authorises transfer of a pack to the pack opener.
// The admin accesses the pack opener to open the pack, which deposits the cards into the user's
// card collection and destroys the pack.
//
pub contract PackOpener: ContractVersion {

    pub fun getVersion(): String {
        return "0.7.9"
    }

    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event PackOpened(packId: UInt64, packType: UInt64, cardIDs:[UInt64], serials: [UInt64])

    pub resource interface IPackOpenerPublic {
        pub fun openPack(adminRef: &MotoGPAdmin.Admin, id: UInt64, cardIDs: [UInt64], serials: [UInt64])
        pub fun deposit(token: @MotoGPPack.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowPack(id: UInt64): &MotoGPPack.NFT?
    }

    pub let packOpenerStoragePath: StoragePath
    pub let packOpenerPublicPath: PublicPath

    pub resource Collection: IPackOpenerPublic {

        access(self) let packMap: @{UInt64: MotoGPPack.NFT}
        access(self) let cardCollectionCap: Capability<&MotoGPCard.Collection{MotoGPCard.ICardCollectionPublic}>

        pub fun getIDs(): [UInt64] {
            return self.packMap.keys
        }

        pub fun deposit(token: @MotoGPPack.NFT) {
            let id: UInt64 = token.id
            let oldToken <- self.packMap[id] <- token
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }
            destroy oldToken
        }

        // The withdraw method is not part of IPackOpenerPublic interface, and can only be accessed by the PackOpener collection owner
        //
        pub fun withdraw(withdrawID: UInt64): @MotoGPPack.NFT {
            let token <- self.packMap.remove(key: withdrawID) ?? panic("MotoGPPack not found and can't be removed")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        // The openPack method requires a admin reference as argument to open the pack
        //
        pub fun openPack(adminRef: &MotoGPAdmin.Admin, id: UInt64, cardIDs: [UInt64], serials: [UInt64]) {
            pre {
                adminRef != nil : "adminRef is nil"
                cardIDs.length == serials.length : "cardIDs and serials are not same length"
                UInt64(cardIDs.length) == UInt64(3) : "cardsIDs.length is not 3"
            }

            let cardCollectionRef = self.cardCollectionCap.borrow()!
            let pack <- self.withdraw(withdrawID: id)
            var tempCardCollection <- MotoGPCard.createEmptyCollection()
            let numberOfCards: UInt64 = UInt64(cardIDs.length)

            var i: UInt64 = 0
            while i < numberOfCards {
                let tempCardID = cardIDs[i]
                let tempSerial = serials[i]
                let newCard <- MotoGPCard.createNFT(cardID: tempCardID, serial: tempSerial)
                tempCardCollection.deposit(token: <-newCard)
                i = i + (1 as UInt64)
            }
            cardCollectionRef.depositBatch(cardCollection: <-tempCardCollection)

            emit PackOpened(packId: id, packType: pack.packInfo.packType, cardIDs: cardIDs, serials: serials)

            destroy pack
        }

        pub fun borrowPack(id: UInt64): &MotoGPPack.NFT? {
            let packRef = &self.packMap[id] as &MotoGPPack.NFT
            return packRef
        }

        init(_cardCollectionCap: Capability<&MotoGPCard.Collection{MotoGPCard.ICardCollectionPublic}>){
            self.packMap <- {}
            self.cardCollectionCap = _cardCollectionCap
        }

        destroy(){
            destroy self.packMap
        }
    }

    pub fun createEmptyCollection(cardCollectionCap: Capability<&MotoGPCard.Collection{MotoGPCard.ICardCollectionPublic}>): @Collection {
        return <- create Collection(_cardCollectionCap: cardCollectionCap)
    }

    init(){
        self.packOpenerStoragePath = /storage/motogpPackOpenerCollection
        self.packOpenerPublicPath = /public/motogpPackOpenerCollection
    }

}