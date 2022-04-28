/*
    Description: FlowFestAccess Contract
   
    This contract allows users to redeem their FlowFest NFTs to gain access to thefabricant.studio
*/

import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import TheFabricantMysteryBox_FF1 from 0xa0cbe021821c0965
pub contract FlowFestAccess {

    // -----------------------------------------------------------------------
    // FlowFestAccess contract Events
    // -----------------------------------------------------------------------
    pub event AccountVerified(address: Address, id: UInt64)
    
    access(self) var accountsVerified: {Address: UInt64}

    //redeem a flowfest nft by adding it into accountsVerified mapping if not already
    pub fun giveAccess(id: UInt64, collectionCap: Capability<&{TheFabricantMysteryBox_FF1.FabricantCollectionPublic}>){
        pre {
            FlowFestAccess.isAccountVerified(address: collectionCap.address) == false:
            "account already has access"
            !FlowFestAccess.accountsVerified.values.contains(id):
            "id is already used"
        }
        let collection = collectionCap.borrow()!
        // get the ids of the flowfest nfts that the collection contains
        let ids = collection.getIDs()
        // check that collection actually contains nft with that id
        if ids.contains(id) {
            //verify account and store in dictionary
            FlowFestAccess.accountsVerified[collectionCap.address] = id
            emit AccountVerified(address: collectionCap.address, id: id)
        } else {
            panic("user does not have nft with this id")
        }

    }

    // get dictionary of accounts that are verified and the id
    // of the flowfest nft that it used
    pub fun getAccountsVerified(): {Address: UInt64} {
        return FlowFestAccess.accountsVerified
    }

    // check if account is already verified
    pub fun isAccountVerified(address: Address): Bool {
        for key in FlowFestAccess.accountsVerified.keys {
            if key == address {
                return true
            }
        }
        return false
    }
    
    // -----------------------------------------------------------------------
    // initialization function
    // -----------------------------------------------------------------------
    //
    init() {
        self.accountsVerified = {}
    }
}