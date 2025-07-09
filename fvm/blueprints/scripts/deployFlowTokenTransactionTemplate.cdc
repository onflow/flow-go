transaction(code: String) {
  prepare(flowTokenAccount: auth(AddContract) &Account, serviceAccount: auth(Storage, Capabilities) &Account) {
    let adminAccount = serviceAccount
    flowTokenAccount.contracts.add(name: "FlowToken", code: code.decodeHex(), adminAccount: adminAccount)
  }
}
