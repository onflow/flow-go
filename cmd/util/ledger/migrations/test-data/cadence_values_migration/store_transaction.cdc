import Test from 0x01cf0e2f2f715450

transaction {

  prepare(acct: AuthAccount) {
    acct.save("Cafe\u{0301}", to: /storage/string_value_1)
    acct.save("Caf\u{00E9}", to: /storage/string_value_2)
    acct.save(Type<AuthAccount>(), to: /storage/type_value)

    // String keys in dictionary
    acct.save(
      {
        "Cafe\u{0301}": 1,
        "H\u{00E9}llo": 2
      },
      to: /storage/dictionary_with_string_keys,
    )

    // Restricted typed keys in dictionary
    acct.save(
      {
        Type<AnyStruct{Test.Bar, Test.Foo}>(): 1,
        Type<AnyStruct{Test.Foo, Test.Bar, Test.Baz}>(): 2
      },
      to: /storage/dictionary_with_restricted_typed_keys,
    )

    // Capabilities and links
    acct.save(<- Test.createR(), to: /storage/r)
    var cap = acct.link<&Test.R>(/public/linkR, target: /storage/r)
    acct.save(cap, to: /storage/capability)

    // account-typed keys in dictionary
    acct.save(
      {
        Type<AuthAccount>(): 1,
        Type<AuthAccount.Capabilities>(): 2,
        Type<AuthAccount.AccountCapabilities>(): 3,
        Type<AuthAccount.StorageCapabilities>(): 4,
        Type<AuthAccount.Contracts>(): 5,
        Type<AuthAccount.Keys>(): 6,
        Type<AuthAccount.Inbox>(): 7,

        Type<PublicAccount>(): 8,

        Type<AccountKey>(): 9
      },
      to: /storage/dictionary_with_account_type_keys,
    )
  }

  execute {
    log("Values successfully saved in storage")
  }
}
