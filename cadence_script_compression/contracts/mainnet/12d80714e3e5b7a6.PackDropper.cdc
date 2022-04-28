/*
    PackSeller.cdc

    Description: Contract for pack buying and adding new packs  

*/
import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393

pub contract PackDropper {  
    pub event ContractInitialized()
    pub event PackAdded(id: UInt32, name: String, size: Int, price: UFix64, availableFrom: UFix64)
    pub event PackBought(id: UInt32, address: Address?, order: Int)
    pub let PacksHandlerStoragePath: StoragePath
    pub let PacksHandlerPublicPath: PublicPath
    pub let PacksInfoPublicPath: PublicPath
    // Variable size dictionary of CombinationData structs
    pub var currentPackId: UInt32

    pub struct PackData {

        pub let id: UInt32
        pub let name: String
        pub let size: Int
        pub let price: UFix64
        pub let availableFromTimeStamp: UFix64
        pub let buyers: [Address]

        init(name: String, size: Int, price: UFix64, availableFrom: UFix64) {
            self.id = PackDropper.currentPackId
            self.name = name
            self.size = size
            self.price = price
            self.availableFromTimeStamp = availableFrom
            self.buyers = []
            // Increment the ID so that it isn't used again
            PackDropper.currentPackId = PackDropper.currentPackId + (1 as UInt32)            
        }
    }

        // PackPurchaser
    // An interface to allow purchasing packs
    pub resource interface PackPurchaser {
        pub fun buyPack(
            packId: UInt32,
            buyerPayment: @FungibleToken.Vault,
            buyerAddress: Address
        )
        
        pub fun getPackPrice(packId:UInt32) : UFix64
        pub fun getPackBuyers(packId:UInt32) : [Address]
    }

    // PacksInfo
    // An interface to allow checking information about packs in the account
    pub resource interface PacksInfo {        
        pub fun getIDs() : [UInt32]
    }

    // PacksHandler
    // Resource that an admin would own to be able to handle packs
	pub resource PacksHandler : PackPurchaser, PacksInfo {

		access(self) var packStats: {UInt32: PackData}

		pub fun addPack(name: String, size: Int, price: UFix64, availableFrom: UFix64) {
        pre {
            name.length > 0 : "Pack must have a name"
            size > 0 : "Size must be positive number"
            price > 0.0 : "Price must be greater than zero"
        }
            let pack = PackData(name: name, size: size, price:price, availableFrom: availableFrom)
            self.packStats[pack.id] = pack
            emit PackAdded(id: pack.id, name: name, size: size, price: price, availableFrom: availableFrom)
		}

        pub fun getIDs(): [UInt32] {
            return self.packStats.keys
        }


        pub fun getPackPrice(packId:UInt32) : UFix64 {
            pre {
                self.packStats[packId] != nil : "Pack must exist"
            }

            return self.packStats[packId]!.price
        }

        pub fun getPackBuyers(packId:UInt32) : [Address] {
            pre {
                self.packStats[packId] != nil : "Pack must exist"
            }

            return self.packStats[packId]!.buyers
        }

        pub fun buyPack(
            packId: UInt32,
            buyerPayment: @FungibleToken.Vault,
            buyerAddress: Address
        ) {
            pre {
                self.packStats[packId] != nil: "Pack does not exist in the collection!"
                buyerPayment.balance == self.packStats[packId]!.price : "payment does not equal the price of the pack"
                getCurrentBlock().timestamp > self.packStats[packId]!.availableFromTimeStamp : "Pack selling not enabled yet!"
                self.packStats[packId]!.buyers.length < (self.packStats[packId]!.size as Int) : "All packs are bought"
                self.owner!.getCapability<&FUSD.Vault{FungibleToken.Receiver}>(/public/fusdReceiver).borrow() != nil : "FUSD receiver cant be nil"  
            }      

            let ownerReceiverVault = self.owner!.getCapability<&FUSD.Vault{FungibleToken.Receiver}>(/public/fusdReceiver).borrow()!

            ownerReceiverVault.deposit(from: <- buyerPayment)
            
            self.packStats[packId]!.buyers.append(buyerAddress)
            emit PackBought(id: packId, address: buyerAddress, order: self.packStats[packId]!.buyers.length)
        }

        init () {
            self.packStats = {}
        }
	}

    init () {
        self.currentPackId = 1
        self.PacksHandlerStoragePath = /storage/packsHandler
        self.PacksHandlerPublicPath = /public/packsHandler
        self.PacksInfoPublicPath = /public/packsInfo

        let packsHandler <- create PacksHandler()
        self.account.save(<-packsHandler, to: self.PacksHandlerStoragePath)
        self.account.link<&PackDropper.PacksHandler{PackDropper.PackPurchaser}>(self.PacksHandlerPublicPath, target: self.PacksHandlerStoragePath)
        self.account.link<&PackDropper.PacksHandler{PackDropper.PacksInfo}>(self.PacksInfoPublicPath, target: self.PacksHandlerStoragePath)

        emit ContractInitialized()
    }
}

