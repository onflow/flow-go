import EventHeavy from 0x%s

transaction {
  prepare(acct: AuthAccount) {}
  execute {
    EventHeavy.EventHeavy(220)
  }
}
