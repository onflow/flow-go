transaction(code: String) {
  prepare(serviceAccount: auth(AddContract) &Account) {
	serviceAccount.contracts.add(name: "RandomBeaconHistory", code: code.utf8)
  }
}
