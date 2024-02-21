import LedgerHeavy from 0x%s

transaction {
  prepare(acct: AuthAccount) {}
  execute {
    LedgerHeavy.LedgerInteractionHeavy(700)
  }
}
