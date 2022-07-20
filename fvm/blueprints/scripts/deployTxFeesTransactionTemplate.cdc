transaction(code: String) {
  prepare(flowFeesAccount: AuthAccount, serviceAccount: AuthAccount) {
    let adminAccount = serviceAccount
    flowFeesAccount.contracts.add(name: "FlowFees", code: code.decodeHex(), adminAccount: adminAccount)
  }
}
