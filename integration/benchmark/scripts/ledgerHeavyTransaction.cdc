import LedgerHeavy from 0x%s

transaction {
  prepare(acct: &Account) {}
  execute {
    LedgerHeavy.LedgerInteractionHeavy(700)
  }
}
