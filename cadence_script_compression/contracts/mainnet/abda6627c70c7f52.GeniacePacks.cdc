import NonFungibleToken from 0x1d7e57aa55817448;
import FungibleToken from 0xf233dcee88fe0abe;
import GeniaceAuction from 0xabda6627c70c7f52;


// This contract is to support the mystery packs functionality in Geniace platform
// In here, there are functions to do manage the payment for the mysterypack, as well as
// there is a resource that is to support the concept of collection capability transfer
// which is similar to the concept of allownce in erc721 Ethereum standerad 
pub contract GeniacePacks{

    pub struct SaleCutEventData {
        
        // Address of the salecut receiver
        pub let receiver: Address

        // The amount of the payment FungibleToken that will be paid to the receiver.
        pub let percentage: UFix64

        // initializer
        //
        init(receiver: Address, percentage: UFix64) {
            self.receiver = receiver
            self.percentage = percentage
        }
    }

    pub event Purchased(
        collectionName: String,
        tier: String,
        packIDs: [String],
        price: UFix64,
        currency: Type,
        buyerAddress: Address,
        saleCuts: [SaleCutEventData])

    pub fun purchase(
        collectionName: String,
        tier: String,
        paymentVaultType: Type,
        packIDs: [String],
        price: UFix64,
        buyerAddress: Address,
        paymentVault: @FungibleToken.Vault,
        saleCuts: [GeniaceAuction.SaleCut]) {

        pre {
            paymentVault.balance == price: "payment does not equal offer price"
            paymentVault.isInstance(paymentVaultType): "payment vault is not requested fungible token"
        }

        // Rather than aborting the transaction if any receiver is absent when we try to pay it,
        // we send the cut to the first valid receiver.
        // The first receiver should therefore either be the seller, or an agreed recipient for
        // any unpaid cuts.
        var residualReceiver: &{FungibleToken.Receiver}? = nil

        // This struct is a helper to map the sale cut address and percentage
        // and pass it into the events

        var saleCutsEventMapping: [SaleCutEventData] = []

        // Pay each beneficiary their amount of the payment.
        for cut in saleCuts {
            if let receiver = cut.receiver.borrow() {

                //Withdraw cutPercentage to marketplace and put it in their vault
                let amount=price*cut.percentage
                let paymentCut <- paymentVault.withdraw(amount: amount)
                receiver.deposit(from: <-paymentCut)
                if (residualReceiver == nil) {
                    residualReceiver = receiver
                }

                // 
                saleCutsEventMapping.append(
                    SaleCutEventData(
                        receiver: receiver.owner!.address,
                        percentage: cut.percentage
                    )
                )
            }
        }

        assert(residualReceiver != nil, message: "No valid payment receivers")

        // At this point, if all recievers were active and availabile, then the payment Vault will have
        // zero tokens left, and this will functionally be a no-op that consumes the empty vault
        residualReceiver!.deposit(from: <-paymentVault.withdraw(amount: paymentVault.balance))

        destroy paymentVault

        emit Purchased(
            collectionName: collectionName,
            tier: tier,
            packIDs: packIDs,
            price: price,
            currency: paymentVaultType,
            buyerAddress: buyerAddress,
            saleCuts: saleCutsEventMapping
        )
    }

    // publically assessible functions
    pub resource interface collectionCapabilityPublic {
        pub fun setCollectionCapability(collectionOwner: Address, capability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>)
        pub fun isCapabilityExist(collectionOwner: Address): Bool
    }

    // Prviate functions
    pub resource interface collectionCapabilityManager {
        pub fun getCollectionCapability(collectionOwner: Address): &{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}
    }

    // This resource is using to hold an NFT Collection Capabilty of multiple accounts,
    // In this way one account can transfer NFT's behalf of it's original owner
    // This is work similar to the concept of approval-allowance of the erc721 standerad 
    pub resource collectionCapabilityHolder: collectionCapabilityPublic, collectionCapabilityManager{
        
        // This dictionary variable will hold the address-capabilty pair
        access(self) var collectionCapabilityList: {Address: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>}

        // The owner of the NFT collection will create a private capability of his collection
        // transer it to the collection holder using this publically assessible function
        pub fun setCollectionCapability(collectionOwner: Address, capability: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>){
            self.collectionCapabilityList[collectionOwner] = capability
        }

        // This publically accessible function will be used to check weather a collection capability
        // of a specific account is available or not
        pub fun isCapabilityExist(collectionOwner: Address): Bool {
            let ref = self.collectionCapabilityList[collectionOwner]?.borrow()
                if(ref == nil){
                    return false
                }
                return true
            }

        // This private function can be used to fetch the stored capability of a specific user
        // and can call the private functions such as 'withdraw'
        pub fun getCollectionCapability(collectionOwner: Address): &{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic} {
            let ref = self.collectionCapabilityList[collectionOwner]!.borrow()!
            return ref
        } 

        init(){
            self.collectionCapabilityList = {}
        }
    }

    // Public function to create and return collectionCapabilityHolder resource
    pub fun createCapabilityHolder(): @collectionCapabilityHolder {
            return <-create collectionCapabilityHolder()
        }
}
