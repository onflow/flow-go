/**

# Common swap config

# Author: Increment Labs

*/
pub contract SwapConfig {
    pub var PairPublicPath: PublicPath
    pub var LpTokenCollectionStoragePath: StoragePath
    pub var LpTokenCollectionPublicPath: PublicPath

    /// Scale factor applied to fixed point number calculation.
    /// Note: The use of scale factor is due to fixed point number in cadence is only precise to 1e-8:
    /// https://docs.onflow.org/cadence/language/values-and-types/#fixed-point-numbers
    pub let scaleFactor: UInt256
    /// 100_000_000.0, i.e. 1.0e8
    pub let ufixScale: UFix64
    /// 0.00000001, i.e. 1e-8
    pub let ufix64NonZeroMin: UFix64
    /// Reserved parameter fields: {ParamName: Value}
    access(self) let _reservedFields: {String: AnyStruct}


    /// Utility function to convert a UFix64 number to its scaled equivalent in UInt256 format
    /// e.g. 184467440737.09551615 (UFix64.max) => 184467440737095516150000000000
    ///
    pub fun UFix64ToScaledUInt256(_ f: UFix64): UInt256 {
        let integral = UInt256(f)
        let fractional = f % 1.0
        let ufixScaledInteger =  integral * UInt256(self.ufixScale) + UInt256(fractional * self.ufixScale)
        return ufixScaledInteger * self.scaleFactor / UInt256(self.ufixScale)
    }
    
    /// Utility function to convert a fixed point number in form of scaled UInt256 back to UFix64 format
    /// e.g. 184467440737095516150000000000 => 184467440737.09551615
    ///
    pub fun ScaledUInt256ToUFix64(_ scaled: UInt256): UFix64 {
        let integral = scaled / self.scaleFactor
        let ufixScaledFractional = (scaled % self.scaleFactor) * UInt256(self.ufixScale) / self.scaleFactor
        return UFix64(integral) + (UFix64(ufixScaledFractional) / self.ufixScale)
    }
    
    /// SliceTokenTypeIdentifierFromVaultType
    ///
    /// @Param vaultTypeIdentifier - eg. A.f8d6e0586b0a20c7.FlowToken.Vault
    /// @Return tokenTypeIdentifier - eg. A.f8d6e0586b0a20c7.FlowToken
    ///
    pub fun SliceTokenTypeIdentifierFromVaultType(vaultTypeIdentifier: String): String {
        return vaultTypeIdentifier.slice(
            from: 0,
            upTo: vaultTypeIdentifier.length - 6
        )
    }

    /// Helper function:
    /// Compute √x using Newton's method.
    /// @Param - x: Scaled UFix64 number in cadence. e.g. UFix64ToScaledUInt256( 16.0 )
    ///
    pub fun sqrt(_ x: UInt256): UInt256 {
        var res: UInt256 = 0
        var one: UInt256 = self.scaleFactor
        if (x > 0) {
            var x0 = x
            var mid = (x + one) / 2
            while ((x0 > mid + 1) || (mid > x0 + 1)) {
                x0 = mid
                mid = (x0 + x * self.scaleFactor / x0) / 2
            }
            res = mid
        } else {
            res = 0
        }        
        return res
    }

    /// Helper function:
    /// Given pair reserves and the exact input amount of an asset, returns the maximum output amount of the other asset
    ///
    pub fun getAmountOut(amountIn: UFix64, reserveIn: UFix64, reserveOut: UFix64): UFix64 {
        pre {
            amountIn > 0.0: "SwapPair: insufficient input amount"
            reserveIn > 0.0 && reserveOut > 0.0: "SwapPair: insufficient liquidity"
        }
        let amountInScaled = SwapConfig.UFix64ToScaledUInt256(amountIn)
        let reserveInScaled = SwapConfig.UFix64ToScaledUInt256(reserveIn)
        let reserveOutScaled = SwapConfig.UFix64ToScaledUInt256(reserveOut)

        let amountInWithFeeScaled = SwapConfig.UFix64ToScaledUInt256(0.997) * amountInScaled / SwapConfig.scaleFactor

        let amountOutScaled = amountInWithFeeScaled * reserveOutScaled / (reserveInScaled + amountInWithFeeScaled)
        return SwapConfig.ScaledUInt256ToUFix64(amountOutScaled)
    }

    /// Helper function:
    /// Given pair reserves and the exact output amount of an asset wanted, returns the required (minimum) input amount of the other asset
    ///
    pub fun getAmountIn(amountOut: UFix64, reserveIn: UFix64, reserveOut: UFix64): UFix64 {
        pre {
            amountOut < reserveOut: "SwapPair: insufficient output amount"
            reserveIn > 0.0 && reserveOut > 0.0: "SwapPair: insufficient liquidity"
        }
        let amountOutScaled = SwapConfig.UFix64ToScaledUInt256(amountOut)
        let reserveInScaled = SwapConfig.UFix64ToScaledUInt256(reserveIn)
        let reserveOutScaled = SwapConfig.UFix64ToScaledUInt256(reserveOut)

        let amountInScaled = amountOutScaled * reserveInScaled / (reserveOutScaled - amountOutScaled) * SwapConfig.scaleFactor / SwapConfig.UFix64ToScaledUInt256(0.997)
        return SwapConfig.ScaledUInt256ToUFix64(amountInScaled) + SwapConfig.ufix64NonZeroMin
    }

    /// Helper function:
    /// Given some amount of an asset and pair reserves, returns an equivalent amount of the other asset
    ///
    pub fun quote(amountA: UFix64, reserveA: UFix64, reserveB: UFix64): UFix64 {
        pre {
            amountA > 0.0: "SwapPair: insufficient input amount"
            reserveB > 0.0 && reserveB > 0.0: "SwapPair: insufficient liquidity"
        }
        let amountAScaled = SwapConfig.UFix64ToScaledUInt256(amountA)
        let reserveAScaled = SwapConfig.UFix64ToScaledUInt256(reserveA)
        let reserveBScaled = SwapConfig.UFix64ToScaledUInt256(reserveB)

        var amountBScaled = amountAScaled * reserveBScaled / reserveAScaled
        return SwapConfig.ScaledUInt256ToUFix64(amountBScaled)
    }


    init() {
        self.PairPublicPath = /public/increment_swap_pair
        self.LpTokenCollectionStoragePath = /storage/increment_swap_lptoken_collection
        self.LpTokenCollectionPublicPath  = /public/increment_swap_lptoken_collection
        
        /// 1e18
        self.scaleFactor = 1_000_000_000_000_000_000
        /// 1.0e8
        self.ufixScale = 100_000_000.0
        /// 1.0e-8
        self.ufix64NonZeroMin = 0.00000001

        self._reservedFields = {}
    }
}