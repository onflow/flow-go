import FungibleToken from "FungibleToken"
import FlowToken from "FlowToken"

transaction(amount: UFix64, recipient: Address) {

	let sentVault: @{FungibleToken.Vault}

	prepare(signer: auth(BorrowValue) &Account) {
	    let vaultRef = signer.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
		    ?? panic("failed to borrow reference to sender vault")
	    self.sentVault <- vaultRef.withdraw(amount: amount)
	}

	execute {
	    let receiverRef =  getAccount(recipient)
		    .capabilities.borrow<&FlowToken.Vault>(/public/flowTokenReceiver)
		    ?? panic("failed to borrow reference to recipient vault")
	    receiverRef.deposit(from: <-self.sentVault)
	}
}
