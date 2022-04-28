// SPDX-License-Identifier: Apache License 2.0
import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393
import Elvn from 0x6292b23b3eb3f999

pub contract ElvnPackPurchaseTreasury {
    access(contract) let elvnVault: @Elvn.Vault

    pub event Initialize()

    pub event WithdrawnElvn(amount: UFix64)
    pub event DepositedElvn(amount: UFix64)

    pub resource ElvnAdministrator {
	pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
	    pre {
		amount > 0.0: "amount is not positive"
	    }
	    let vaultAmount = ElvnPackPurchaseTreasury.elvnVault.balance
	    if vaultAmount < amount {
		panic("not enough balance in vault")
	    }

	    emit WithdrawnElvn(amount: amount)
	    return <- ElvnPackPurchaseTreasury.elvnVault.withdraw(amount: amount)
    	}

	pub fun withdrawAllAmount(): @FungibleToken.Vault {
	    let vaultAmount = ElvnPackPurchaseTreasury.elvnVault.balance
	    if vaultAmount <= 0.0 {
		panic("not enough balance in vault")
	    }

	    emit WithdrawnElvn(amount: vaultAmount)
	    return <- ElvnPackPurchaseTreasury.elvnVault.withdraw(amount: vaultAmount)
	}
    }

    pub fun depositElvn(vault: @FungibleToken.Vault) {
	pre {
	    vault.balance > 0.0: "amount is not positive"
	}
	let amount = vault.balance
	self.elvnVault.deposit(from: <- vault)

	emit DepositedElvn(amount: amount)
    }

    pub fun getBalance(): UFix64 {
	return self.elvnVault.balance
    }

    init() { 
	self.elvnVault <- Elvn.createEmptyVault() as! @Elvn.Vault

	let elvnAdmin <- create ElvnAdministrator()
	self.account.save(<- elvnAdmin, to: /storage/packPurchaseTreasuryAdmin)

	emit Initialize()
    }
}
 