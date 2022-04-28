// SPDX-License-Identifier: Apache License 2.0
import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393
import Elvn from 0x6292b23b3eb3f999

pub contract ElvnFUSDTreasury {
    access(contract) let elvnVault: @Elvn.Vault
    access(contract) let fusdVault: @FUSD.Vault

    pub event Initialize()

    pub event WithdrawnElvn(amount: UFix64)
    pub event DepositedElvn(amount: UFix64)

    pub event WithdrawnFUSD(amount: UFix64)
    pub event DepositedFUSD(amount: UFix64)

    pub event SwapElvnToFUSD(amount: UFix64)
    pub event SwapFUSDToElvn(amount: UFix64)

    pub resource ElvnAdministrator {
	pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
	    pre {
		amount > 0.0: "amount is not positive"
	    }
	    let vaultAmount = ElvnFUSDTreasury.elvnVault.balance
	    if vaultAmount < amount {
		panic("not enough balance in vault")
	    }

	    emit WithdrawnElvn(amount: amount)
	    return <- ElvnFUSDTreasury.elvnVault.withdraw(amount: amount)
    	}

	pub fun withdrawAllAmount(): @FungibleToken.Vault {
	    let vaultAmount = ElvnFUSDTreasury.elvnVault.balance
	    if vaultAmount <= 0.0 {
		panic("not enough balance in vault")
	    }

	    emit WithdrawnElvn(amount: vaultAmount)
	    return <- ElvnFUSDTreasury.elvnVault.withdraw(amount: vaultAmount)
	}
    }

    pub resource FUSDAdministrator {
	pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
	    pre {
		amount > 0.0: "amount is not positive"
	    }
	    let vaultAmount = ElvnFUSDTreasury.fusdVault.balance
	    if vaultAmount < amount {
		panic("not enough balance in vault")
	    }

	    emit WithdrawnFUSD(amount: amount)
	    return <- ElvnFUSDTreasury.fusdVault.withdraw(amount: amount)
    	}

	pub fun withdrawAllAmount(): @FungibleToken.Vault {
	    let vaultAmount = ElvnFUSDTreasury.fusdVault.balance
	    if vaultAmount <= 0.0 {
		panic("not enough balance in vault")
	    }

	    emit WithdrawnFUSD(amount: vaultAmount)
	    return <- ElvnFUSDTreasury.fusdVault.withdraw(amount: vaultAmount)
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

    pub fun depositFUSD(vault: @FungibleToken.Vault) {
	pre {
	    vault.balance > 0.0: "amount is not positive"
	}
	let amount = vault.balance
	self.fusdVault.deposit(from: <- vault)

	emit DepositedFUSD(amount: amount)
    }

    pub fun swapElvnToFUSD(vault: @Elvn.Vault): @FungibleToken.Vault {
	let vaultAmount = vault.balance
	if vaultAmount <= 0.0 {
	    panic("vault balance is not positive")
	}
	if ElvnFUSDTreasury.fusdVault.balance < vaultAmount {
	    panic("not enough balance in Treasury.fusdVault")
	}

	self.elvnVault.deposit(from: <- vault) 
	emit SwapElvnToFUSD(amount: vaultAmount)
	return <- self.fusdVault.withdraw(amount: vaultAmount)
    }

    pub fun swapFUSDToElvn(vault: @FUSD.Vault): @FungibleToken.Vault {
	let vaultAmount = vault.balance
	if vaultAmount <= 0.0 {
	    panic("vault balance is not positive")
	}
	if ElvnFUSDTreasury.elvnVault.balance < vaultAmount {
	    panic("not enough balance in Treasury.elvnVault")
	}

	self.fusdVault.deposit(from: <- vault)
	emit SwapFUSDToElvn(amount: vaultAmount)
	return <- self.elvnVault.withdraw(amount: vaultAmount)
    }

    pub fun getBalance(): [UFix64] {
	return [self.elvnVault.balance, self.fusdVault.balance]
    }

    init() { 
	self.elvnVault <- Elvn.createEmptyVault() as! @Elvn.Vault
	self.fusdVault <- FUSD.createEmptyVault()

	let elvnAdmin <- create ElvnAdministrator()
	self.account.save(<- elvnAdmin, to: /storage/treasuryElvnAdmin)
	let fusdAdmin <- create FUSDAdministrator()
	self.account.save(<- fusdAdmin, to: /storage/treasuryFUSDAdmin)

	emit Initialize()
    }
}
 