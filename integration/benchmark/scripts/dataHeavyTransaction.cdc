import DataHeavy from 0x%s

transaction {
  prepare(acct: &Account) {}
  execute {
    DataHeavy.LedgerInteractionHeavy(100)
    DataHeavy.EventHeavy(100)
  }
}
