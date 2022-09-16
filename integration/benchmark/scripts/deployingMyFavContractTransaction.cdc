transaction {
  prepare(signer: AuthAccount) {
		signer.contracts.add(name: "%s", code: "%s".decodeHex())
  }
}
