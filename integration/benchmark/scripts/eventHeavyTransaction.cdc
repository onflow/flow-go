import EventHeavy from 0x%s

transaction {
  prepare(acct: &Account) {}
  execute {
    EventHeavy.EventHeavy(220)
  }
}
