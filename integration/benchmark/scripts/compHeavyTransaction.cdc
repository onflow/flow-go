import MyFavContract from 0x%s

transaction {
  prepare(acct: &Account) {}
  execute {
    MyFavContract.ComputationHeavy(15000)
  }
}
