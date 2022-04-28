/**

# The router supports a chained swap trade

# Author: Increment Labs

*/
import FungibleToken from 0xf233dcee88fe0abe
import SwapFactory from 0xb063c16cac85dbd1
import SwapConfig from 0xb78ef7afa52ff906
import SwapError from 0xb78ef7afa52ff906
import SwapInterfaces from 0xb78ef7afa52ff906

pub contract SwapRouter {
    /// Perform a chained swap calculation starting with exact amountIn
    ///
    /// @Param  - amountIn:     e.g. 50.0
    /// @Param  - tokenKeyPath: e.g. [A.f8d6e0586b0a20c7.FUSD, A.f8d6e0586b0a20c7.FlowToken, A.f8d6e0586b0a20c7.USDC]
    /// @Return - [UFix64]:     e.g. [50.0, 10.0, 48.0]
    ///
    pub fun getAmountsOut(amountIn: UFix64, tokenKeyPath: [String]): [UFix64] {
        pre {
            tokenKeyPath.length >= 2: SwapError.ErrorEncode(msg: "SwapRouter: Invalid path", err: SwapError.ErrorCode.INVALID_PARAMETERS)
        }
        var amounts: [UFix64] = []
        for tokenKey in tokenKeyPath {
            amounts.append(0.0)
        }
        amounts[0] = amountIn

        var i: Int = 0
        while (i < tokenKeyPath.length-1) {
            let pairAddr = SwapFactory.getPairAddress(token0Key: tokenKeyPath[i], token1Key: tokenKeyPath[i+1]) ?? panic(
                SwapError.ErrorEncode(
                    msg: "SwapRouter: nonexistent pair ".concat(tokenKeyPath[i]).concat(" <-> ").concat(tokenKeyPath[i+1]),
                    err: SwapError.ErrorCode.NONEXISTING_SWAP_PAIR
                )
            )
            
            let pairPublicRef = getAccount(pairAddr).getCapability<&{SwapInterfaces.PairPublic}>(SwapConfig.PairPublicPath).borrow() ?? panic(
                SwapError.ErrorEncode(
                    msg: "SwapRouter: Lost SwapPair public capability",
                    err: SwapError.ErrorCode.LOST_PUBLIC_CAPABILITY
                )
            )
            /// Previous swap result will be the input of the next swap
            amounts[i+1] = pairPublicRef.getAmountOut(amountIn: amounts[i], tokenInKey: tokenKeyPath[i])

            i = i + 1
        }
        
        return amounts
    }

    /// Perform a chained swap calculation end with exact amountOut
    ///
    pub fun getAmountsIn(amountOut: UFix64, tokenKeyPath: [String]): [UFix64] {
        pre {
            tokenKeyPath.length >= 2: SwapError.ErrorEncode(msg: "SwapRouter: Invalid path", err: SwapError.ErrorCode.INVALID_PARAMETERS)
        }
        var amounts: [UFix64] = []
        for tokenKey in tokenKeyPath {
            amounts.append(0.0)
        }
        amounts[amounts.length-1] = amountOut

        var i: Int = tokenKeyPath.length-1
        while (i > 0) {
            let pairAddr = SwapFactory.getPairAddress(token0Key: tokenKeyPath[i], token1Key: tokenKeyPath[i-1]) ?? panic(
                SwapError.ErrorEncode(
                    msg: "SwapRouter: nonexistent pair ".concat(tokenKeyPath[i]).concat(" <-> ").concat(tokenKeyPath[i+1]),
                    err: SwapError.ErrorCode.NONEXISTING_SWAP_PAIR
                )
            )
            
            let pairPublicRef = getAccount(pairAddr).getCapability<&{SwapInterfaces.PairPublic}>(SwapConfig.PairPublicPath).borrow() ?? panic(
                SwapError.ErrorEncode(
                    msg: "SwapRouter: Lost SwapPair public capability",
                    err: SwapError.ErrorCode.LOST_PUBLIC_CAPABILITY
                )
            )
            /// Calculate from back to front
            amounts[i-1] = pairPublicRef.getAmountIn(amountOut: amounts[i], tokenOutKey: tokenKeyPath[i])

            i = i - 1
        }
        
        return amounts
    }

    /// SwapExactTokensForTokens
    ///
    /// Make sure the exact amountIn in swap start
    /// @Param  - exactVaultIn: Vault with exact amountIn
    /// @Param  - amountOutMin: Desired minimum amountOut to do slippage check
    /// @Param  - tokenKeyPath: Chained swap
    ///                         e.g. if swap from FUSD to USDC through FlowToken
    ///                              [A.f8d6e0586b0a20c7.FUSD, A.f8d6e0586b0a20c7.FlowToken, A.f8d6e0586b0a20c7.USDC]
    /// @Param  - deadline:     The timeout block timestamp for the transaction
    /// @Return - Vault:        outVault
    ///
    pub fun swapExactTokensForTokens(
        exactVaultIn: @FungibleToken.Vault,
        amountOutMin: UFix64,
        tokenKeyPath: [String],
        deadline: UFix64
    ): @FungibleToken.Vault {
        assert(deadline >= getCurrentBlock().timestamp, message:
            SwapError.ErrorEncode(
                msg: "SwapRouter: expired",
                err: SwapError.ErrorCode.EXPIRED
            )
        )
        let amounts = self.getAmountsOut(amountIn: exactVaultIn.balance, tokenKeyPath: tokenKeyPath)
        assert(amounts[amounts.length-1] >= amountOutMin, message:
            SwapError.ErrorEncode(
                msg: "SwapRouter: INSUFFICIENT_OUTPUT_AMOUNT",
                err: SwapError.ErrorCode.INSUFFICIENT_OUTPUT_AMOUNT
            )
        )
        return <- self.swapWithPath(vaultIn: <-exactVaultIn, tokenKeyPath: tokenKeyPath, exactAmounts: nil)
    }

    /// SwapTokensForExactTokens
    ///
    /// @Param  - vaultInMax:     Vault with enough input to swap, checks slippage
    /// @Param  - exactAmountOut: Make sure the exact amountOut in swap end
    /// @Param  - tokenKeyPath:   Chained swap
    ///                           e.g. if swap from FUSD to USDC through FlowToken
    ///                                [A.f8d6e0586b0a20c7.FUSD, A.f8d6e0586b0a20c7.FlowToken, A.f8d6e0586b0a20c7.USDC]
    /// @Param  - deadline:       The timeout block timestamp for the transaction
    /// @Return - [OutVault, RemainingInVault]
    ///
    pub fun swapTokensForExactTokens(
        vaultInMax: @FungibleToken.Vault,
        exactAmountOut: UFix64,
        tokenKeyPath: [String],
        deadline: UFix64
    ): @[FungibleToken.Vault] {
        assert(deadline >= getCurrentBlock().timestamp, message:
            SwapError.ErrorEncode(
                msg: "SwapRouter: expired",
                err: SwapError.ErrorCode.EXPIRED
            )
        )
        let amountInMax = vaultInMax.balance
        let amounts = self.getAmountsIn(amountOut: exactAmountOut, tokenKeyPath: tokenKeyPath)
        assert(amounts[0] <= amountInMax, message:
            SwapError.ErrorEncode(
                msg: "SwapRouter: EXCESSIVE_INPUT_AMOUNT",
                err: SwapError.ErrorCode.EXCESSIVE_INPUT_AMOUNT
            )
        )
        let vaultInExact <- vaultInMax.withdraw(amount: amounts[0])

        return <-[<-self.swapWithPath(vaultIn: <-vaultInExact, tokenKeyPath: tokenKeyPath, exactAmounts: amounts), <-vaultInMax]
    }

    /// SwapWithPath
    ///
    pub fun swapWithPath(vaultIn: @FungibleToken.Vault, tokenKeyPath: [String], exactAmounts: [UFix64]?): @FungibleToken.Vault {
        pre {
            tokenKeyPath.length >= 2: SwapError.ErrorEncode(msg: "Invalid path.", err: SwapError.ErrorCode.INVALID_PARAMETERS)
        }
        /// To reduce the gas cost, handle the first five swap out of the loop
        var exactAmountOut1: UFix64? = nil
        if exactAmounts != nil { exactAmountOut1 = exactAmounts![1] }
        let vaultOut1 <- self.swapWithPair(vaultIn: <- vaultIn, exactAmountOut: exactAmountOut1, token0Key: tokenKeyPath[0], token1Key: tokenKeyPath[1])
        if tokenKeyPath.length == 2 {
            return <-vaultOut1
        }

        var exactAmountOut2: UFix64? = nil
        if exactAmounts != nil { exactAmountOut2 = exactAmounts![2] }
        let vaultOut2 <- self.swapWithPair(vaultIn: <- vaultOut1, exactAmountOut: exactAmountOut2, token0Key: tokenKeyPath[1], token1Key: tokenKeyPath[2])
        if tokenKeyPath.length == 3 {
            return <-vaultOut2
        }

        var exactAmountOut3: UFix64? = nil
        if exactAmounts != nil { exactAmountOut3 = exactAmounts![3] }
        let vaultOut3 <- self.swapWithPair(vaultIn: <- vaultOut2, exactAmountOut: exactAmountOut3, token0Key: tokenKeyPath[2], token1Key: tokenKeyPath[3])
        if tokenKeyPath.length == 4 {
            return <-vaultOut3
        }

        var exactAmountOut4: UFix64? = nil
        if exactAmounts != nil { exactAmountOut4 = exactAmounts![4] }
        let vaultOut4 <- self.swapWithPair(vaultIn: <- vaultOut3, exactAmountOut: exactAmountOut4, token0Key: tokenKeyPath[3], token1Key: tokenKeyPath[4])
        if tokenKeyPath.length == 5 {
            return <-vaultOut4
        }
        /// Loop swap for any length path
        var index = 4
        var curVaultOut <- vaultOut4
        while(index < tokenKeyPath.length-1) {
            var in <- curVaultOut.withdraw(amount: curVaultOut.balance)
            
            var exactAmountOut: UFix64? = nil
            if exactAmounts != nil { exactAmountOut = exactAmounts![index+1] }
            var out <- self.swapWithPair(vaultIn: <- in, exactAmountOut: exactAmountOut, token0Key: tokenKeyPath[index], token1Key:tokenKeyPath[index+1])
            curVaultOut <-> out

            destroy out
            index = index + 1
        }
    
        return <-curVaultOut
    }

    /// SwapWithPair
    ///
    pub fun swapWithPair(
        vaultIn: @FungibleToken.Vault,
        exactAmountOut: UFix64?,
        token0Key: String,
        token1Key: String
    ): @FungibleToken.Vault {
        let pairAddr = SwapFactory.getPairAddress(token0Key: token0Key, token1Key: token1Key)!
        let pairPublicRef = getAccount(pairAddr).getCapability<&{SwapInterfaces.PairPublic}>(SwapConfig.PairPublicPath).borrow()!
        return <- pairPublicRef.swap(vaultIn: <- vaultIn, exactAmountOut: exactAmountOut)
    }
}