
pub contract interface FungibleToken {

    pub resource interface Provider {

        pub fun withdraw(amount: Int): <-Vault {
            pre {
                amount > 0:
                    "Withdrawal amount must be positive"
            }
            post {
                result.balance == amount:
                    "Incorrect amount returned"
            }
        }
    }

    pub resource interface Receiver {

        pub fun deposit(vault: <-Vault)
    }

    pub resource Vault: Provider, Receiver {

        pub balance: Int {
            get {
                post {
                    result >= 0:
                        "Balances are always non-negative"
                }
            }
        }

        init(balance: Int) {
            pre {
                balance >= 0:
                    "Initial balance must be non-negative"
            }
            post {
                self.balance == balance:
                    "Balance must be initialized to the initial balance"
            }
        }

        pub fun withdraw(amount: Int): <-Vault {
            pre {
                amount <= self.balance:
                    "Insufficient funds"
            }
            post {
                self.balance == before(self.balance) - amount:
                    "Incorrect amount removed"
            }
        }

        pub fun deposit(vault: <-Vault) {
            post {
                self.balance == before(self.balance) + vault.balance:
                    "Incorrect amount removed"
            }
        }
    }

    pub fun absorb(vault: <-Vault) {
        pre {
            vault.balance == 0:
                "Can only absorb empty vaults"
        }
    }
}

pub abstract contract BasicToken: FungibleToken {

    pub resource Vault {

        pub var balance: Int

        pub fun withdraw(amount: Int): <-Vault {
            self.balance -= amount
            return create Vault(balance: amount)
        }

        pub fun deposit(from: <-Vault) {
            self.balance = self.balance + from.balance
            destroy from
        }

        init(balance: Int) {
            self.balance = balance
        }

        init() {
            self.balance = 0
        }
    }

    pub fun absorb(vault: <-Vault) {
        destroy vault
    }
}

pub contract DeteToken includes BasicToken {

    pub resource Minter: Provider {

        init() {}

        pub fun withdraw(amount: Int): <-Vault {
            return create Vault(balance: amount)
        }
    }
}

import Timestamp from "time"

pub resource Faucet: FungibleToken.Provider {

    private let source: <-FungibleToken.Provider
    private let dailyLimit: Int

    private var lastResetTime: Timestamp
    private var remainingAmount: Int

    pub fun withdraw(amount: Int): <-Vault {
        self.maybeReset()
        return self._withdraw(amount: amount)
    }

    private fun _withdraw(amount: Int): <-Vault {
        pre {
            self.amount <= self.remainingAmount:
                "The faucet has no more funds for today"
        }

        return self.source.withdraw(amount: amount)
    }

    private fun maybeReset() {
        let now = system.blockTime

        if now - self.lastResetTime < Time.days(1) {
            return
        }

        self.lastResetTime = now
        self.remainingAmount = self.dailyLimit
    }

    init(source: <-FungibleToken.Provider, dailyLimit: Int) {
        pre {
            dailyLimit > 0:
                "Daily limit must be positive"
        }

        self.source = source
        self.dailyLimit = dailyLimit

        self.lastResetTime = system.blockTime
        self.remainingAmount = dailyLimit
    }
}
