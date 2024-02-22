transaction(code: String) {
  prepare(flowFeesAccount: auth(AddContract) &Account, serviceAccount: auth(SaveValue) &Account) {
    let adminAccount = serviceAccount
    flowFeesAccount.contracts.add(name: "FlowFees", code: code.decodeHex(), adminAccount: adminAccount)
  }
}
