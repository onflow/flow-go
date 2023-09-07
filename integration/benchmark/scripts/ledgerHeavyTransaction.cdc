import MyFavContract from 0x%s

transaction {
  prepare(acct: AuthAccount) {}
  execute {
    MyFavContract.LedgerInteractionHeavy(700)
  }
}
