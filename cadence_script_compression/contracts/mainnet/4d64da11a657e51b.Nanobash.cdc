// Digital Art as Fungible Tokens

// Each piece of artwork on Nanobash has a certain number of "editions".
// However, instead of these editions being numbered in the way NFTs generally are, they are fungible and interchangeable.
// This contract is used to create multiple tokens each representing a unique piece of artwork stored via ipfs within the metadata.
// Each token will have a predefined maximum quantity when created.
// Once the maximum number of editions are created or the token is locked, no additional editions will be able to be created.
// Accounts are free to send / exchange any of these tokens with one another.

pub contract Nanobash {

    // Total supply of all tokens in existence.
    pub var nextTokenID: UInt64

    access(self) var tokens: @{UInt64: Token}

    pub event ContractInitialized()
    pub event Withdraw(tokenID: UInt64, quantity: UInt64, from: Address?)
    pub event Deposit(tokenID: UInt64, quantity: UInt64, to: Address?)
    pub event TokenCreated(tokenID: UInt64, metadata: {String: String})
    pub event TokensMinted(tokenID: UInt64, quantity: UInt64)

    pub resource Token {
        pub let tokenID: UInt64
        pub let maxEditions: UInt64
        pub var numberMinted: UInt64

        // Intended to contain name and ipfs link to asset
        access (self) let metadata: {String: String}

        // Additional editions of tokens are un-mintable when the token is locked
        pub var locked: Bool

        init(metadata: {String: String}, maxEditions: UInt64) {
            self.tokenID = Nanobash.nextTokenID
            self.maxEditions = maxEditions
            self.numberMinted = 0
            self.metadata = metadata
            self.locked = false
            Nanobash.nextTokenID = Nanobash.nextTokenID + (1 as UInt64)
            emit TokenCreated(tokenID: self.tokenID, metadata: self.metadata)
        }

        // Lock the token to prevent additional editions being minted
        pub fun lock() {
            if !self.locked {
                self.locked = true
            }
        }

        pub fun getMetadata(): {String: String} {
            return self.metadata
        }

        // Creates a vault with a specific quantity of the current token
        pub fun mintTokens(quantity: UInt64): @Vault {
            pre {
                !self.locked: "Unable to mint an edition after the piece has been locked."
                self.numberMinted + quantity <= self.maxEditions: "Maximum editions minted."
            }

            self.numberMinted = self.numberMinted + quantity
            emit TokensMinted(tokenID: self.tokenID, quantity: quantity)

            return <-create Vault(balances: {self.tokenID: quantity})
        }
    }

    pub resource Admin {
        // Allows creation of new token types by the admin
        pub fun createToken(metadata: {String: String}, maxEditions: UInt64): UInt64 {
            var newToken <- create Token(metadata: metadata, maxEditions: maxEditions)
            let newID = newToken.tokenID

            Nanobash.tokens[newID] <-! newToken
            return newID
        }

        // Allow admins to borrow the token instance for token specific methods
        pub fun borrowToken(tokenID: UInt64): &Token {
            pre {
                Nanobash.tokens[tokenID] != nil: "Cannot borrow Token: The Token doesn't exist"
            }

            return &Nanobash.tokens[tokenID] as &Token
        }
    }

    pub resource interface Provider {
        pub fun withdraw(amount: UInt64, tokenID: UInt64): @Vault {
            post {
                result.getBalance(tokenID: tokenID) == UInt64(amount):
                    "Withdrawal amount must be the same as the balance of the withdrawn Vault"
            }
        }
    }

	pub resource interface Receiver {
        pub fun deposit(from: @Vault)
    }

    pub resource interface Balance {
        pub fun getBalance(tokenID: UInt64): UInt64
        pub fun getBalances(): {UInt64: UInt64}
    }

    pub resource Vault: Provider, Receiver, Balance {
        // Map of tokenIDs to # of editions within the vault
        access (self) let balances: {UInt64: UInt64}

        init(balances: {UInt64: UInt64}) {
            self.balances = balances
        }

        pub fun getBalances(): {UInt64: UInt64} {
            return self.balances
        }

        pub fun getBalance(tokenID: UInt64): UInt64 {
            return self.balances[tokenID] ?? 0 as UInt64
        }

        pub fun withdraw(amount: UInt64, tokenID: UInt64): @Vault {
            pre {
                self.balances[tokenID] != nil: "User does not own this token"
                self.balances[tokenID]! >= amount: "Insufficient tokens"
            }
            let balance: UInt64 = self.balances[tokenID]!
            self.balances[tokenID] = balance - amount
            emit Withdraw(tokenID: tokenID, quantity: amount, from: self.owner?.address)
            return <-create Vault(balances: {tokenID: amount})
        }

        pub fun deposit(from: @Vault) {
            // For each entry in our {tokenID: quantity} map, add tokens in vault to account balances

            let tokenIDs = from.balances.keys
            for tokenID in tokenIDs {
                // Create empty balance for any tokens not yet owned
                if self.balances[tokenID] == nil {
                    self.balances[tokenID] = 0
                }
                emit Deposit(tokenID: tokenID, quantity: from.balances[tokenID]!, to: self.owner?.address)
                self.balances[tokenID] = self.balances[tokenID]! + from.balances[tokenID]!
            }

            destroy from
        }
    }

    pub fun createEmptyVault(): @Vault {
        return <-create Vault(balances: {})
    }

    pub fun getNumberMinted(tokenID: UInt64): UInt64 {
        return self.tokens[tokenID]?.numberMinted ?? 0 as UInt64
    }

    pub fun getMaxEditions(tokenID: UInt64): UInt64 {
        return self.tokens[tokenID]?.maxEditions ?? 0 as UInt64
    }

    pub fun getTokenMetadata(tokenID: UInt64): {String: String} {
        return self.tokens[tokenID]?.getMetadata() ?? {} as {String: String}
    }

    pub fun isTokenLocked(tokenID: UInt64): Bool {
        return Nanobash.tokens[tokenID]?.locked ?? false
    }

    init() {
        // A map of tokenIDs to Tokens contained within the contract
        self.tokens <- {}
        self.nextTokenID = 1

        // Create a vault for the contract creator
        let vault <- create Vault(balances: {})
        self.account.save(<-vault, to: /storage/MainVault)

        // Create an admin resource for token methods
        self.account.save<@Admin>(<- create Admin(), to: /storage/NanobashAdmin)
    }
}
