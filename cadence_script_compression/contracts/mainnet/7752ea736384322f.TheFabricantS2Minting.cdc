/*
    Description: TheFabricantS2Minting Contract
   
    This contract lets users mint TheFabricantS2ItemNFT NFTs for a specified amount of FLOW
*/

import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import TheFabricantS2GarmentNFT from 0x7752ea736384322f
import TheFabricantS2MaterialNFT from 0x7752ea736384322f
import TheFabricantS2ItemNFT from 0x7752ea736384322f
import FlowToken from 0x1654653399040a61
import TheFabricantNFTAccess from 0x7752ea736384322f
import TheFabricantMysteryBox_FF1 from 0xa0cbe021821c0965
import ItemNFT from 0xfc91de5e6566cc7c
import TheFabricantS1ItemNFT from 0x9e03b1f871b3513

pub contract TheFabricantS2Minting{

    pub event ItemMintedAndTransferred(recipientAddr: Address, garmentDataID: UInt32, materialDataID: UInt32, primaryColor: String, secondaryColor: String, itemID: UInt64, itemDataID: UInt32, name: String, eventName: String, variant: String)

    pub event EventAdded(eventName: String, eventDetail: EventDetail)

    pub event IsEventClosedChanged(eventName: String, isClosed: Bool)
    pub event MaxMintAmountChanged(eventName: String, newMax: UInt32)
    pub event PaymentTypeChanged(eventName: String, newPaymentType: Type)
    pub event PaymentAmountChanged(eventName: String, newPaymentAmount: UFix64)

    pub event ItemMinterCapabilityChanged(address: Address)
    pub event GarmentMinterCapabilityChanged(address: Address)
    pub event MaterialMinterCapabilityChanged(address: Address)
    pub event PaymentReceiverCapabilityChanged(address: Address, paymentType: Type)

    pub let AdminStoragePath: StoragePath
    pub let MinterStoragePath: StoragePath

    access(self) var eventsDetail: {String: EventDetail}

    access(contract) var itemMinterCapability: Capability<&TheFabricantS2ItemNFT.Admin>?
    access(contract) var garmentMinterCapability: Capability<&TheFabricantS2GarmentNFT.Admin>?
    access(contract) var materialMinterCapability: Capability<&TheFabricantS2MaterialNFT.Admin>?

    access(contract) var paymentReceiverCapability: Capability<&{FungibleToken.Receiver}>?


    pub struct EventDetail {

        access(self) var addressMintCount: {Address: UInt32}

        pub var closed: Bool

        pub var paymentAmount: UFix64

        pub var paymentType: Type

        pub var maxMintAmount: UInt32

        init(paymentAmount: UFix64, paymentType: Type, maxMintAmount: UInt32) {
            self.addressMintCount = {}
            self.closed = true
            self.paymentAmount = paymentAmount
            self.paymentType = paymentType
            self.maxMintAmount = maxMintAmount
        }

        pub fun changeIsEventClosed(isClosed: Bool) {
            self.closed = isClosed
        }

        pub fun changeMaxMintAmount(newMax: UInt32) {
            self.maxMintAmount = newMax
        }

        pub fun changePaymentType(newPaymentType: Type) {
            self.paymentType = newPaymentType
        }

        pub fun changePaymentAmount(newPaymentAmount: UFix64) {
            self.paymentAmount = newPaymentAmount
        }

        access(contract) fun incrementAddressMintCount(address: Address) {
            if(self.addressMintCount[address] == nil) {
                self.addressMintCount[address] = 1
            } else {
                self.addressMintCount[address] = self.addressMintCount[address]! + 1
            }
        }

        pub fun getAddressMintCount(): {Address: UInt32} {
            return self.addressMintCount
        }
    }

    // check if an address holds certain nfts to allow mint
    pub fun doesAddressHoldNFTs(address: Address): Bool {
        var hasTheFabricantMysteryBox_FF1: Bool = false
        var hasItemNFT: Bool = false
        var hasTheFabricantS1ItemNFT: Bool = false
        if (getAccount(address).getCapability<&{TheFabricantMysteryBox_FF1.FabricantCollectionPublic}>(TheFabricantMysteryBox_FF1.CollectionPublicPath).check()) {
            let collectionRef = getAccount(address).getCapability(TheFabricantMysteryBox_FF1.CollectionPublicPath)
                                .borrow<&{TheFabricantMysteryBox_FF1.FabricantCollectionPublic}>()!
            hasTheFabricantMysteryBox_FF1 = collectionRef.getIDs().length > 0
        }

        if (getAccount(address).getCapability<&{ItemNFT.ItemCollectionPublic}>(ItemNFT.CollectionPublicPath).check()) {
            let collectionRef = getAccount(address).getCapability(ItemNFT.CollectionPublicPath)
                                .borrow<&{ItemNFT.ItemCollectionPublic}>()!
            hasItemNFT = collectionRef.getIDs().length > 0
        }

        if (getAccount(address).getCapability<&{TheFabricantS1ItemNFT.ItemCollectionPublic}>(TheFabricantS1ItemNFT.CollectionPublicPath).check()) {
            let collectionRef = getAccount(address).getCapability(TheFabricantS1ItemNFT.CollectionPublicPath)
                                .borrow<&{TheFabricantS1ItemNFT.ItemCollectionPublic}>()!
            hasTheFabricantS1ItemNFT = collectionRef.getIDs().length > 0
        }
        return hasTheFabricantMysteryBox_FF1 || hasItemNFT || hasTheFabricantS1ItemNFT
    }

    pub resource Minter{
        
        //call S2ItemNFT's mintItem function
        //each address can only mint 5 times
        pub fun mintAndTransferItem(
            garmentDataID: UInt32,
            materialDataID: UInt32,
            primaryColor: String,
            secondaryColor: String,
            payment: @FungibleToken.Vault,
            eventName: String): @TheFabricantS2ItemNFT.NFT {
    
            pre {
                TheFabricantS2Minting.eventsDetail[eventName] != nil:
                "event does not exist"
                TheFabricantS2Minting.eventsDetail[eventName]!.closed == false:
                "minting is closed"
                payment.isInstance(TheFabricantS2Minting.eventsDetail[eventName]!.paymentType): 
                "payment vault is not requested fungible token"
                payment.balance == TheFabricantS2Minting.eventsDetail[eventName]!.paymentAmount: 
                "payment vault does not contain requested price" 
                TheFabricantNFTAccess.getAccessList()[eventName]!.contains(self.owner!.address) || TheFabricantS2Minting.doesAddressHoldNFTs(address: self.owner!.address):
                "address is not in access list or does not hold a s0item, s1item or flowfest nft"
            }
        
            if(TheFabricantS2Minting.eventsDetail[eventName]!.getAddressMintCount()[self.owner!.address] != nil) {
                if(TheFabricantS2Minting.eventsDetail[eventName]!.getAddressMintCount()[self.owner!.address]! >= TheFabricantS2Minting.eventsDetail[eventName]!.maxMintAmount) {
                    panic("Address has minted max amount of items already")
                }
            }

            // mint the garment and material
            let garment <- TheFabricantS2Minting.garmentMinterCapability!.borrow()!.mintNFT(garmentDataID: garmentDataID)
            let material <- TheFabricantS2Minting.materialMinterCapability!.borrow()!.mintNFT(materialDataID: materialDataID)  

            // split the royalty from price to garment and material address
            let garmentData = garment.garment.garmentDataID
            let garmentRoyalties = TheFabricantS2GarmentNFT.getGarmentData(id: garmentData).getRoyalty()
            let materialData = material.material.materialDataID
            let materialRoyalties = TheFabricantS2MaterialNFT.getMaterialData(id: materialData).getRoyalty()

            let garmentRoyaltyCount = UFix64(garmentRoyalties.keys.length)
            let materialRoyaltyCount = UFix64(materialRoyalties.keys.length)
            let paymentAmount = payment.balance

            for key in garmentRoyalties.keys {
                let paymentSplit = (paymentAmount*0.45)/garmentRoyaltyCount
                if let garmentRoyaltyReceiver = garmentRoyalties[key]!.wallet.borrow() {
                   let garmentRoyaltyPaymentCut <- payment.withdraw(amount: paymentSplit)
                   garmentRoyaltyReceiver.deposit(from: <- garmentRoyaltyPaymentCut)
                }
            }

            for key in materialRoyalties.keys {
                let paymentSplit = (paymentAmount*0.45)/materialRoyaltyCount
                if let materialRoyaltyReceiver = materialRoyalties[key]!.wallet.borrow() {
                   let materialRoyaltyPaymentCut <- payment.withdraw(amount: paymentSplit)
                   materialRoyaltyReceiver.deposit(from: <- materialRoyaltyPaymentCut)
                }
            }

            // create the metadata for the item
            let metadatas: {String: TheFabricantS2ItemNFT.Metadata} = {}
            metadatas["itemImage"] = 
            TheFabricantS2ItemNFT.Metadata(
                metadataValue: "https://leela.mypinata.cloud/ipfs/Qmf8VBBYSQBWBzqUvCu8Ko5qgxCf1RrwKyrXCBGJ9dZ24B/LOOP.png",
                mutable: true)
            metadatas["itemVideo"] =
            TheFabricantS2ItemNFT.Metadata(
                metadataValue: "https://leela.mypinata.cloud/ipfs/Qmf8VBBYSQBWBzqUvCu8Ko5qgxCf1RrwKyrXCBGJ9dZ24B/LOOP.mp4",
                mutable: true)
            metadatas["itemImage2"] =     
            TheFabricantS2ItemNFT.Metadata(
                metadataValue: "",
                mutable: true)
            metadatas["itemImage3"] =     
            TheFabricantS2ItemNFT.Metadata(
                metadataValue: "",
                mutable: true)
            metadatas["itemImage4"] =     
            TheFabricantS2ItemNFT.Metadata(
                metadataValue: "",
                mutable: true)
            metadatas["season"] =     
            TheFabricantS2ItemNFT.Metadata(
                metadataValue: "2",
                mutable: false)
            metadatas["variant"] =     
            TheFabricantS2ItemNFT.Metadata(
                metadataValue: "",
                mutable: false)
            metadatas["eventName"] =     
            TheFabricantS2ItemNFT.Metadata(
                metadataValue: eventName,
                mutable: false)

            // create the item data with allocation for the item
            TheFabricantS2Minting.itemMinterCapability!.borrow()!.createItemDataWithAllocation(
                garmentDataID: garment.garment.garmentDataID, 
                materialDataID: material.material.materialDataID, 
                primaryColor: primaryColor, 
                secondaryColor: secondaryColor,
                metadatas: metadatas,
                coCreator: self.owner!.address)

            // create the royalty struct for the item
            let royalty = TheFabricantS2ItemNFT.Royalty(
                    wallet: getAccount(self.owner!.address).getCapability<&FlowToken.Vault{FungibleToken.Receiver}>(/public/flowTokenReceiver),
                    initialCut: 0.3,
                    cut: 0.1/3.0
                )


            //set mint count of transacter as 1 if first time, else increment
            let eventDetails = TheFabricantS2Minting.eventsDetail[eventName]!
            eventDetails.incrementAddressMintCount(address: self.owner!.address)
            
            //update event detail for eventName with new detail
            TheFabricantS2Minting.eventsDetail[eventName] = eventDetails

            //set initial name of item to "S2 Zodiac Collection #:id"
            let name = "Season 2 Zodiac Collection #".concat((TheFabricantS2ItemNFT.totalSupply + 1).toString())

            //user mints the item
            let item <- TheFabricantS2ItemNFT.mintNFT(
                name: name,
                royaltyVault: royalty, 
                garment: <- garment, 
                material: <- material,
                primaryColor: primaryColor,
                secondaryColor: secondaryColor)

            emit ItemMintedAndTransferred(
                recipientAddr: self.owner!.address, 
                garmentDataID: garmentDataID, 
                materialDataID: materialDataID, 
                primaryColor: primaryColor, 
                secondaryColor: secondaryColor, 
                itemID: item.id, 
                itemDataID: item.item.itemDataID, 
                name: name, 
                eventName: eventName,
                variant: "")

            //The Fabricant receives the remainder of the payment ofter royalty split
            TheFabricantS2Minting.paymentReceiverCapability!.borrow()!.deposit(from: <-payment)

            return <- item
        }
    }

    pub resource Admin{

        pub fun changeIsEventClosed(eventName: String, isClosed: Bool) {
            pre {
                TheFabricantS2Minting.eventsDetail[eventName] != nil: 
                "eventName doesnt exist"
            }
            let eventDetail = TheFabricantS2Minting.eventsDetail[eventName]!
            eventDetail.changeIsEventClosed(isClosed: isClosed)
            TheFabricantS2Minting.eventsDetail[eventName] = eventDetail

            emit IsEventClosedChanged(eventName: eventName, isClosed: isClosed)
        }

        pub fun changeMaxMintAmount(eventName: String, newMax: UInt32) {
            pre {
                TheFabricantS2Minting.eventsDetail[eventName] != nil: 
                "eventName doesnt exist"
            }
            let eventDetail = TheFabricantS2Minting.eventsDetail[eventName]!
            eventDetail.changeMaxMintAmount(newMax: newMax)
            TheFabricantS2Minting.eventsDetail[eventName] = eventDetail

            emit MaxMintAmountChanged(eventName: eventName, newMax: newMax)
        }

        pub fun changePaymentType(eventName: String, newPaymentType: Type) {
            pre {
                TheFabricantS2Minting.eventsDetail[eventName] != nil: 
                "eventName doesnt exist"
            }
            let eventDetail = TheFabricantS2Minting.eventsDetail[eventName]!
            eventDetail.changePaymentType(newPaymentType: newPaymentType)
            TheFabricantS2Minting.eventsDetail[eventName] = eventDetail

            emit PaymentTypeChanged(eventName: eventName, newPaymentType: newPaymentType)
        }

        pub fun changePaymentAmount(eventName: String, newPaymentAmount: UFix64) {
            pre {
                TheFabricantS2Minting.eventsDetail[eventName] != nil: 
                "eventName doesnt exist"
            }
            let eventDetail = TheFabricantS2Minting.eventsDetail[eventName]!
            eventDetail.changePaymentAmount(newPaymentAmount: newPaymentAmount)
            TheFabricantS2Minting.eventsDetail[eventName] = eventDetail

            emit PaymentAmountChanged(eventName: eventName, newPaymentAmount: newPaymentAmount)
        }

        pub fun changeItemMinterCapability(minterCapability: Capability<&TheFabricantS2ItemNFT.Admin>) {
            TheFabricantS2Minting.itemMinterCapability = minterCapability

            emit ItemMinterCapabilityChanged(address: minterCapability.address)
        }

        pub fun changeGarmentMinterCapability(minterCapability: Capability<&TheFabricantS2GarmentNFT.Admin>) {
            TheFabricantS2Minting.garmentMinterCapability = minterCapability

            emit GarmentMinterCapabilityChanged(address: minterCapability.address)
        }
        
        pub fun changeMaterialMinterCapability(minterCapability: Capability<&TheFabricantS2MaterialNFT.Admin>) {
            TheFabricantS2Minting.materialMinterCapability = minterCapability

            emit MaterialMinterCapabilityChanged(address: minterCapability.address)
        }

        pub fun changePaymentReceiverCapability(paymentReceiverCapability: Capability<&{FungibleToken.Receiver}>) {
            TheFabricantS2Minting.paymentReceiverCapability = paymentReceiverCapability

            emit PaymentReceiverCapabilityChanged(address: paymentReceiverCapability.address, paymentType: paymentReceiverCapability.getType())
        }

        pub fun addEvent(eventName: String, eventDetail: EventDetail){
            pre {
                TheFabricantS2Minting.eventsDetail[eventName] == nil:
                "eventName already exists"
            }
            TheFabricantS2Minting.eventsDetail[eventName] = eventDetail

            emit EventAdded(eventName: eventName, eventDetail: eventDetail)
        }
        
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
    }

    pub fun createNewMinter(): @Minter {
        return <-create Minter()
    }

    pub fun getEventsDetail(): {String: EventDetail} {
        return TheFabricantS2Minting.eventsDetail
    }

    pub fun getPaymentReceiverAddress(): Address {
        return TheFabricantS2Minting.paymentReceiverCapability!.address
    }

    pub fun getMinterCapabilityAddress(): Address {
        return TheFabricantS2Minting.itemMinterCapability!.address
    }
    
    init() {
        self.paymentReceiverCapability = nil
        self.eventsDetail = {}
        self.itemMinterCapability = nil
        self.garmentMinterCapability = nil
        self.materialMinterCapability = nil
        self.AdminStoragePath = /storage/TheFabricantS2MintingAdmin0022
        self.MinterStoragePath = /storage/TheFabricantS2Minter0022
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)
    }
}
 