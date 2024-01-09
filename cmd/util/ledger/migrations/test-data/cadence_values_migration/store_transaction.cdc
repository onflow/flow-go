// This transaction is used to store values to the emulator state
// for testing the cadence value migrations.

transaction {

  prepare(acct: AuthAccount) {
    acct.save("Cafe\u{0301}", to: /storage/string_value_1)
    acct.save("Caf\u{00E9}", to: /storage/string_value_2)
    acct.save(Type<AuthAccount>(), to: /storage/type_value_1)
  }

  execute {
    log("Done!")
  }
}
