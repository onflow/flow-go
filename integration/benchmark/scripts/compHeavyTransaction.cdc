import ComputationHeavy from 0x%s

transaction {
  prepare(acct: AuthAccount) {}
  execute {
    ComputationHeavy.ComputationHeavy(15000)
  }
}
