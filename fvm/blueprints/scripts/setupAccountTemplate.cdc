// This transaction is a template for a transaction
// to add a Vault resource to their account
// so that they can use the flowToken

import FungibleToken from 0xFUNGIBLETOKENADDRESS
import FlowToken from 0xFLOWTOKENADDRESS

transaction {

    prepare(signer: auth(Storage, Capabilities) &Account) {

        if signer.storage.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault) == nil {
            // Create a new flowToken Vault and put it in storage
            signer.storage.save(<-FlowToken.createEmptyVault(), to: /storage/flowTokenVault)

            // Create a public capability to the Vault that only exposes
            // the deposit function through the Receiver interface
            let receiverCap = signer.capabilities.storage.issue<&FlowToken.Vault>(/storage/flowTokenVault)
            signer.capabilities.publish(receiverCap, at: /public/flowTokenReceiver)

            // Create a public capability to the Vault that only exposes
            // the balance field through the Balance interface
            let balanceCap = signer.capabilities.storage.issue<&FlowToken.Vault>(/storage/flowTokenVault)
            signer.capabilities.publish(balanceCap, at: /public/flowTokenBalance)
        }
    }
}
