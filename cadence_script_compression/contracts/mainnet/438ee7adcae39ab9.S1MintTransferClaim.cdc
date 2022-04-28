import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import S1GarmentNFT from 0x438ee7adcae39ab9
import S1MaterialNFT from 0x438ee7adcae39ab9
import S1ItemNFT from 0x438ee7adcae39ab9

pub contract S1MintTransferClaim{

    pub event ItemMintedAndTransferred(garmentDataID: UInt32, materialDataID: UInt32, primaryColor: String, secondaryColor: String, itemID: UInt64)

    // dictionary of addresses and how many items they have minted
    access(self) var addressMintCount: {Address: UInt32}

    pub let AdminStoragePath: StoragePath

    //is the contract closed
    pub var closed: Bool

    pub resource Admin{
        
        //prevent minting
        pub fun closeClaim() {
            S1MintTransferClaim.closed = true
        }

        //call S1ItemNFT's mintItem function
        //each address can only mint 2 times
        pub fun mintAndTransferItem(
            name: String, 
            recipientAddr: Address,
            royalty: S1ItemNFT.Royalty,
            garment: @S1GarmentNFT.NFT, 
            material: @S1MaterialNFT.NFT,
            primaryColor: String,
            secondaryColor: String): @S1ItemNFT.NFT {
    
            pre {
                !S1MintTransferClaim.closed:
                    "Minting is closed"
                S1MintTransferClaim.addressMintCount[recipientAddr] != 2:
                    "Address has minted 2 items already"
            }

            let garmentDataID = garment.garment.garmentDataID
            let materialDataID = material.material.materialDataID

            // we check using the itemdataallocation using the garment/material dataid and primary/secondary color
            let itemDataID = S1ItemNFT.getItemDataAllocation(
                garmentDataID: garmentDataID,
                materialDataID: materialDataID,
                primaryColor: primaryColor,
                secondaryColor: secondaryColor)

            //set mint count of transacter as 1 if first time, else 2
            if(S1MintTransferClaim.addressMintCount[recipientAddr] == nil) {
                S1MintTransferClaim.addressMintCount[recipientAddr] = 1
            } else {
                S1MintTransferClaim.addressMintCount[recipientAddr] = 2
            }

            let garmentID = garment.id

            let materialID = material.id
            
            //admin mints the item
            let item <- S1ItemNFT.mintNFT(
                name: name, 
                royaltyVault: royalty, 
                garment: <- garment, 
                material: <- material,
                primaryColor: primaryColor,
                secondaryColor: secondaryColor)

            emit ItemMintedAndTransferred(garmentDataID: garmentDataID, materialDataID: materialDataID, primaryColor: primaryColor, secondaryColor: secondaryColor, itemID: item.id)

            return <- item
        }

        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

    }

    // get each address' current mint count
    pub fun getAddressMintCount(): {Address: UInt32} { 
        return S1MintTransferClaim.addressMintCount
    }
    
    init() {
        self.closed = false
        self.addressMintCount = {}
        self.AdminStoragePath = /storage/S1MintTransferClaimAdmin0015
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)
    }
}