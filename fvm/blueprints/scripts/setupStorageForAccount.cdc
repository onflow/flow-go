import FlowServiceAccount from 0xFLOWSERVICEADDRESS
import FlowStorageFees from 0xFLOWSTORAGEFEESADDRESS
import FungibleToken from 0xFUNGIBLETOKENADDRESS
import FlowToken from 0xFLOWTOKENADDRESS

// This transaction sets up storage on a auth account.
// This is used during bootstrapping a local environment
transaction() {
    prepare(account: AuthAccount, service: AuthAccount) {

        // take all the funds from the service account
        let tokenVault = service.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
            ?? panic("Unable to borrow reference to the default token vault")

        let storageReservation <- tokenVault.withdraw(amount: FlowStorageFees.minimumStorageReservation) as! @FlowToken.Vault
        let hasReceiver = account.getCapability(/public/flowTokenReceiver)!.check<&{FungibleToken.Receiver}>()
        if !hasReceiver {
            FlowServiceAccount.initDefaultToken(account)
        }
        let receiver = account.getCapability(/public/flowTokenReceiver)!.borrow<&{FungibleToken.Receiver}>()
            ?? panic("Could not borrow receiver reference to the recipient's Vault")

        receiver.deposit(from: <-storageReservation)
    }
}
