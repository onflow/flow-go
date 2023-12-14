import FlowServiceAccount from 0xFLOWSERVICEADDRESS
import FlowStorageFees from 0xFLOWSTORAGEFEESADDRESS
import FungibleToken from 0xFUNGIBLETOKENADDRESS
import FlowToken from 0xFLOWTOKENADDRESS

// This transaction sets up storage on a auth account.
// This is used during bootstrapping a local environment
transaction() {
    prepare(
        account: auth(SaveValue, Capabilities) &Account,
        service: auth(BorrowValue) &Account
    ) {
        // take all the funds from the service account
        let tokenVault = service.storage
            .borrow<auth(FungibleToken.Withdrawable) &FlowToken.Vault>(from: /storage/flowTokenVault)
            ?? panic("Unable to borrow reference to the default token vault")

        let storageReservation <- tokenVault.withdraw(amount: FlowStorageFees.minimumStorageReservation) as! @FlowToken.Vault

        let hasReceiver = account.capabilities
            .get<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)?.check() ?? false
        if !hasReceiver {
            FlowServiceAccount.initDefaultToken(account)
        }

        let receiver = account.capabilities
            .borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
            ?? panic("Could not borrow receiver reference to the recipient's Vault")

        receiver.deposit(from: <-storageReservation)
    }
}
