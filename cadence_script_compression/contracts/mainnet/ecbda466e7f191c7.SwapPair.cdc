/**

# SwapPair

# Author: Increment Labs

*/
import FungibleToken from 0xf233dcee88fe0abe
import SwapInterfaces from 0xb78ef7afa52ff906
import SwapConfig from 0xb78ef7afa52ff906
import SwapError from 0xb78ef7afa52ff906
import SwapFactory from 0xb063c16cac85dbd1

pub contract SwapPair: FungibleToken {
    /// Total supply of pair lpTokens in existence
    pub var totalSupply: UFix64
    
    /// Two vaults for the trading pair.
    access(self) let token0Vault: @FungibleToken.Vault
    access(self) let token1Vault: @FungibleToken.Vault
    pub let token0VaultType: Type
    pub let token1VaultType: Type
    pub let token0Key: String
    pub let token1Key: String
    
    /// TWAP: last cumulative price
    pub var blockTimestampLast: UFix64
    pub var price0CumulativeLastScaled: UInt256
    pub var price1CumulativeLastScaled: UInt256

    /// Transaction lock 
    access(self) var lock: Bool
    
    /// √(reserve0 * reserve1), as of immediately after the most recent liquidity event
    pub var rootKLast: UFix64

    /// Reserved parameter fields: {ParamName: Value}
    access(self) let _reservedFields: {String: AnyStruct}

    /// Event that is emitted when the contract is created
    pub event TokensInitialized(initialSupply: UFix64)
    /// Event that is emitted when tokens are withdrawn from a Vault
    pub event TokensWithdrawn(amount: UFix64, from: Address?)
    /// Event that is emitted when tokens are deposited to a Vault
    pub event TokensDeposited(amount: UFix64, to: Address?)
    /// Event that is emitted when new tokens are minted
    pub event TokensMinted(amount: UFix64)
    /// Event that is emitted when tokens are destroyed
    pub event TokensBurned(amount: UFix64)
    /// Event that is emitted when a swap trade happenes to this trading pair
    /// direction: 0 - in self.token0 swapped to out self.token1
    ///            1 - in self.token1 swapped to out self.token0
    pub event Swap(inTokenAmount: UFix64, outTokenAmount: UFix64, direction: UInt8)

    /// Lptoken Vault
    ///
    pub resource Vault: FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance {
        /// Holds the balance of a users tokens
        pub var balance: UFix64

        /// Initialize the balance at resource creation time
        init(balance: UFix64) {
            self.balance = balance
        }

        /// withdraw
        ///
        /// Function that takes an integer amount as an argument
        /// and withdraws that amount from the Vault.
        /// It creates a new temporary Vault that is used to hold
        /// the money that is being transferred. It returns the newly
        /// created Vault to the context that called so it can be deposited
        /// elsewhere.
        ///
        pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
            self.balance = self.balance - amount
            emit TokensWithdrawn(amount: amount, from: self.owner?.address)
            return <-create Vault(balance: amount)
        }

        /// deposit
        ///
        /// Function that takes a Vault object as an argument and adds
        /// its balance to the balance of the owners Vault.
        /// It is allowed to destroy the sent Vault because the Vault
        /// was a temporary holder of the tokens. The Vault's balance has
        /// been consumed and therefore can be destroyed.
        ///
        pub fun deposit(from: @FungibleToken.Vault) {
            let vault <- from as! @SwapPair.Vault
            self.balance = self.balance + vault.balance
            emit TokensDeposited(amount: vault.balance, to: self.owner?.address)
            vault.balance = 0.0
            destroy vault
        }

        destroy() {
            SwapPair.totalSupply = SwapPair.totalSupply - self.balance
        }
    }

    /// createEmptyVault
    //
    /// Function that creates a new Vault with a balance of zero
    /// and returns it to the calling context. A user must call this function
    /// and store the returned Vault in their storage in order to allow their
    /// account to be able to receive deposits of this token type.
    ///
    pub fun createEmptyVault(): @FungibleToken.Vault {
        return <-create Vault(balance: 0.0)
    }
    
    /// Permanently lock the first 0.00000001 lpTokens
    access(self) fun donateInitialMinimumLpToken() {
        self.totalSupply = self.totalSupply + SwapConfig.ufix64NonZeroMin
        emit TokensMinted(amount: SwapConfig.ufix64NonZeroMin)
    }

    /// Mint lpTokens
    access(self) fun mintLpToken(amount: UFix64): @SwapPair.Vault {
        self.totalSupply = self.totalSupply + amount
        emit TokensMinted(amount: amount)
        return <- create Vault(balance: amount)
    } 

    /// Burn lpTokens
    access(self) fun burnLpToken(from: @SwapPair.Vault) {
        let amount = from.balance
        destroy from
        emit TokensBurned(amount: amount)
    }

    /// Add liquidity
    ///
    pub fun addLiquidity(tokenAVault: @FungibleToken.Vault, tokenBVault: @FungibleToken.Vault): @FungibleToken.Vault {
        pre {
            tokenAVault.balance > 0.0 && tokenBVault.balance > 0.0:
                SwapError.ErrorEncode(
                    msg: "SwapPair: added zero liquidity",
                    err: SwapError.ErrorCode.ADD_ZERO_LIQUIDITY
                )
            (tokenAVault.isInstance(self.token0VaultType) && tokenBVault.isInstance(self.token1VaultType)) || 
            (tokenBVault.isInstance(self.token0VaultType) && tokenAVault.isInstance(self.token1VaultType)):
                SwapError.ErrorEncode(
                    msg: "SwapPair: added incompatible liquidity pair vaults",
                    err: SwapError.ErrorCode.INVALID_PARAMETERS
                )
            self.lock == false: SwapError.ErrorEncode(msg: "SwapPair: Reentrant", err: SwapError.ErrorCode.REENTRANT)
        }
        post {
            self.lock == false: "SwapPair: unlock"
        }
        self.lock = true

        let reserve0LastScaled = SwapConfig.UFix64ToScaledUInt256(self.token0Vault.balance)
        let reserve1LastScaled = SwapConfig.UFix64ToScaledUInt256(self.token1Vault.balance)
            
        /// Update twap at the first transaction in one block with the last block balance
        self._update(reserve0LastScaled: reserve0LastScaled, reserve1LastScaled: reserve1LastScaled)
        /// Mint fee
        let feeOn = self._mintFee(reserve0: self.token0Vault.balance, reserve1: self.token1Vault.balance)

        var liquidity = 0.0
        if (self.totalSupply == 0.0) {
            /// Add initial liquidity
            if (tokenAVault.isInstance(self.token0VaultType)) {
                self.token0Vault.deposit(from: <-tokenAVault)
                self.token1Vault.deposit(from: <-tokenBVault)
            } else {
                self.token0Vault.deposit(from: <-tokenBVault)
                self.token1Vault.deposit(from: <-tokenAVault)
            }
            /// Mint initial liquidity token and donate 1e-8 initial minimum liquidity token
            let initialLpAmount = SwapConfig.ScaledUInt256ToUFix64(
                SwapConfig.sqrt(SwapConfig.UFix64ToScaledUInt256(self.token0Vault.balance) * SwapConfig.UFix64ToScaledUInt256(self.token1Vault.balance) / SwapConfig.scaleFactor)
            )
            self.donateInitialMinimumLpToken()
            
            liquidity = initialLpAmount - SwapConfig.ufix64NonZeroMin
        } else {
            var lptokenMintAmount0Scaled: UInt256 = 0
            var lptokenMintAmount1Scaled: UInt256 = 0
            /// Use UFIx64ToUInt256 in division & multiply to solve precision issues
            let inAmountAScaled = SwapConfig.UFix64ToScaledUInt256(tokenAVault.balance)
            let inAmountBScaled = SwapConfig.UFix64ToScaledUInt256(tokenBVault.balance)

            let totalSupplyScaled = SwapConfig.UFix64ToScaledUInt256(self.totalSupply)

            if (tokenAVault.isInstance(self.token0VaultType)) {
                lptokenMintAmount0Scaled = inAmountAScaled * totalSupplyScaled / reserve0LastScaled
                lptokenMintAmount1Scaled = inAmountBScaled * totalSupplyScaled / reserve1LastScaled
                
                self.token0Vault.deposit(from: <-tokenAVault)
                self.token1Vault.deposit(from: <-tokenBVault)
            } else {
                lptokenMintAmount0Scaled = inAmountBScaled * totalSupplyScaled / reserve0LastScaled
                lptokenMintAmount1Scaled = inAmountAScaled * totalSupplyScaled / reserve1LastScaled

                self.token0Vault.deposit(from: <-tokenBVault)
                self.token1Vault.deposit(from: <-tokenAVault)
            }

            /// Note: User should add proportional liquidity as any extra is added into pool.
            let mintLptokenAmountScaled = lptokenMintAmount0Scaled < lptokenMintAmount1Scaled ? lptokenMintAmount0Scaled : lptokenMintAmount1Scaled
            
            /// Mint liquidity token pro rata
            liquidity = SwapConfig.ScaledUInt256ToUFix64(mintLptokenAmountScaled)
        }
        /// Mint lpTokens
        let lpTokenVault <-self.mintLpToken(amount: liquidity)

        if feeOn {
            self.rootKLast = SwapConfig.ScaledUInt256ToUFix64(
                SwapConfig.sqrt(SwapConfig.UFix64ToScaledUInt256(self.token0Vault.balance) * SwapConfig.UFix64ToScaledUInt256(self.token1Vault.balance) / SwapConfig.scaleFactor)
            )
        }

        self.lock = false
        return <-lpTokenVault
    }

    /// Remove Liquidity
    ///
    /// @Return: @[FungibleToken.Vault; 2]
    ///
    pub fun removeLiquidity(lpTokenVault: @FungibleToken.Vault) : @[FungibleToken.Vault] {
        pre {
            lpTokenVault.balance > 0.0:
                SwapError.ErrorEncode(
                    msg: "SwapPair: removed zero liquidity",
                    err: SwapError.ErrorCode.INVALID_PARAMETERS
                )
            lpTokenVault.isInstance(Type<@SwapPair.Vault>()):
                SwapError.ErrorEncode(
                    msg: "SwapPair: input vault type mismatch with lpTokenVault type",
                    err: SwapError.ErrorCode.MISMATCH_LPTOKEN_VAULT
                )
            self.lock == false: SwapError.ErrorEncode(msg: "SwapPair: Reentrant", err: SwapError.ErrorCode.REENTRANT)
        }
        post {
            self.lock == false: "SwapPair: unlock"
        }
        self.lock = true

        let reserve0LastScaled = SwapConfig.UFix64ToScaledUInt256(self.token0Vault.balance)
        let reserve1LastScaled = SwapConfig.UFix64ToScaledUInt256(self.token1Vault.balance)

        /// Update twap
        self._update(reserve0LastScaled: reserve0LastScaled, reserve1LastScaled: reserve1LastScaled)
        /// Mint fee
        let feeOn = self._mintFee(reserve0: self.token0Vault.balance, reserve1: self.token1Vault.balance)

        /// Use UFIx64ToUInt256 in division & multiply to solve precision issues
        let removeAmountScaled = SwapConfig.UFix64ToScaledUInt256(lpTokenVault.balance)
        let totalSupplyScaled = SwapConfig.UFix64ToScaledUInt256(self.totalSupply)
        
        let token0AmountScaled = removeAmountScaled * reserve0LastScaled / totalSupplyScaled
        let token1AmountScaled = removeAmountScaled * reserve1LastScaled / totalSupplyScaled
        let token0Amount = SwapConfig.ScaledUInt256ToUFix64(token0AmountScaled)
        let token1Amount = SwapConfig.ScaledUInt256ToUFix64(token1AmountScaled)

        let withdrawnToken0 <- self.token0Vault.withdraw(amount: token0Amount)
        let withdrawnToken1 <- self.token1Vault.withdraw(amount: token1Amount)

        /// Burn lpTokens
        self.burnLpToken(from: <- (lpTokenVault as! @SwapPair.Vault))

        if feeOn {
            self.rootKLast = SwapConfig.ScaledUInt256ToUFix64(
                SwapConfig.sqrt(SwapConfig.UFix64ToScaledUInt256(self.token0Vault.balance) * SwapConfig.UFix64ToScaledUInt256(self.token1Vault.balance) / SwapConfig.scaleFactor)
            )
        }

        self.lock = false
        return <- [<-withdrawnToken0, <-withdrawnToken1]
    }

    /// Swap
    ///
    pub fun swap(vaultIn: @FungibleToken.Vault, exactAmountOut: UFix64?): @FungibleToken.Vault {
        pre {
            vaultIn.balance > 0.0: SwapError.ErrorEncode(msg: "SwapPair: zero in balance", err: SwapError.ErrorCode.INVALID_PARAMETERS)
            vaultIn.isInstance(self.token0VaultType) || vaultIn.isInstance(self.token1VaultType):
                SwapError.ErrorEncode(
                    msg: "SwapPair: incompatible in token vault",
                    err: SwapError.ErrorCode.INVALID_PARAMETERS
                )
            self.lock == false: SwapError.ErrorEncode(msg: "SwapPair: Reentrant", err: SwapError.ErrorCode.REENTRANT)
        }
        post {
            self.lock == false: "SwapPair: unlock"
        }
        self.lock = true

        let reserve0LastScaled = SwapConfig.UFix64ToScaledUInt256(self.token0Vault.balance)
        let reserve1LastScaled = SwapConfig.UFix64ToScaledUInt256(self.token1Vault.balance)

        self._update(reserve0LastScaled: reserve0LastScaled, reserve1LastScaled: reserve1LastScaled)

        var amountOut = 0.0
        /// Calculate the swap result
        if (vaultIn.isInstance(self.token0VaultType)) {
            amountOut = SwapConfig.getAmountOut(amountIn: vaultIn.balance, reserveIn: self.token0Vault.balance, reserveOut: self.token1Vault.balance)
        } else {
            amountOut = SwapConfig.getAmountOut(amountIn: vaultIn.balance, reserveIn: self.token1Vault.balance, reserveOut: self.token0Vault.balance)
        }
        /// Check and swap exact output amount if specified in argument
        if exactAmountOut != nil {
            assert(amountOut >= exactAmountOut!, message:
                SwapError.ErrorEncode(
                    msg: "SwapPair: INSUFFICIENT_OUTPUT_AMOUNT",
                    err: SwapError.ErrorCode.INSUFFICIENT_OUTPUT_AMOUNT
                )
            )
            amountOut = exactAmountOut!
        }

        if (vaultIn.isInstance(self.token0VaultType)) {
            emit Swap(inTokenAmount: vaultIn.balance, outTokenAmount: amountOut, direction: 0)
            self.token0Vault.deposit(from: <-vaultIn)

            self.lock = false
            return <- self.token1Vault.withdraw(amount: amountOut)
        } else {
            emit Swap(inTokenAmount: vaultIn.balance, outTokenAmount: amountOut, direction: 1)
            self.token1Vault.deposit(from: <-vaultIn)

            self.lock = false
            return <- self.token0Vault.withdraw(amount: amountOut)
        }
    }

    /// Update cumulative price on the first call per block
    ///
    access(self) fun _update(reserve0LastScaled: UInt256, reserve1LastScaled: UInt256) {
        let blockTimestamp = getCurrentBlock().timestamp
        let timeElapsed = blockTimestamp - self.blockTimestampLast
        if (timeElapsed > 0.0 && reserve0LastScaled != 0 && reserve1LastScaled != 0) {
            let timeElapsedScaled = SwapConfig.UFix64ToScaledUInt256(timeElapsed)

            self.price0CumulativeLastScaled = self.price0CumulativeLastScaled +
                reserve1LastScaled * timeElapsedScaled / reserve0LastScaled

            self.price1CumulativeLastScaled = self.price1CumulativeLastScaled +
                reserve0LastScaled * timeElapsedScaled / reserve1LastScaled
        }
        self.blockTimestampLast = blockTimestamp
    }
    
    /// If feeTo is set, mint 1/6th of the growth in sqrt(k) which is only generated by trading behavior.
    /// Instead of collecting the 0.05% fee at the time of each trade, accumulated fees are collected only
    /// when liquidity is deposited or withdrawn.
    ///
    access(self) fun _mintFee(reserve0: UFix64, reserve1: UFix64): Bool {
        let rootKLast = self.rootKLast
        if SwapFactory.feeTo == nil {
            if rootKLast > 0.0 {
                self.rootKLast = 0.0
            }
            return false
        }
        if rootKLast > 0.0 {
            let rootK = SwapConfig.ScaledUInt256ToUFix64(
                SwapConfig.sqrt(SwapConfig.UFix64ToScaledUInt256(reserve0) * SwapConfig.UFix64ToScaledUInt256(reserve1) / SwapConfig.scaleFactor)
            )
            if rootK > rootKLast {
                let numerator = self.totalSupply * (rootK - rootKLast)
                let denominator = rootK * 5.0 + rootKLast
                let liquidity = numerator / denominator
                if liquidity > 0.0 {
                    let lpTokenVault <-self.mintLpToken(amount: liquidity)
                    let lpTokenCollectionCap = getAccount(SwapFactory.feeTo!).getCapability<&{SwapInterfaces.LpTokenCollectionPublic}>(SwapConfig.LpTokenCollectionPublicPath)
                    assert(lpTokenCollectionCap.check(), message:
                        SwapError.ErrorEncode(
                            msg: "SwapPair: Cannot borrow reference to LpTokenCollection resource in feeTo account",
                            err: SwapError.ErrorCode.LOST_PUBLIC_CAPABILITY
                        )
                    )
                    lpTokenCollectionCap.borrow()!.deposit(pairAddr: self.account.address, lpTokenVault: <-lpTokenVault)
                }
            }
        }
        return true
    }

    /// Public interfaces
    ///
    pub resource PairPublic: SwapInterfaces.PairPublic {
        pub fun swap(vaultIn: @FungibleToken.Vault, exactAmountOut: UFix64?): @FungibleToken.Vault {
            return <- SwapPair.swap(vaultIn: <-vaultIn, exactAmountOut: exactAmountOut)
        }

        pub fun removeLiquidity(lpTokenVault: @FungibleToken.Vault) : @[FungibleToken.Vault] {
            return <- SwapPair.removeLiquidity(lpTokenVault: <- lpTokenVault)
        }

        pub fun addLiquidity(tokenAVault: @FungibleToken.Vault, tokenBVault: @FungibleToken.Vault): @FungibleToken.Vault {
            return <- SwapPair.addLiquidity(tokenAVault: <- tokenAVault, tokenBVault: <- tokenBVault)
        }

        pub fun getAmountIn(amountOut: UFix64, tokenOutKey: String): UFix64 {
            if tokenOutKey == SwapPair.token1Key {
                return SwapConfig.getAmountIn(amountOut: amountOut, reserveIn: SwapPair.token0Vault.balance, reserveOut: SwapPair.token1Vault.balance)
            } else {
                return SwapConfig.getAmountIn(amountOut: amountOut, reserveIn: SwapPair.token1Vault.balance, reserveOut: SwapPair.token0Vault.balance)
            }
        }
        pub fun getAmountOut(amountIn: UFix64, tokenInKey: String): UFix64 {
            if tokenInKey == SwapPair.token0Key {
                return SwapConfig.getAmountOut(amountIn: amountIn, reserveIn: SwapPair.token0Vault.balance, reserveOut: SwapPair.token1Vault.balance)
            } else {
                return SwapConfig.getAmountOut(amountIn: amountIn, reserveIn: SwapPair.token1Vault.balance, reserveOut: SwapPair.token0Vault.balance)
            }
        }
        pub fun getPrice0CumulativeLastScaled(): UInt256 {
            return SwapPair.price0CumulativeLastScaled
        }
        pub fun getPrice1CumulativeLastScaled(): UInt256 {
            return SwapPair.price1CumulativeLastScaled
        }
        pub fun getBlockTimestampLast(): UFix64 {
            return SwapPair.blockTimestampLast
        }

        pub fun getPairInfo(): [AnyStruct] {
            return [
                SwapPair.token0Key,
                SwapPair.token1Key,
                SwapPair.token0Vault.balance,
                SwapPair.token1Vault.balance,
                SwapPair.account.address,
                SwapPair.totalSupply         
            ]
        }
        pub fun getLpTokenVaultType(): Type {
            return Type<@SwapPair.Vault>()
        }
    }

    init(token0Vault: @FungibleToken.Vault, token1Vault: @FungibleToken.Vault) {
        
        self.totalSupply = 0.0

        self.token0VaultType = token0Vault.getType()
        self.token1VaultType = token1Vault.getType()
        self.token0Vault <- token0Vault
        self.token1Vault <- token1Vault

        self.lock = false

        self.blockTimestampLast = getCurrentBlock().timestamp
        self.price0CumulativeLastScaled = 0
        self.price1CumulativeLastScaled = 0

        self.rootKLast = 0.0
        self._reservedFields = {}

        self.token0Key = SwapConfig.SliceTokenTypeIdentifierFromVaultType(vaultTypeIdentifier: self.token0VaultType.identifier)
        self.token1Key = SwapConfig.SliceTokenTypeIdentifierFromVaultType(vaultTypeIdentifier: self.token1VaultType.identifier)

        /// Open public interface capability
        destroy <-self.account.load<@AnyResource>(from: /storage/pair_public)
        self.account.save(<-create PairPublic(), to: /storage/pair_public)
        /// Pair interface public path: SwapConfig.PairPublicPath
        self.account.link<&{SwapInterfaces.PairPublic}>(SwapConfig.PairPublicPath, target: /storage/pair_public)                

        emit TokensInitialized(initialSupply: self.totalSupply)
    }
}