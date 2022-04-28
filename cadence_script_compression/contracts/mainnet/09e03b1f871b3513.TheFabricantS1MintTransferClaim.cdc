/*
    Description: TheFabricantS1MintTransferClaim Contract
   
    This contract prevents users from minting TheFabricantS1ItemNFTs 
    more than a selected amount of times
*/

import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import TheFabricantS1GarmentNFT from 0x9e03b1f871b3513
import TheFabricantS1MaterialNFT from 0x9e03b1f871b3513
import TheFabricantS1ItemNFT from 0x9e03b1f871b3513

pub contract TheFabricantS1MintTransferClaim{

    pub event ItemMintedAndTransferred(garmentDataID: UInt32, materialDataID: UInt32, primaryColor: String, secondaryColor: String, itemID: UInt64, itemDataID: UInt32, name: String)

    // dictionary of addresses and how many items they have minted
    access(self) var addressMintCount: {Address: UInt32}

    pub let AdminStoragePath: StoragePath

    pub var maxMintAmount: UInt32
    //is the contract closed
    pub var closed: Bool

    pub resource Admin{
        
        //prevent minting
        pub fun closeClaim() {
            TheFabricantS1MintTransferClaim.closed = true
        }

        //call S1ItemNFT's mintItem function
        //each address can only mint 2 times
        pub fun mintAndTransferItem(
            name: String, 
            recipientAddr: Address,
            royalty: TheFabricantS1ItemNFT.Royalty,
            garment: @TheFabricantS1GarmentNFT.NFT, 
            material: @TheFabricantS1MaterialNFT.NFT,
            primaryColor: String,
            secondaryColor: String): @TheFabricantS1ItemNFT.NFT {
    
            pre {
                !TheFabricantS1MintTransferClaim.closed:
                    "Minting is closed"
                TheFabricantS1MintTransferClaim.addressMintCount[recipientAddr] != TheFabricantS1MintTransferClaim.maxMintAmount:
                    "Address has minted max amount of items already"
            }

            let garmentDataID = garment.garment.garmentDataID
            let materialDataID = material.material.materialDataID

            // we check using the itemdataallocation using the garment/material dataid and primary/secondary color
            let itemDataID = TheFabricantS1ItemNFT.getItemDataAllocation(
                garmentDataID: garmentDataID,
                materialDataID: materialDataID,
                primaryColor: primaryColor,
                secondaryColor: secondaryColor)

            //set mint count of transacter as 1 if first time, else 2
            if(TheFabricantS1MintTransferClaim.addressMintCount[recipientAddr] == nil) {
                TheFabricantS1MintTransferClaim.addressMintCount[recipientAddr] = 1
            } else {
                TheFabricantS1MintTransferClaim.addressMintCount[recipientAddr] = TheFabricantS1MintTransferClaim.addressMintCount[recipientAddr]! + 1
            }

            let garmentID = garment.id

            let materialID = material.id
            
            //admin mints the item
            let item <- TheFabricantS1ItemNFT.mintNFT(
                name: name, 
                royaltyVault: royalty, 
                garment: <- garment, 
                material: <- material,
                primaryColor: primaryColor,
                secondaryColor: secondaryColor)

            emit ItemMintedAndTransferred(garmentDataID: garmentDataID, materialDataID: materialDataID, primaryColor: primaryColor, secondaryColor: secondaryColor, itemID: item.id, itemDataID: item.item.itemDataID, name: name)

            return <- item
        }

        pub fun changeMaxMintAmount(newMax: UInt32) {
            TheFabricantS1MintTransferClaim.maxMintAmount = newMax
        }
        
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

    }

    pub fun getAddressMintCount(): {Address: UInt32} {
        return TheFabricantS1MintTransferClaim.addressMintCount
    }
    
    init() {
        self.closed = false
        self.maxMintAmount = 2
        self.addressMintCount = {}
        self.AdminStoragePath = /storage/TheFabricantS1MintTransferClaimAdmin0019
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)
    }
}