
// TODO:
// - How to get current total supply?
// - How to "instantiate" `Faucet` and `ApprovableProvider` for `DeteToken`?

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
            self.balance = self.balance - amount
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

import Timestamp from "time"

pub abstract resource Faucet: FungibleToken.Provider {

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

pub abstract resource ApprovableProvider: FungibleToken.Provider {

    private let source: <-FungibleToken.Provider
    private let approvals: Int[Address]

    init(source: <-FungibleToken.Provider) {
        self.source = source
    }

    pub fun approvedAmount(for address: Address): Int {
        return self.approvals[address] ?? 0
    }

    pub fun removeApproval(for address: Address) {
        self.approvals.remove(key: address)
    }

    pub fun setApproval(for address: Address, amount: Int) {
        pre {
            amount > 0:
                "Withdrawal amount must be positive"
        }
        post {
            self.approvedAmount(for address) == amount:
                "Approval amount must be set"
        }
        self.approvals[address] = amount
    }

    pub fun withdraw(amount: Int): <-Vault {
        pre {
            // TODO: what is with draw
            amount <= self.approvedAmount(for: address):
                "Withdrawal amount must be less or equal to approved amount"
        }
        post {
            self.approvalAmount(for: address) == before(self.approvalAmount(for: address)) - amount:
                "Approval amount must be decremented by amount"
        }

        let remainingApproval = self.approvals[address] ?? 0
        self.approvals[address] = remainingApproval - amount

        return self.source.withdraw(amount: amount)
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