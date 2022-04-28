import StarlyToken from 0x142fa6570b62fd97
import FungibleToken from 0xf233dcee88fe0abe

pub contract StarlyTokenReward {

    pub event RewardPaid(rewardId: String, to: Address)

    pub let AdminStoragePath: StoragePath

    pub resource Admin {

        pub fun transfer(rewardId: String, to: Address, amount: UFix64) {
            let rewardsVaultRef = StarlyTokenReward.account.borrow<&StarlyToken.Vault>(from: StarlyToken.TokenStoragePath)!
            let receiverRef = getAccount(to).getCapability(StarlyToken.TokenPublicReceiverPath).borrow<&{FungibleToken.Receiver}>()
                ?? panic("Could not borrow StarlyToken receiver reference to the recipient's vault!")
            receiverRef.deposit(from: <-rewardsVaultRef.withdraw(amount: amount))
            emit RewardPaid(rewardId: rewardId, to: to)
        }
    }

    init() {
        self.AdminStoragePath = /storage/starlyTokenRewardAdmin

        let admin <- create Admin()
        self.account.save(<-admin, to: self.AdminStoragePath)

        // for payouts we will use account's default Starly token vault
        if (self.account.borrow<&StarlyToken.Vault>(from: StarlyToken.TokenStoragePath) == nil) {
            self.account.save(<-StarlyToken.createEmptyVault(), to: StarlyToken.TokenStoragePath)
            self.account.link<&StarlyToken.Vault{FungibleToken.Receiver}>(
                StarlyToken.TokenPublicReceiverPath,
                target: StarlyToken.TokenStoragePath)
            self.account.link<&StarlyToken.Vault{FungibleToken.Balance}>(
                StarlyToken.TokenPublicBalancePath,
                target: StarlyToken.TokenStoragePath)
        }
    }
}
