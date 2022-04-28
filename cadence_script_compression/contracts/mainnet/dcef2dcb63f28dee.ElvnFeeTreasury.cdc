// SPDX-License-Identifier: Apache License 2.0
import FungibleToken from 0xf233dcee88fe0abe
import Elvn from 0x6292b23b3eb3f999

pub contract ElvnFeeTreasury {
    pub event Initialize()

    pub event Withdrawn(amount: UFix64)

    pub event Deposited(amount: UFix64)

    pub resource Administrator {
	pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
	    pre {
		amount > 0.0: "amount is not positive"
	    }
	    let vault = ElvnFeeTreasury.getVault()

	    let vaultAmount = vault.balance
	    if vaultAmount < amount {
		panic("not enough balance in vault")
	    }

	    emit Withdrawn(amount: amount)
	    return <- vault.withdraw(amount: amount)
    	}

	pub fun withdrawAllAmount(): @FungibleToken.Vault {
	    let vault = ElvnFeeTreasury.getVault()
	    let vaultAmount = vault.balance
	    if vaultAmount <= 0.0 {
		panic("not enough balance in vault")
	    }

	    emit Withdrawn(amount: vaultAmount)
	    return <- vault.withdraw(amount: vaultAmount)
	}
    }

    pub fun deposit(vault: @FungibleToken.Vault) {
	pre {
	    vault.balance > 0.0: "amount is not positive"
	}
	let treasuryVault = ElvnFeeTreasury.getVault()

	let amount = vault.balance
	treasuryVault.deposit(from: <- vault)

	emit Deposited(amount: amount)
    }

    access(self) fun getVault(): &Elvn.Vault {
	return self.account.borrow<&Elvn.Vault>(from: /storage/elvnVault) ?? panic("failed borrow elvn vault")
    }

    pub fun getBalance(): UFix64 {
	let vault = ElvnFeeTreasury.getVault()

	return vault.balance
    }

    pub fun getReceiver(): Capability<&Elvn.Vault{FungibleToken.Receiver}> {
	return self.account.getCapability<&Elvn.Vault{FungibleToken.Receiver}>(/public/elvnReceiver)
    }

    init() { 
	let admin <- create Administrator()
	self.account.save(<- admin, to: /storage/elvnFeeTreasuryAdmin)

        if self.account.borrow<&Elvn.Vault>(from: /storage/elvnVault) == nil {
            self.account.save(<-Elvn.createEmptyVault(), to: /storage/elvnVault)

            self.account.link<&Elvn.Vault{FungibleToken.Receiver}>(
                /public/elvnReceiver,
                target: /storage/elvnVault
            )

            self.account.link<&Elvn.Vault{FungibleToken.Balance}>(
                /public/elvnBalance,
                target: /storage/elvnVault
            )
        }

	emit Initialize()
    }
}
 