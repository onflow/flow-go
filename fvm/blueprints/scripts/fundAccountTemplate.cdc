import FungibleToken from 0xFUNGIBLETOKENADDRESS
import FlowToken from 0xFLOWTOKENADDRESS

transaction(amount: UFix64, recipient: Address) {
	let sentVault: @{FungibleToken.Vault}
	prepare(signer: AuthAccount) {
	let vaultRef = signer.borrow<auth(FungibleToken.Withdrawable) &FlowToken.Vault>(from: /storage/flowTokenVault)
		?? panic("failed to borrow reference to sender vault")
	self.sentVault <- vaultRef.withdraw(amount: amount)
	}
	execute {
	let receiverRef =  getAccount(recipient)
		.getCapability(/public/flowTokenReceiver)
		.borrow<&FlowToken.Vault>()
		?? panic("failed to borrow reference to recipient vault")
	receiverRef.deposit(from: <-self.sentVault)
	}
}
