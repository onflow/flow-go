import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448
import FlowToken from 0x1654653399040a61
import ArleePartner from 0xd1eaa1286d4c85a2
import ArleeScene from 0xd1eaa1286d4c85a2

pub contract Arlequin {
    
    pub var arleepartnerNFTPrice : UFix64 
    pub var sceneNFTPrice : UFix64

    // This is the ratio to partners in arleepartnerNFT sales, ratio to Arlequin will be (1 - partnerSplitRatio)
    pub var partnerSplitRatio : UFix64

    // Paths
    pub let ArleePartnerAdminStoragePath : StoragePath
    pub let ArleeSceneAdminStoragePath : StoragePath

    // Query Functions
    /* For ArleePartner */
    pub fun checkArleePartnerNFT(addr: Address): Bool {
        return ArleePartner.checkArleePartnerNFT(addr: addr)
    }

    pub fun getArleePartnerNFTIDs(addr: Address) : [UInt64]? {
        return ArleePartner.getArleePartnerNFTIDs(addr: addr)
    }

    pub fun getArleePartnerNFTName(id: UInt64) : String? {
        return ArleePartner.getArleePartnerNFTName(id: id)
    }

    pub fun getArleePartnerNFTNames(addr: Address) : [String]? {
        return ArleePartner.getArleePartnerNFTNames(addr: addr)
    }

    pub fun getArleePartnerAllNFTNames() : {UInt64 : String} {
        return ArleePartner.getAllArleePartnerNFTNames()
    }

    pub fun getArleePartnerRoyalties() : {String : ArleePartner.Royalty} {
        return ArleePartner.getRoyalties()
    }

    pub fun getArleePartnerRoyaltiesByPartner(partner: String) : ArleePartner.Royalty? {
        return ArleePartner.getPartnerRoyalty(partner: partner)
    }

    pub fun getArleePartnerOwner(id: UInt64) : Address? {
        return ArleePartner.getOwner(id: id)
    }

    pub fun getArleePartnerMintable() : {String : Bool} {
        return ArleePartner.getMintable()
    }

    pub fun getArleePartnerTotalSupply() : UInt64 {
        return ArleePartner.totalSupply
    }

    // For Minting 
    pub fun getArleePartnerMintPrice() : UFix64 {
        return Arlequin.arleepartnerNFTPrice
    }

    pub fun getArleePartnerSplitRatio() : UFix64 {
        return Arlequin.partnerSplitRatio
    }



    /* For ArleeScene */
    pub fun getArleeSceneNFTIDs(addr: Address) : [UInt64]? {
        return ArleeScene.getArleeSceneIDs(addr: addr)
    }

    pub fun getArleeSceneRoyalties() : [ArleeScene.Royalty] {
        return ArleeScene.getRoyalty()
    }

    pub fun getArleeSceneCID(id: UInt64) : String? {
        return ArleeScene.getArleeSceneCID(id: id)
    }

    pub fun getAllArleeSceneCID() : {UInt64 : String} {
        return ArleeScene.getAllArleeSceneCID()
    }

    pub fun getArleeSceneFreeMintAcct() : {Address : UInt64} {
        return ArleeScene.getFreeMintAcct()
    }

    pub fun getArleeSceneFreeMintQuota(addr: Address) : UInt64? {
        return ArleeScene.getFreeMintQuota(addr: addr)
    }

    pub fun getArleeSceneOwner(id: UInt64) : Address? {
        return ArleeScene.getOwner(id: id)
    }

    pub fun getArleeSceneMintable() : Bool {
        return ArleeScene.mintable
    }

    pub fun getArleeSceneTotalSupply() : UInt64 {
        return ArleeScene.totalSupply
    }

    // For Minting 
    pub fun getArleeSceneMintPrice() : UFix64 {
        return Arlequin.sceneNFTPrice
    }



    pub resource ArleePartnerAdmin {
        // ArleePartner NFT Admin Functinos
        pub fun addPartner(creditor: String, addr: Address, cut: UFix64 ) {
            ArleePartner.addPartner(creditor: creditor, addr: addr, cut: cut )
        }

        pub fun removePartner(creditor: String) {
            ArleePartner.removePartner(creditor: creditor)
        }

        pub fun setMarketplaceCut(cut: UFix64) {
            ArleePartner.setMarketplaceCut(cut: cut)
        }

        pub fun setPartnerCut(partner: String, cut: UFix64) {
            ArleePartner.setPartnerCut(partner: partner, cut: cut)
        }

        pub fun setMintable(mintable: Bool) {
            ArleePartner.setMintable(mintable: mintable)
        }

        pub fun setSpecificPartnerNFTMintable(partner:String, mintable: Bool) {
            ArleePartner.setSpecificPartnerNFTMintable(partner:partner, mintable: mintable)
        }

        // for Minting
        pub fun setArleePartnerMintPrice(price: UFix64) {
            Arlequin.arleepartnerNFTPrice = price
        }

        pub fun setArleePartnerSplitRatio(ratio: UFix64) {
            pre{
                ratio <= 1.0 : "The spliting ratio cannot be greater than 1.0"
            }
            Arlequin.partnerSplitRatio = ratio
        }

        // Add flexibility to giveaway : an Admin mint function.
        pub fun adminMintArleePartnerNFT(partner: String){
            // get all merchant receiving vault references 
            let recipientCap = getAccount(Arlequin.account.address).getCapability<&ArleePartner.Collection{ArleePartner.CollectionPublic}>(ArleePartner.CollectionPublicPath)
            let recipient = recipientCap.borrow() ?? panic("Cannot borrow Arlequin's Collection Public")

            // deposit
            ArleePartner.adminMintArleePartnerNFT(recipient:recipient, partner: partner)
        }
    }

    pub resource ArleeSceneAdmin {
        // Arlee Scene NFT Admin Functinos
        pub fun setMarketplaceCut(cut: UFix64) {
            ArleeScene.setMarketplaceCut(cut: cut)
        }

        pub fun addFreeMintAcct(addr: Address, mint:UInt64) {
            ArleeScene.addFreeMintAcct(addr: addr, mint:mint)
        }

        pub fun batchAddFreeMintAcct(list:{Address : UInt64}) {
            ArleeScene.batchAddFreeMintAcct(list: list)
        }

        pub fun removeFreeMintAcct(addr: Address) {
            ArleeScene.removeFreeMintAcct(addr: addr)
        }

        // set an acct's free minting limit
        pub fun setFreeMintAcctQuota(addr: Address, mint: UInt64) {
            ArleeScene.setFreeMintAcctQuota(addr: addr, mint: mint)
        }

        // add to an acct's free minting limit
        pub fun addFreeMintAcctQuota(addr: Address, additionalMint: UInt64) {
            ArleeScene.addFreeMintAcctQuota(addr: addr, additionalMint: additionalMint)
        }

        pub fun setMintable(mintable: Bool) {
            ArleeScene.setMintable(mintable: mintable)
        }

        // for minting
        pub fun setArleeSceneMintPrice(price: UFix64) {
            Arlequin.sceneNFTPrice = price
        }

    }

    /* Public Minting for ArleePartnerNFT */
    pub fun mintArleePartnerNFT(buyer: Address, partner: String, paymentVault:  @FungibleToken.Vault) {
        pre{
            paymentVault.balance >= Arlequin.arleepartnerNFTPrice: "Insufficient payment amount."
            paymentVault.getType() == Type<@FlowToken.Vault>(): "payment type not in FlowToken.Vault."
        }

        // get all merchant receiving vault references 
        let arlequinVault = self.account.borrow<&FlowToken.Vault{FungibleToken.Receiver}>(from: /storage/flowTokenVault) ?? panic("Cannot borrow Arlequin's receiving vault reference")

        let partnerRoyalty = self.getArleePartnerRoyaltiesByPartner(partner:partner) ?? panic ("Cannot find partner : ".concat(partner))
        let partnerAddr = partnerRoyalty.wallet
        let partnerVaultCap = getAccount(partnerAddr).getCapability<&FlowToken.Vault{FungibleToken.Receiver}>(/public/flowTokenReceiver)
        let partnerVault = partnerVaultCap.borrow() ?? panic("Cannot borrow partner's receiving vault reference")

        let recipientCap = getAccount(buyer).getCapability<&ArleePartner.Collection{ArleePartner.CollectionPublic}>(ArleePartner.CollectionPublicPath)
        let recipient = recipientCap.borrow() ?? panic("Cannot borrow recipient's Collection Public")

        // splitting vaults for partner and arlequin
        let toPartnerVault <- paymentVault.withdraw(amount: paymentVault.balance * Arlequin.partnerSplitRatio)

        // deposit
        arlequinVault.deposit(from: <- paymentVault)
        partnerVault.deposit(from: <- toPartnerVault)

        ArleePartner.mintArleePartnerNFT(recipient:recipient, partner: partner)
    }

    /* Public Minting for ArleeSceneNFT */
    pub fun mintSceneNFT(buyer: Address, cid: String, description:String, paymentVault:  @FungibleToken.Vault) {
        pre{
            paymentVault.balance >= Arlequin.sceneNFTPrice: "Insufficient payment amount."
            paymentVault.getType() == Type<@FlowToken.Vault>(): "payment type not in FlowToken.Vault."
        }

        // get all merchant receiving vault references 
        let arlequinVault = self.account.borrow<&FlowToken.Vault{FungibleToken.Receiver}>(from: /storage/flowTokenVault) ?? panic("Cannot borrow Arlequin's receiving vault reference")

        let recipientCap = getAccount(buyer).getCapability<&ArleeScene.Collection{ArleeScene.CollectionPublic}>(ArleeScene.CollectionPublicPath)
        let recipient = recipientCap.borrow() ?? panic("Cannot borrow recipient's Collection Public")

        // deposit
        arlequinVault.deposit(from: <- paymentVault)

        ArleeScene.mintSceneNFT(recipient:recipient, cid:cid, description:description)
    }

    /* Free Minting for ArleeSceneNFT */
    pub fun mintSceneFreeMintNFT(buyer: Address, cid: String, description:String) {
        pre{
            Arlequin.getArleeSceneFreeMintQuota(addr: buyer) != nil : "You are not given free mint quotas"
            Arlequin.getArleeSceneFreeMintQuota(addr: buyer)! > 0 : "You ran out of free mint quotas"
        }

        let recipientCap = getAccount(buyer).getCapability<&ArleeScene.Collection{ArleeScene.CollectionPublic}>(ArleeScene.CollectionPublicPath)
        let recipient = recipientCap.borrow() ?? panic("Cannot borrow recipient's Collection Public")

        ArleeScene.freeMintAcct[buyer] = ArleeScene.freeMintAcct[buyer]! - 1

        // deposit
        ArleeScene.mintSceneNFT(recipient:recipient, cid:cid, description:description)
    }

    init(){
        self.arleepartnerNFTPrice = 10.0
        self.sceneNFTPrice = 10.0

        self.partnerSplitRatio = 1.0

        self.ArleePartnerAdminStoragePath = /storage/ArleePartnerAdmin
        self.ArleeSceneAdminStoragePath = /storage/ArleeSceneAdmin              
        
        self.account.save(<- create ArleePartnerAdmin(), to:Arlequin.ArleePartnerAdminStoragePath)
        self.account.save(<- create ArleeSceneAdmin(), to:Arlequin.ArleeSceneAdminStoragePath)
        
    }


}
 