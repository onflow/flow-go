import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction(publicKey: [UInt8], count: Int, initialTokenAmount: UFix64) {
  prepare(signer: auth(AddContract) &Account) {
	let vault = signer.borrow<auth(FungibleToken.Withdrawable) &FlowToken.Vault>(from: /storage/flowTokenVault)
      ?? panic("Could not borrow reference to the owner's Vault")

    var i = 0
    while i < count {
      let account = Account(payer: signer)
      let publicKey2 = PublicKey(
        publicKey: publicKey,
        signatureAlgorithm: SignatureAlgorithm.ECDSA_P256
      )
      account.keys.add(
        publicKey: publicKey2,
        hashAlgorithm: HashAlgorithm.SHA3_256,
        weight: 1000.0
      )

	  let receiver = account.capabilities.get<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)!
        .borrow()
		?? panic("Could not borrow receiver reference to the recipient's Vault")

      receiver.deposit(from: <-vault.withdraw(amount: initialTokenAmount))

      i = i + 1
    }
  }
}
