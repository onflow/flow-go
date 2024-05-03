import FungibleToken from 0xFUNGIBLETOKENADDRESS
import FlowToken from 0xTOKENADDRESS

transaction(amount: UFix64, to: [Address]) {
    let sentVault: @FungibleToken.Vault

    prepare(signer: AuthAccount) {
        let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
			?? panic("Could not borrow reference to the owner's Vault!")
        self.sentVault <- vaultRef.withdraw(amount: amount * UFix64(to.length))
    }

    execute {
        for recipient in to {
            let receiverRef =  getAccount(recipient)
                .getCapability(/public/flowTokenReceiver)
                .borrow<&{FungibleToken.Receiver}>()
                ?? panic("Could not borrow receiver reference to the recipient's Vault")
            receiverRef.deposit(from: <-self.sentVault.withdraw(amount: amount))
        }
        destroy self.sentVault
    }
}
