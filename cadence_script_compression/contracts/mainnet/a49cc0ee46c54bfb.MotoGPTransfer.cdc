import FlowToken from 0x1654653399040a61
import MotoGPAdmin from 0xa49cc0ee46c54bfb
import MotoGPPack from 0xa49cc0ee46c54bfb
import MotoGPCard from 0xa49cc0ee46c54bfb
import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import FlowStorageFees from 0xe467b9dd11fa00df
import ContractVersion from 0xa49cc0ee46c54bfb
import PackOpener from 0xa49cc0ee46c54bfb

// Contract for topping up an account's storage capacity when it receives a MotoGP pack or card
//
pub contract MotoGPTransfer: ContractVersion {

    pub fun getVersion(): String {
        return "0.7.8"
    }

    // The minium amount to top up
    //
    access(account) var minFlowTopUp:UFix64

    // The maximum amount to top up
    //
    access(account) var maxFlowTopUp:UFix64

    // Vault where the admin stores Flow tokens to pay for top-ups
    //
    access(self) var flowVault: @FlowToken.Vault

    access(self) var isPaused: Bool //for future use

    // Transfers packs from one collection to another, with storage top-up if needed
    //
    pub fun transferPacks(fromCollection: @MotoGPPack.Collection{NonFungibleToken.Provider, MotoGPPack.IPackCollectionPublic}, toCollection: &MotoGPPack.Collection{MotoGPPack.IPackCollectionPublic}, toAddress: Address) {

        pre {
            fromCollection.getIDs().length > 0 : "No packs in fromCollection"
        }

        for id in fromCollection.getIDs() {
            toCollection.deposit(token: <- fromCollection.withdraw(withdrawID: id))
        }

        self.topUp(toAddress)

        destroy fromCollection
    }

    // Transfer cards from one collection to another, with storage top-up if needed
    //
    pub fun transferCards(fromCollection: @MotoGPCard.Collection{NonFungibleToken.Provider, MotoGPCard.ICardCollectionPublic}, toCollection: &MotoGPCard.Collection{MotoGPCard.ICardCollectionPublic}, toAddress: Address) {
        pre {
            fromCollection.getIDs().length > 0 : "No cards in fromCollection"
        }

        for id in fromCollection.getIDs() {
            toCollection.deposit(token: <- fromCollection.withdraw(withdrawID: id))
        }

        self.topUp(toAddress)

        destroy fromCollection
    }

    // Transfer a pack to a Pack opener collection, with storage top-up if needed
    pub fun transferPackToPackOpenerCollection(pack: @MotoGPPack.NFT, toCollection: &PackOpener.Collection{PackOpener.IPackOpenerPublic}, toAddress: Address) {
        toCollection.deposit(token: <- pack)
        self.topUp(toAddress)
    }

    // Admin-controlled method for use in transactions where admin wants to do top-up, e.g. open packs
    //
    pub fun topUpFlowForAccount(adminRef: &MotoGPAdmin.Admin, toAddress: Address){
        pre {
            adminRef != nil : "AdminRef is nil"
        }
        self.topUp(toAddress)
    }

    // Core logic for topping up an account
    //
    access(self) fun topUp(_ toAddress:Address){

        post {
            (before(self.flowVault.balance) - self.flowVault.balance) <= self.maxFlowTopUp : "Top up exceeds max top up"
        }

        let toAccount = getAccount(toAddress)

        if (toAccount.storageCapacity < toAccount.storageUsed){
            let topUpAmount:UFix64 = self.flowForStorage(toAccount.storageUsed - toAccount.storageCapacity)
            let toVault = toAccount.getCapability(/public/flowTokenReceiver).borrow<&FlowToken.Vault{FungibleToken.Receiver}>()!
            toVault.deposit(from: <- self.flowVault.withdraw(amount: topUpAmount))
        }
    } 

    // Converts storage bytes to a FLOW token amount
    //
    access(self) fun flowForStorage(_ storage:UInt64): UFix64 {
        return FlowStorageFees.storageCapacityToFlow(FlowStorageFees.convertUInt64StorageBytesToUFix64Megabytes(storage))
    }

    pub fun setMinFlowTopUp(adminRef: &MotoGPAdmin.Admin, amount: UFix64){
        pre {
            adminRef != nil: "adminRef is nil"
        }
        self.minFlowTopUp = amount
    }

    pub fun setMaxFlowTopUp(adminRef: &MotoGPAdmin.Admin, amount: UFix64){
        pre {
            adminRef != nil: "adminRef is nil"
        }
        self.maxFlowTopUp = amount
    }

    pub fun getFlowBalance(): UFix64 {
        return self.flowVault.balance
    }

    pub fun depositFlow(from: @FungibleToken.Vault){
        let vault <- from as! @FlowToken.Vault
        self.flowVault.deposit(from: <- vault)
    }

    pub fun withdrawFlow(adminRef: &MotoGPAdmin.Admin, amount: UFix64): @FungibleToken.Vault{
        pre {
            adminRef != nil: "adminRef is nil"
        }
        return <- self.flowVault.withdraw(amount: amount)
    }

    init(){
        self.flowVault <- FlowToken.createEmptyVault() as! @FlowToken.Vault
        self.minFlowTopUp = 0.0
        self.maxFlowTopUp = 0.1
        self.isPaused = false //for future use
    }

}
