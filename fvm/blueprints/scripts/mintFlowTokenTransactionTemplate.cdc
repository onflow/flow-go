import FungibleToken from 0xFUNGIBLETOKENADDRESS
import FlowToken from 0xFLOWTOKENADDRESS

transaction(amount: UFix64) {

  let tokenAdmin: &FlowToken.Administrator
  let tokenReceiver: &FlowToken.Vault

  prepare(signer: auth(BorrowValue) &Account) {
	self.tokenAdmin = signer.storage
	  .borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)
	  ?? panic("Signer is not the token admin")

	self.tokenReceiver = signer
	  .capabilities.get<&FlowToken.Vault>(/public/flowTokenReceiver)!
	  .borrow()
	  ?? panic("Unable to borrow receiver reference for recipient")
  }

  execute {
	let minter <- self.tokenAdmin.createNewMinter(allowedAmount: amount)
	let mintedVault <- minter.mintTokens(amount: amount)

	self.tokenReceiver.deposit(from: <-mintedVault)

	destroy minter
  }
}
