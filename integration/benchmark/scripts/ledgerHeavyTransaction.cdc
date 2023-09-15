import MyFavContract from 0x%s

transaction {
  prepare(acct: &Account) {}
  execute {
    MyFavContract.LedgerInteractionHeavy(700)
  }
}
