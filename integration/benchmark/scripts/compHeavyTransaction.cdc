import ComputationHeavy from 0x%s

transaction {
  prepare(acct: &Account) {}
  execute {
    ComputationHeavy.ComputationHeavy(1500)
  }
}
