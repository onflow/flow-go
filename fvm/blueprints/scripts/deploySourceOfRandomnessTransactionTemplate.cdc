transaction(code: String) {
  prepare(serviceAccount: AuthAccount) {
	serviceAccount.contracts.add(name: "SourceOfRandomness", code: code.decodeHex())
  }
}
