import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import GarmentNFT from 0xfc91de5e6566cc7c
import MaterialNFT from 0xfc91de5e6566cc7c
import ItemNFT from 0xfc91de5e6566cc7c
import FBRC from 0xfc91de5e6566cc7c

//helper contract that combines the garment and material nft to mint item nft AND gives you an amount of fbrc as reward
//only admin can deposit to fbrc vault
pub contract MintTransferClaim{

    //fbrcVault that holds fbrc that will be rewarded to accounts to mint a Item
    access(self) var fbrcVault: @FBRC.Vault

    pub event ItemMintedAndTransferred(garmentID: UInt64, materialID: UInt64, itemID: UInt64)

    // array of itemDataIDs that are already claimed by users
    access(self) var claimedItems: [UInt32]

    // dictionary of addresses that can claim fbrc
    access(self) var addressClaim: {Address: Bool}

    pub let AdminStoragePath: StoragePath

    //amount of fbrc that will be claimable
    pub var claimAmount: UFix64 

    //is reward claimable
    pub var closed: Bool

    pub resource Admin{
        
        //deposit FBRC to contract vault
        pub fun deposit(from: @FungibleToken.Vault) {
            let from <- from as! @FBRC.Vault
            let balance = from.balance
            MintTransferClaim.fbrcVault.deposit(from: <-from)
        }

        //change claim FBRC claim amount from contract vault
        pub fun changeClaimAmount(amount: UFix64) {
            MintTransferClaim.claimAmount = amount
        }

        //prevent claiming
        pub fun closeClaim() {
            MintTransferClaim.closed = true
        }

        //withdraw FBRC from the contract vault
        pub fun withdraw(amount: UFix64, to:  &FBRC.Vault{FungibleToken.Receiver}){
            let withdrawnVault <- MintTransferClaim.fbrcVault.withdraw(amount: amount);
            to.deposit(from: <-withdrawnVault)
        }

        //combine garment and material to mint item, then return the itemd
        //each address can only use this function once
        pub fun mintAndTransferItem(
            name: String, 
            fbrcCap: Capability<&FBRC.Vault{FungibleToken.Receiver}>, 
            garment: @GarmentNFT.NFT, 
            material: @MaterialNFT.NFT): @ItemNFT.NFT {
    
            //check if a garment with garmentdataid X and material with materialdataid Y has already been transferred to another user
            pre {
                !MintTransferClaim.claimedItems.contains(ItemNFT.getItemDataAllocation(garmentDataID: garment.garment.garmentDataID, materialDataID:material.material.materialDataID)):
                    ("garment with garmentDataID and material with materialDataID not avaiable")
                !MintTransferClaim.closed:
                    "Claiming fbrc for minting kitties are closed"
                 MintTransferClaim.addressClaim[fbrcCap.address] == nil:
                    "Address can only mint one item"
            }

            // we check using the itemdataallocation using the garment and material dataid
            let itemDataID = ItemNFT.getItemDataAllocation(garmentDataID: garment.garment.garmentDataID, materialDataID:material.material.materialDataID)
        
            // if combination is not yet claimed, add to claimed list
            MintTransferClaim.claimedItems.append(itemDataID)    

            //check if fbrc vault capability is invalid
            let recipientFBRCVault = fbrcCap.borrow()??
                                    panic("FBRC Vault Capability invalid")
                                    
            let royaltyVaultAddr = fbrcCap.address

            //set the address of transacter as false in addressClaim dict
            if(MintTransferClaim.addressClaim[royaltyVaultAddr] == nil) {
                MintTransferClaim.addressClaim[royaltyVaultAddr] = false
            } 

            let garmentID = garment.id

            let materialID = material.id
            
            //admin mints the item
            let item <- ItemNFT.mintNFT(name: name, royaltyVault: fbrcCap, garment: <- garment, material: <- material)

            emit ItemMintedAndTransferred(garmentID: garmentID, materialID: materialID, itemID: item.id)

            return <- item
        
        }

        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }

    }


    //users can claim FBRC if they have minted an item
    pub fun claimFBRC(fbrcCap: Capability<&FBRC.Vault{FungibleToken.Receiver}>) {     
        
        //Make sure they are in the claim list (not null) and they have not claimed yet (not true)
        pre {
            MintTransferClaim.addressClaim[fbrcCap.address] == false:
                "your address is not eligible to claim fbrc"
        }
        
        //set address in dictionary value to be true thereby not allowing the address to claim anymore
        MintTransferClaim.addressClaim[fbrcCap.address] = true

        //withdraw fbrc from contract vault
        let withdrawnFBRC <- MintTransferClaim.fbrcVault.withdraw(amount: MintTransferClaim.claimAmount)

        let recipientFBRCVault = fbrcCap.borrow()??
                                      panic("FBRC Vault Capability invalid")

        //deposit fbrc to claimer's fbrc vault
        recipientFBRCVault.deposit(from: <- withdrawnFBRC)
    }

    //get balance of contract vault
    pub fun getBalance(): UFix64 {
        let balance = MintTransferClaim.fbrcVault.balance
        return balance
    }

    // get all claimedItems created
    pub fun getClaimedItems(): [UInt32] {
        return MintTransferClaim.claimedItems
    }

    // get all ItemDatas created
    pub fun getAddressClaim(): {Address: Bool} {
        return MintTransferClaim.addressClaim
    }
    
    init() {
        self.fbrcVault <- FBRC.createEmptyVault() as! @FBRC.Vault
        self.claimAmount = 1000.0
        self.claimedItems = []
        self.closed = false
        self.addressClaim = {}
        self.AdminStoragePath = /storage/MintItemAndClaimFBRCAdmin20
        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)
    }
}