transaction(code: String) {
  prepare(flowTokenAccount: AuthAccount, serviceAccount: AuthAccount) {
    let adminAccount = serviceAccount
    flowTokenAccount.contracts.add(name: "FlowToken", code: code.decodeHex(), adminAccount: adminAccount)
  }
}
