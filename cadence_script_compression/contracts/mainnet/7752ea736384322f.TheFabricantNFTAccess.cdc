import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
pub contract TheFabricantNFTAccess {

    // -----------------------------------------------------------------------
    // TheFabricantNFTAccess contract Events
    // -----------------------------------------------------------------------
    pub let AdminStoragePath: StoragePath

    pub let RedeemerStoragePath: StoragePath

    pub event EventAdded(eventName: String, types: [Type])

    pub event EventRedemption(eventName: String, address: Address, nftID: UInt64, nftType: Type, nftUuid: UInt64)

    pub event AccessListChanged(eventName: String, addresses: [Address])

    // eventName: {redeemerAddress: nftUuid}
    access(self) var event: {String: {Address: UInt64}} 

    // eventName: [nftTypes]
    access(self) var eventToTypes: {String: [Type]}

    // eventName: [addresses]
    access(self) var accessList: {String: [Address]}


    pub resource Admin{

        //add event to event dictionary
        pub fun addEvent(eventName: String, types: [Type]){
            pre {
                TheFabricantNFTAccess.event[eventName] == nil:
                "eventName already exists"
            }
            TheFabricantNFTAccess.event[eventName] = {}
            TheFabricantNFTAccess.eventToTypes[eventName] = types

            emit EventAdded(eventName: eventName, types: types)
        }

        pub fun changeAccessList(eventName: String, addresses: [Address]){
            TheFabricantNFTAccess.accessList[eventName] = addresses

            emit AccessListChanged(eventName: eventName, addresses: addresses)
        }

        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

    }

    pub resource Redeemer {

        // user redeems an nft for an event
        pub fun redeem(eventName: String, nftRef: &NonFungibleToken.NFT) {
            pre {
                nftRef.owner!.address == self.owner!.address:
                "redeemer is not owner of nft"
                TheFabricantNFTAccess.event[eventName] != nil:
                "event does exist"
                !TheFabricantNFTAccess.event[eventName]!.keys.contains(self.owner!.address):
                "address already redeemed for this event"
                !TheFabricantNFTAccess.event[eventName]!.values.contains(nftRef.uuid):
                "nft is already used for redemption for this event"
            }

            let array = TheFabricantNFTAccess.getEventToTypes()[eventName]!

            if (array.contains(nftRef.getType())) {
                let oldAddressToUUID = TheFabricantNFTAccess.event[eventName]!
                oldAddressToUUID[self.owner!.address] = nftRef.uuid
                TheFabricantNFTAccess.event[eventName] = oldAddressToUUID
                emit EventRedemption(
                    eventName: eventName,
                    address: self.owner!.address, 
                    nftID: nftRef.id, 
                    nftType: nftRef.getType(), 
                    nftUuid: nftRef.uuid
                )
                return
            } else {
                panic ("the nft you have provided is not a redeemable type for this event")
            }
        }

        // destructor
        //
        destroy () {}

        // initializer
        //
        init () {}
    }

    pub fun createNewRedeemer(): @Redeemer {
        return <-create Redeemer()
    }

    pub fun getEvent(): {String: {Address: UInt64}} {
        return TheFabricantNFTAccess.event
    }

    pub fun getEventToTypes(): {String: [Type]} {
        return TheFabricantNFTAccess.eventToTypes
    }

    pub fun getAccessList(): {String: [Address]} {
        return TheFabricantNFTAccess.accessList
    }

    // -----------------------------------------------------------------------
    // initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        self.event = {}
        self.eventToTypes = {}
        self.accessList = {}
        self.AdminStoragePath = /storage/NFTAccessAdmin0022
        self.RedeemerStoragePath = /storage/NFTAccessRedeemer0022
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)
    }
}