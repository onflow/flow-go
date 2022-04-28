// SPDX-License-Identifier: Unlicense

import StarvingChildren from 0x3f431c443183c8b
import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393

pub contract StarvingChildrenMarket {
    // -----------------------------------------------------------------------
    // StarvingChildrenMarket contract-level fields.
    // These contain actual values that are stored in the smart contract.
    // -----------------------------------------------------------------------
    
    /// Path where the `SaleCollection` is stored
    pub let marketStoragePath: StoragePath

    /// Path where the public capability for the `SaleCollection` is
    pub let marketPublicPath: PublicPath

    /// Event used on Purchase method
    pub event StarvingChildrenPurchased(idBuyer: UInt64, idCharity: UInt64, templateId: UInt64, buyerAddress: Address, charityAddress: Address)
    
    // ManagerPublic 
    //
    // The interface that a user can publish a capability to their sale
    // to allow others to access their sale
    pub resource interface ManagerPublic {
       pub fun purchase(templateId: UInt64, buyTokens: &FungibleToken.Vault, buyerAddress: Address):  @StarvingChildren.NFT
    }

    // Manager
    // Is a resource used to do purchase action
    //
    pub resource Manager: ManagerPublic {

        /// A admin capability of admin resource used to mintNFT
        access(self) var adminCapability: Capability<&StarvingChildren.Admin>

        /// A receiver capability of fungible token resource used to receive FUSD
        access(self) var receiverCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>
        
        /// A buyer's collection to get purchased NFT 
        access(self) var collectionCapability: Capability<&{StarvingChildren.StarvingChildrenCollectionPublic}>

        init (adminCapability: Capability<&StarvingChildren.Admin>, 
             receiverCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
             collectionCapability: Capability<&{StarvingChildren.StarvingChildrenCollectionPublic}>) {
            pre {
                // Check that the owner's NFT collection capability is correct
                adminCapability.borrow() != nil: 
                    "Admin Capability is invalid!"
                receiverCapability.borrow() != nil: 
                    "FUSD receiver Capability is invalid!"
                collectionCapability.borrow() != nil: 
                    "Collection Capability is invalid!"
            }
            
            self.adminCapability = adminCapability
            self.receiverCapability = receiverCapability
            self.collectionCapability = collectionCapability
        }
        
        /// purchase lets a user send tokens to purchase an NFT that is for sale
        /// the purchased NFT is returned to the transaction context that called it
        ///
        /// Parameters: templateId: the template ID of the NFT to purchase
        ///             buyTokens: the fungible tokens that are used to buy the NFT
        ///             buyerAddress: the buyer wallet address used to send on event
        ///
        /// Returns: @StarvingChildren.NFT: the purchased NFT
        pub fun purchase(templateId: UInt64, buyTokens: &FungibleToken.Vault, buyerAddress: Address): @StarvingChildren.NFT {
            pre {
                StarvingChildren.getMetadatas().containsKey(templateId) : "templateId not found"
                buyTokens.isInstance(Type<@FUSD.Vault>()):"payment vault is not requested fungible token"
                StarvingChildren.getMetadatas()[templateId]!.price > 0.0 :"price must be more than zero "
                StarvingChildren.getMetadatas()[templateId]!.maxEditions > StarvingChildren.getNumberMintedByTemplate(templateId:templateId)! : "template not available"
            }

            let template = StarvingChildren.getMetadatas()[templateId]!
            let price = template.price

            // Withdraw funds
            let buyerVault <-buyTokens.withdraw(amount: price)
            self.receiverCapability.borrow()!.deposit(from: <-buyerVault)

            // Mint NFTs
            let nftBuyer <- self.adminCapability.borrow()!.mintNFT(metadata:template.metadataBuyer, templateId:templateId, buyer: true)
            let nftCharity <- self.adminCapability.borrow()!.mintNFT(metadata:template.metadataCharity, templateId:templateId, buyer: false)
            
            emit StarvingChildrenPurchased(idBuyer: nftBuyer.id, idCharity: nftCharity.id, templateId: templateId, buyerAddress: buyerAddress, charityAddress: StarvingChildren.getAdminAddress()!)
            
            // Transfer NFT to charity
            self.collectionCapability.borrow()!.deposit(token: <-nftCharity)

            return <- nftBuyer
        }
    }

    // -----------------------------------------------------------------------
    // StarvingChildrenMarket contract-level function definitions
    // -----------------------------------------------------------------------

    // createManager creates a new Manager resource
    //
    pub fun createManager(adminCapability: Capability<&StarvingChildren.Admin>, receiverCapability: Capability<&FUSD.Vault{FungibleToken.Receiver}>,
             collectionCapability: Capability<&{StarvingChildren.StarvingChildrenCollectionPublic}>): @Manager {
        return <- create Manager(adminCapability: adminCapability, receiverCapability:receiverCapability, collectionCapability:collectionCapability)
    }

    init() {
        self.marketStoragePath = /storage/starvingMarketCollection
        self.marketPublicPath = /public/starvingMarketCollection
    }
}
 
