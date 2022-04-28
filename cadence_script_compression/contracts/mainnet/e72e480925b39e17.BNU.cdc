import FungibleToken from 0xf233dcee88fe0abe

pub contract BNU: FungibleToken {

    pub event TokensInitialized(initialSupply: UFix64)
    pub event TokensWithdrawn(amount: UFix64, from: Address?)
    pub event TokensDeposited(amount: UFix64, to: Address?)
    pub event TokensMinted(amount: UFix64)
    pub event MinterCreated()

    // The storage path for the admin resource
    pub let AdminStoragePath: StoragePath;
    pub let StorageVaultPath: StoragePath;
    pub let BalancePublicPath: PublicPath;
    pub let ReceiverPath: PublicPath;

    // Total supply of bnu in existence
    pub var totalSupply: UFix64

    pub resource Vault: FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance {

        // holds the balance of a users tokens
        pub var balance: UFix64

        // initialize the balance at resource creation time
        init(balance: UFix64) {
            self.balance = balance
        }

        pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
            self.balance = self.balance - amount
            emit TokensWithdrawn(amount: amount, from: self.owner?.address)
            return <-create Vault(balance: amount)
        }

        pub fun deposit(from: @FungibleToken.Vault) {
            let vault <- from as! @BNU.Vault
            self.balance = self.balance + vault.balance
            emit TokensDeposited(amount: vault.balance, to: self.owner?.address)
            vault.balance = 0.0
            destroy vault
        }

        destroy() {
            BNU.totalSupply = BNU.totalSupply - self.balance
        }
    }

    pub fun createEmptyVault(): @BNU.Vault {
        return <-create Vault(balance: 0.0)
    }

    pub resource Minter {
        pub fun mintTokens(amount: UFix64): @BNU.Vault {
            pre {
                amount > 0.0: "Amount minted must be greater than zero"
            }
            BNU.totalSupply = BNU.totalSupply + amount
            emit TokensMinted(amount: amount)
            return <-create Vault(balance: amount)
        }
    }
    
    pub resource Administrator {
        pub fun createNewMinter(): @Minter {
            emit MinterCreated()
            return <- create Minter()
        }
    }

    init() {
        self.AdminStoragePath = /storage/bnuAdmin04
        self.ReceiverPath = /public/bnuReceiver04
        self.StorageVaultPath = /storage/bnuVault04
        self.BalancePublicPath = /public/bnuBalance04
        self.totalSupply = 0.0

        let admin <- create Administrator()

        // Emit an event that shows that the contract was initialized
        emit TokensInitialized(initialSupply: 0.0)

        self.account.save(<-admin, to: self.AdminStoragePath)

        self.account.link<&BNU.Vault{FungibleToken.Receiver}>(
            self.ReceiverPath,
            target: self.StorageVaultPath 
        )

        self.account.link<&BNU.Vault{FungibleToken.Balance}>(
            self.BalancePublicPath,
            target: self.StorageVaultPath 
        )
    }
}
