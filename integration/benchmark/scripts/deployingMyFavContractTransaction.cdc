transaction {
  prepare(signer: auth(AddContract) &Account) {
		signer.contracts.add(name: "%s", code: "%s".decodeHex())
  }
}
