import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import FiatToken from 0xb19436aae4d94622

pub contract TokenLendingPlace {

    // Event emitted when the user deposits token and mint mToken
    pub event Mint(minter: Address?, kind: Type, mintAmount: UFix64, mintTokens: UFix64)

    // Event emitted when the user redeems mToken and withdraw token
    pub event Redeem(redeemer: Address?, kind: Type, redeemAmount: UFix64, redeemTokens: UFix64)

    // Event emitted when the user borrows the token
    pub event Borrow(borrower: Address?, kind: Type, borrowAmount: UFix64)

    // Event emitted when the user repays the token
    pub event RepayBorrow(payer: Address?, borrower: Address?, kind: Type, repayAmount: UFix64)

    // Event emitted when the user liquidates the token
    pub event LiquidateBorrow(liquidator: Address?, borrower: Address?, kindRepay: Type, kindSeize: Type, repayAmount: UFix64, seizeTokens: UFix64)

    // Where tokens are stored
    access(contract) let TokenVaultFlow: @FlowToken.Vault
    access(contract) let TokenVaultFiatToken: @FiatToken.Vault

    // Tokens minted in the protocol are represented as mToken, and the price of mToken will only increase
    // User will mint mToken when deposit
    pub var mFlowtokenPrice: UFix64 
    pub var mFiatTokentokenPrice: UFix64

    // User will mint mBorrowingToken when borrow
    pub var mFlowBorrowingtokenPrice: UFix64 
    pub var mFiatTokenBorrowingtokenPrice: UFix64

    // The real price of token
    pub var FlowTokenRealPrice: UFix64
    pub var FiatTokenRealPrice: UFix64
    
    // The APR of each deposit
    pub var mFlowInterestRate: UFix64
    pub var mFiatTokenInterestRate: UFix64 

    // The APR of each borrow
    pub var mFlowBorrowingInterestRate: UFix64
    pub var mFiatTokenBorrowingInterestRate: UFix64 

    // The last interest update timestamp
    pub var finalTimestamp: UFix64

    // The total amount of tokens lent in the protocol, which affect the calculation of interest
    pub var mFlowBorrowingAmountToken: UFix64
    pub var mFiatTokenBorrowingAmountToken: UFix64

    // The deposit limit of token
    pub var depositeLimitFLOWToken: UFix64
    pub var depositeLimitFiatToken: UFix64

    //The penalty of liquidation
    pub var liquidationPenalty: UFix64

    //The liquidate limit at once
    pub var liquidationLimit: UFix64

    // The parameter of protocol 
    pub var optimalUtilizationRate: UFix64
    pub var optimalBorrowApy: UFix64
    pub var loanToValueRatio: UFix64

    access(contract) var lendingCollection: @{Address: TokenLendingCollection}

    // The path of protocol
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath

    // The storage path for the admin resource
    pub let AdminStoragePath: StoragePath

    // The storage path for minters' MinterProxy
    pub let SetterProxyStoragePath: StoragePath

    // The public path for minters' MinterProxy capability
    pub let SetterProxyPublicPath: PublicPath

    // The storage path for user's certificate
    pub let CertificateStoragePath: StoragePath

    // The private path for user's certificate
    pub let CertificatePrivatePath: PrivatePath

    // The rate of borrowed FLOW
    pub fun getFlowUtilizationRate(): UFix64 {
        if (TokenLendingPlace.TokenVaultFlow.balance + TokenLendingPlace.mFlowBorrowingAmountToken * TokenLendingPlace.getmFlowBorrowingTokenPrice() != 0.0) {
            return (TokenLendingPlace.mFlowBorrowingAmountToken * TokenLendingPlace.getmFlowBorrowingTokenPrice()) / (TokenLendingPlace.TokenVaultFlow.balance + TokenLendingPlace.mFlowBorrowingAmountToken * TokenLendingPlace.getmFlowBorrowingTokenPrice())
        } else {
            return 0.0
        }
    }

    // The rate of borrowed FiatToken
    pub fun getFiatTokenUtilizationRate(): UFix64 {
        if (TokenLendingPlace.TokenVaultFiatToken.balance + TokenLendingPlace.mFiatTokenBorrowingAmountToken * TokenLendingPlace.getmFiatTokenBorrowingTokenPrice() != 0.0) {
            return (TokenLendingPlace.mFiatTokenBorrowingAmountToken * TokenLendingPlace.getmFiatTokenBorrowingTokenPrice()) / (TokenLendingPlace.TokenVaultFiatToken.balance + TokenLendingPlace.mFiatTokenBorrowingAmountToken * TokenLendingPlace.getmFiatTokenBorrowingTokenPrice())
        } else {
            return 0.0
        }
    }
    
    

    // Get mFlowBorrowingTokenPrice
    pub fun getmFlowBorrowingTokenPrice(): UFix64 {
        let delta = getCurrentBlock().timestamp - TokenLendingPlace.finalTimestamp
        return TokenLendingPlace.mFlowBorrowingtokenPrice + delta * TokenLendingPlace.mFlowBorrowingInterestRate / ( 365.0 * 24.0 * 60.0 * 60.0)
    }

    // Get mFiatTokenBorrowingTokenPrice
    pub fun getmFiatTokenBorrowingTokenPrice(): UFix64 {
        let delta = getCurrentBlock().timestamp - TokenLendingPlace.finalTimestamp
        return TokenLendingPlace.mFiatTokenBorrowingtokenPrice + delta * TokenLendingPlace.mFiatTokenBorrowingInterestRate / ( 365.0 * 24.0 * 60.0 * 60.0)
    }


    // Get mFlowTokenPrice
    pub fun getmFlowTokenPrice(): UFix64 {
        let delta = getCurrentBlock().timestamp - TokenLendingPlace.finalTimestamp
        return TokenLendingPlace.mFlowtokenPrice + delta * TokenLendingPlace.mFlowInterestRate / ( 365.0 * 24.0 * 60.0 * 60.0)
    }
    // Get mFiatTokenTokenPrice
    pub fun getmFiatTokenTokenPrice(): UFix64 {
        let delta = getCurrentBlock().timestamp - TokenLendingPlace.finalTimestamp
        return TokenLendingPlace.mFiatTokentokenPrice + delta * TokenLendingPlace.mFiatTokenInterestRate / ( 365.0 * 24.0 * 60.0 * 60.0)
    }

    // Get total supply
    pub fun getTotalsupply(): {String: UFix64} {
        return {
            "flowTotalSupply": TokenLendingPlace.TokenVaultFlow.balance + TokenLendingPlace.mFlowBorrowingAmountToken * TokenLendingPlace.getmFlowBorrowingTokenPrice(),
            "fiatTokenTotalSupply": TokenLendingPlace.TokenVaultFiatToken.balance + TokenLendingPlace.mFiatTokenBorrowingAmountToken * TokenLendingPlace.getmFiatTokenBorrowingTokenPrice()
        }
    }
    // Get deposit limit
    pub fun getDepositLimit(): {String: UFix64} {
        return {
            "flowDepositLimit":TokenLendingPlace.depositeLimitFLOWToken,
            "fiatTokenDepositLimit": TokenLendingPlace.depositeLimitFiatToken
        }
    }
    // Get total borrow
    pub fun getTotalBorrow(): {String: UFix64} {
        return {
            "flowTotalBorrow":TokenLendingPlace.mFlowBorrowingAmountToken * TokenLendingPlace.getmFlowBorrowingTokenPrice(),
            "fiatTokenTotalBorrow": TokenLendingPlace.mFiatTokenBorrowingAmountToken * TokenLendingPlace.getmFiatTokenBorrowingTokenPrice()
        }
    }
    // Get token real price
    pub fun getTokenPrice(): {String: UFix64} {
        return {
            "flowTokenPrice":TokenLendingPlace.FlowTokenRealPrice,
            "fiatTokenTokenPrice": TokenLendingPlace.FiatTokenRealPrice
        }
    }

    pub fun borrowCollection(address: Address): &TokenLendingCollection? {
            if self.lendingCollection[address] != nil {
                return &self.lendingCollection[address] as! &TokenLendingCollection
            } else {
                return nil
            }
        }

    // The method for updating mToken and interest rate in the protocol.
    // Every amount changing, such as deposite, repay, withdraw, borrow, and liquidty, will call this method,
    // which updates the latest rate immediately
    access(contract) fun updatePriceAndInterest() {

        // Update token price
        let delta = getCurrentBlock().timestamp - TokenLendingPlace.finalTimestamp
        TokenLendingPlace.mFlowtokenPrice = TokenLendingPlace.mFlowtokenPrice + (delta * TokenLendingPlace.mFlowInterestRate / ( 365.0 * 24.0 * 60.0 * 60.0))
        TokenLendingPlace.mFiatTokentokenPrice = TokenLendingPlace.mFiatTokentokenPrice + (delta * TokenLendingPlace.mFiatTokenInterestRate / ( 365.0 * 24.0 * 60.0 * 60.0))

        TokenLendingPlace.mFlowBorrowingtokenPrice = TokenLendingPlace.mFlowBorrowingtokenPrice + (delta * TokenLendingPlace.mFlowBorrowingInterestRate / ( 365.0 * 24.0 * 60.0 * 60.0))
        TokenLendingPlace.mFiatTokenBorrowingtokenPrice = TokenLendingPlace.mFiatTokenBorrowingtokenPrice + (delta * TokenLendingPlace.mFiatTokenBorrowingInterestRate / ( 365.0 * 24.0 * 60.0 * 60.0))
        TokenLendingPlace.finalTimestamp = getCurrentBlock().timestamp

        // Update interestRate
        if (TokenLendingPlace.TokenVaultFlow.balance + TokenLendingPlace.mFlowBorrowingAmountToken * TokenLendingPlace.getmFlowBorrowingTokenPrice() != 0.0) {
            if (TokenLendingPlace.getFlowUtilizationRate() < TokenLendingPlace.optimalUtilizationRate) {
                TokenLendingPlace.mFlowBorrowingInterestRate = TokenLendingPlace.getFlowUtilizationRate() / TokenLendingPlace.optimalUtilizationRate * TokenLendingPlace.optimalBorrowApy
            } else {
               TokenLendingPlace.mFlowBorrowingInterestRate = (TokenLendingPlace.getFlowUtilizationRate() - TokenLendingPlace.optimalUtilizationRate) / (1.0 - TokenLendingPlace.optimalUtilizationRate) * ( 1.0 - TokenLendingPlace.optimalBorrowApy) + TokenLendingPlace.optimalBorrowApy
            }
            TokenLendingPlace.mFlowInterestRate = TokenLendingPlace.mFlowBorrowingInterestRate * TokenLendingPlace.getFlowUtilizationRate()
        }

        if (TokenLendingPlace.TokenVaultFiatToken.balance + TokenLendingPlace.mFiatTokenBorrowingAmountToken * TokenLendingPlace.getmFiatTokenBorrowingTokenPrice() != 0.0) {
            if (TokenLendingPlace.getFiatTokenUtilizationRate() < TokenLendingPlace.optimalUtilizationRate) {
               TokenLendingPlace.mFiatTokenBorrowingInterestRate = TokenLendingPlace.getFiatTokenUtilizationRate() / TokenLendingPlace.optimalUtilizationRate*TokenLendingPlace.optimalBorrowApy
            } else {
                TokenLendingPlace.mFiatTokenBorrowingInterestRate = (TokenLendingPlace.getFiatTokenUtilizationRate() - TokenLendingPlace.optimalUtilizationRate) / (1.0 - TokenLendingPlace.optimalUtilizationRate) * (1.0 - TokenLendingPlace.optimalBorrowApy) + TokenLendingPlace.optimalBorrowApy
            }
            TokenLendingPlace.mFiatTokenInterestRate = TokenLendingPlace.mFiatTokenBorrowingInterestRate * TokenLendingPlace.getFiatTokenUtilizationRate()
        }

        
    }


    // LendingCollection
    //
    // The Token collection resource records every user data. Users join the protocol through this resource.
    //
    pub resource TokenLendingCollection {

        // User's mtoken amount, which minted when deposit
        access(self) var mFlow: UFix64
        access(self) var mFiatToken: UFix64

        // User's mBorrowingtoken amount, which minted when borrow
        access(self) var myBorrowingmFlow: UFix64
        access(self) var myBorrowingmFiatToken: UFix64
        access(self) var ownerAddress: Address
        
        init (_owner: Address) {
            self.mFlow = 0.0
            self.mFiatToken = 0.0

            self.myBorrowingmFlow = 0.0
            self.myBorrowingmFiatToken = 0.0
            self.ownerAddress = _owner
        }

        pub fun getmFlow(): UFix64 {
            return self.mFlow
        }

        pub fun getmFiatToken(): UFix64 {
            return self.mFiatToken
        }

        pub fun getMyBorrowingmFlow(): UFix64 {
            return self.myBorrowingmFlow
        }

        pub fun getMyBorrowingmFiatToken(): UFix64 {
            return self.myBorrowingmFiatToken
        }

        // User deposits the token as Liquidity and mint mtoken
        pub fun addLiquidity(from: @FungibleToken.Vault, _cer: Capability<&UserCertificate>) {
            assert(
                self.ownerAddress == _cer.borrow()!.owner!.address,
                message: "ownerAddress mismatch"
            )
            var balance = 0.0
            if (from.getType() == Type<@FlowToken.Vault>()) {
                balance = from.balance
                TokenLendingPlace.TokenVaultFlow.deposit(from: <- from)
                self.mFlow = self.mFlow + (balance / TokenLendingPlace.getmFlowTokenPrice())
                 // event
                emit Mint(
                    minter: self.ownerAddress,
                    kind: FlowToken.getType(),
                    mintAmount: balance,
                    mintTokens: balance / TokenLendingPlace.getmFlowTokenPrice()
                )
            } else {
                balance = from.balance
                TokenLendingPlace.TokenVaultFiatToken.deposit(from: <- from)
                self.mFiatToken = self.mFiatToken + (balance / TokenLendingPlace.getmFiatTokenTokenPrice())
                 // event
                emit Mint(
                    minter: self.ownerAddress,
                    kind: FlowToken.getType(),
                    mintAmount: balance,
                    mintTokens: balance / TokenLendingPlace.getmFiatTokenTokenPrice()
                )
            }

            TokenLendingPlace.updatePriceAndInterest()
            self.checkDepositValid()
        }

        // User redeems mtoken and withdraw the token
        pub fun removeLiquidity(_amount: UFix64, _token: Int, _cer: Capability<&UserCertificate>): @FungibleToken.Vault {
            assert(
                self.ownerAddress == _cer.borrow()!.owner!.address,
                message: "ownerAddress mismatch"
            )
            if (_token == 0) {
                let mFlowAmount = _amount / TokenLendingPlace.getmFlowTokenPrice()
                self.mFlow = self.mFlow - mFlowAmount
                let tokenVault <- TokenLendingPlace.TokenVaultFlow.withdraw(amount: _amount) 
                TokenLendingPlace.updatePriceAndInterest()
                self.checkBorrowValid()

                // event
                emit Redeem(
                    redeemer: self.ownerAddress,
                    kind: FlowToken.getType(),
                    redeemAmount: _amount,
                    redeemTokens: _amount / TokenLendingPlace.getmFlowTokenPrice()
                )

                return <- tokenVault

            } else {
                let mFiatTokenAmount = _amount / TokenLendingPlace.getmFiatTokenTokenPrice()
                self.mFiatToken = self.mFiatToken - mFiatTokenAmount
                let tokenVault <- TokenLendingPlace.TokenVaultFiatToken.withdraw(amount: _amount) 
                TokenLendingPlace.updatePriceAndInterest()
                self.checkBorrowValid()

                // event
                emit Redeem(
                    redeemer: self.ownerAddress,
                    kind: FiatToken.getType(),
                    redeemAmount: _amount,
                    redeemTokens: _amount / TokenLendingPlace.getmFiatTokenTokenPrice()
                )

                return <- tokenVault

            }
        }

        // Get user's net value
        pub fun getNetValue(): UFix64 {

            // to USD
            let NetValue = self.mFlow * TokenLendingPlace.getmFlowTokenPrice() * TokenLendingPlace.FlowTokenRealPrice + self.mFiatToken * TokenLendingPlace.getmFiatTokenTokenPrice() * TokenLendingPlace.FiatTokenRealPrice
            - self.myBorrowingmFlow * TokenLendingPlace.getmFlowBorrowingTokenPrice()* TokenLendingPlace.FlowTokenRealPrice - self.myBorrowingmFiatToken * TokenLendingPlace.getmFiatTokenBorrowingTokenPrice() * TokenLendingPlace.FiatTokenRealPrice 
            
            return NetValue
        }

        // Get user's total supply
        pub fun getMyTotalsupply(): UFix64 {
            
            // to USD
            let FlowPower = self.mFlow * TokenLendingPlace.getmFlowTokenPrice() * TokenLendingPlace.FlowTokenRealPrice 
            let FiatTokenPower = self.mFiatToken * TokenLendingPlace.getmFiatTokenTokenPrice() * TokenLendingPlace.FiatTokenRealPrice

            return FlowPower + FiatTokenPower
        }

        // Get user's total borrow
        pub fun getMyTotalborrow(): UFix64 {

            // to USD
            let FlowBorrow = self.myBorrowingmFlow * TokenLendingPlace.getmFlowBorrowingTokenPrice() * TokenLendingPlace.FlowTokenRealPrice 
            let FiatTokenBorrow = self.myBorrowingmFiatToken * TokenLendingPlace.getmFiatTokenBorrowingTokenPrice() * TokenLendingPlace.FiatTokenRealPrice

            return FlowBorrow + FiatTokenBorrow
        }

        // User borrows FLOW token
        pub fun borrowFlow(_amount: UFix64, _cer: Capability<&UserCertificate>): @FungibleToken.Vault {
            pre {
                TokenLendingPlace.TokenVaultFlow.balance - _amount >= 0.0: "Don't have enough FLOW to borrow"
            }
            assert(
                self.ownerAddress == _cer.borrow()!.owner!.address,
                message: "ownerAddress mismatch"
            )
            
            let AmountofmToken = _amount / TokenLendingPlace.getmFlowBorrowingTokenPrice()
            TokenLendingPlace.mFlowBorrowingAmountToken = AmountofmToken + TokenLendingPlace.mFlowBorrowingAmountToken

            self.myBorrowingmFlow = AmountofmToken + self.myBorrowingmFlow

            let tokenVault <- TokenLendingPlace.TokenVaultFlow.withdraw(amount: _amount)
            TokenLendingPlace.updatePriceAndInterest()
            self.checkBorrowValid()

            // event         
            emit Borrow(
                borrower: self.ownerAddress,
                kind: FlowToken.getType(),
                borrowAmount: _amount
            )

            return <- tokenVault
        }

        // User borrows FiatToken token
        pub fun borrowFiatToken(_amount: UFix64, _cer: Capability<&UserCertificate>): @FungibleToken.Vault {
            pre {
                TokenLendingPlace.TokenVaultFiatToken.balance - _amount >= 0.0: "Don't have enough FiatToken to borrow"
            }
            assert(
                self.ownerAddress == _cer.borrow()!.owner!.address,
                message: "ownerAddress mismatch"
            )
            
            let AmountofmToken = _amount / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice()
            TokenLendingPlace.mFiatTokenBorrowingAmountToken = AmountofmToken + TokenLendingPlace.mFiatTokenBorrowingAmountToken

            self.myBorrowingmFiatToken = AmountofmToken + self.myBorrowingmFiatToken

            let tokenVault <- TokenLendingPlace.TokenVaultFiatToken.withdraw(amount: _amount)
            TokenLendingPlace.updatePriceAndInterest()
            self.checkBorrowValid()

            // event         
            emit Borrow(
                borrower: self.ownerAddress,
                kind: FiatToken.getType(),
                borrowAmount: _amount
            )

            return <- tokenVault
        }
        
        
        // User repays FLow
        pub fun repayFlow(from: @FlowToken.Vault, _cer: Capability<&UserCertificate>) {
            pre {
                self.myBorrowingmFlow - from.balance / TokenLendingPlace.getmFlowBorrowingTokenPrice() >= 0.0: "Repay too much FLOW"
            }
            assert(
                self.ownerAddress == _cer.borrow()!.owner!.address,
                message: "ownerAddress mismatch"
            )
            TokenLendingPlace.mFlowBorrowingAmountToken = TokenLendingPlace.mFlowBorrowingAmountToken - from.balance / TokenLendingPlace.getmFlowBorrowingTokenPrice()
            self.myBorrowingmFlow = self.myBorrowingmFlow - from.balance / TokenLendingPlace.getmFlowBorrowingTokenPrice()
            
            // event
            emit RepayBorrow(
                payer: from.owner?.address,
                borrower: self.ownerAddress,
                kind: FlowToken.getType(),
                repayAmount: from.balance
            )

            TokenLendingPlace.TokenVaultFlow.deposit(from: <- from )
            TokenLendingPlace.updatePriceAndInterest()
        }

        // User repays FiatToken
        pub fun repayFiatToken(from: @FiatToken.Vault, _cer: Capability<&UserCertificate>) {
            pre {
                self.myBorrowingmFiatToken - from.balance / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice() >= 0.0: "Repay too much FiatToken"
            }
            assert(
                self.ownerAddress == _cer.borrow()!.owner!.address,
                message: "ownerAddress mismatch"
            )
            TokenLendingPlace.mFiatTokenBorrowingAmountToken = TokenLendingPlace.mFiatTokenBorrowingAmountToken - from.balance / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice()
            self.myBorrowingmFiatToken = self.myBorrowingmFiatToken - from.balance / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice()
            
            // event
            emit RepayBorrow(
                payer: from.owner?.address,
                borrower: self.ownerAddress,
                kind: FiatToken.getType(),
                repayAmount: from.balance
            )

            TokenLendingPlace.TokenVaultFiatToken.deposit(from: <- from )
            TokenLendingPlace.updatePriceAndInterest()
        }
            

        // Check if the borrowing amount over the loan limit
        pub fun checkBorrowValid() {
            if( self.getMyTotalborrow() != 0.0 ) {
                assert(
                    self.getMyTotalborrow() / self.getMyTotalsupply() < TokenLendingPlace.loanToValueRatio, 
                    message: "It's greater than loanToValueRatio"
                )
            }
        }

        // Check if the borrowing amount over the UtilizationRate
        pub fun checkLiquidValid() {
            assert(
                self.getMyTotalborrow() / self.getMyTotalsupply() > TokenLendingPlace.optimalUtilizationRate,
                message: "It's less than optimalUtilizationRate"
            )
        }

        // Check if the deposit amount over the deposit limit
        pub fun checkDepositValid() {
            assert(
                (TokenLendingPlace.TokenVaultFlow.balance + TokenLendingPlace.mFlowBorrowingAmountToken * TokenLendingPlace.getmFlowBorrowingTokenPrice()) < TokenLendingPlace.depositeLimitFLOWToken,
                message: "It's greater than depositeLimitFLOWToken"
            )
            assert(
                (TokenLendingPlace.TokenVaultFiatToken.balance + TokenLendingPlace.mFiatTokenBorrowingAmountToken * TokenLendingPlace.getmFiatTokenBorrowingTokenPrice()) < TokenLendingPlace.depositeLimitFiatToken,
                message: "It's greater than depositeLimitFiatToken"
            )

        }

        // Liquidate the user over the UtilizationRate
        pub fun liquidateFlow(from: @FungibleToken.Vault, liquidatorVault: &TokenLendingCollection) {
            self.checkLiquidValid()
            // FLOW in, FLOW out
            if (from.getType() == Type<@FlowToken.Vault>()) {

                assert(self.myBorrowingmFlow - (from.balance / TokenLendingPlace.getmFlowBorrowingTokenPrice()) >= 0.0, message: "Liquidate too much FLOW")
                assert((from.balance / TokenLendingPlace.getmFlowBorrowingTokenPrice()) / self.myBorrowingmFlow <= TokenLendingPlace.liquidationLimit, message: "Liquidate amount must less than liquidationLimit")
                self.myBorrowingmFlow = self.myBorrowingmFlow - (from.balance / TokenLendingPlace.getmFlowBorrowingTokenPrice())
                TokenLendingPlace.mFlowBorrowingAmountToken = TokenLendingPlace.mFlowBorrowingAmountToken - (from.balance / TokenLendingPlace.getmFlowBorrowingTokenPrice())
                let repaymoney = from.balance

                liquidatorVault.depositemFlow(from:(repaymoney * TokenLendingPlace.FlowTokenRealPrice / TokenLendingPlace.FlowTokenRealPrice / TokenLendingPlace.getmFlowTokenPrice() / (1.0 - TokenLendingPlace.liquidationPenalty)))
                
                // event
                emit LiquidateBorrow(
                    liquidator: from.owner?.address,
                    borrower: self.ownerAddress,
                    kindRepay: FlowToken.getType(),
                    kindSeize: FlowToken.getType(),
                    repayAmount: from.balance,
                    seizeTokens: (repaymoney * TokenLendingPlace.FlowTokenRealPrice / TokenLendingPlace.FlowTokenRealPrice / TokenLendingPlace.getmFlowTokenPrice() / (1.0 - TokenLendingPlace.liquidationPenalty))
                )

                TokenLendingPlace.TokenVaultFlow.deposit(from: <- from)

                self.mFlow = self.mFlow - (repaymoney / TokenLendingPlace.getmFlowTokenPrice())

            } else {
                // FiatToken in, FLOW out
                assert(self.myBorrowingmFiatToken - (from.balance / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice()) >= 0.0, message: "Liquidate too much FLOW")
                assert((from.balance / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice()) / self.myBorrowingmFiatToken <= TokenLendingPlace.liquidationLimit, message: "Liquidate amount must less than liquidationLimit")
                self.myBorrowingmFiatToken = self.myBorrowingmFiatToken - (from.balance / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice())
                TokenLendingPlace.mFiatTokenBorrowingAmountToken = TokenLendingPlace.mFiatTokenBorrowingAmountToken - (from.balance / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice())
                let repaymoney = from.balance

                liquidatorVault.depositemFlow(from:(repaymoney * TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.FlowTokenRealPrice / TokenLendingPlace.getmFlowTokenPrice() / (1.0 - TokenLendingPlace.liquidationPenalty)))
                
                // event
                emit LiquidateBorrow(
                    liquidator: from.owner?.address,
                    borrower: self.ownerAddress,
                    kindRepay: FiatToken.getType(),
                    kindSeize: FlowToken.getType(),
                    repayAmount: from.balance,
                    seizeTokens: (repaymoney * TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.FlowTokenRealPrice / TokenLendingPlace.getmFlowTokenPrice())
                )

                TokenLendingPlace.TokenVaultFiatToken.deposit(from: <- from)

                self.mFlow = self.mFlow - (repaymoney * TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.FlowTokenRealPrice / TokenLendingPlace.getmFlowTokenPrice() / (1.0 - TokenLendingPlace.liquidationPenalty))
            }
            TokenLendingPlace.updatePriceAndInterest()
        }

        pub fun liquidateFiatToken(from: @FungibleToken.Vault, liquidatorVault: &TokenLendingCollection) {
            self.checkLiquidValid()
            // FLOW in, FiatToken out
            if (from.getType() == Type<@FlowToken.Vault>()) {
                assert(self.myBorrowingmFlow - (from.balance / TokenLendingPlace.getmFlowBorrowingTokenPrice()) >= 0.0, message: "Liquidate too much FiatToken")
                assert((from.balance / TokenLendingPlace.getmFlowBorrowingTokenPrice()) / self.myBorrowingmFlow <= TokenLendingPlace.liquidationLimit, message: "Liquidate amount must less than liquidationLimit")
                self.myBorrowingmFlow = self.myBorrowingmFlow - (from.balance / TokenLendingPlace.getmFlowBorrowingTokenPrice())
                TokenLendingPlace.mFlowBorrowingAmountToken = TokenLendingPlace.mFlowBorrowingAmountToken - (from.balance / TokenLendingPlace.getmFlowBorrowingTokenPrice())
                let repaymoney = from.balance

                liquidatorVault.depositemFiatToken(from:(repaymoney * TokenLendingPlace.FlowTokenRealPrice / TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.getmFiatTokenTokenPrice() / (1.0 - TokenLendingPlace.liquidationPenalty)))
                
                // event
                emit LiquidateBorrow(
                    liquidator: from.owner?.address,
                    borrower: self.ownerAddress,
                    kindRepay: FlowToken.getType(),
                    kindSeize: FiatToken.getType(),
                    repayAmount: from.balance,
                    seizeTokens: (repaymoney * TokenLendingPlace.FlowTokenRealPrice / TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.getmFiatTokenTokenPrice() / (1.0 - TokenLendingPlace.liquidationPenalty))
                )

                TokenLendingPlace.TokenVaultFlow.deposit(from: <- from)

                self.mFiatToken = self.mFiatToken - (repaymoney * TokenLendingPlace.FlowTokenRealPrice / TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.getmFiatTokenTokenPrice())
            } else {
                // FiatToken in, FiatToken out
                assert(self.myBorrowingmFiatToken - (from.balance / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice()) >= 0.0, message: "Liquidate too much FiatToken")
                assert((from.balance / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice()) / self.myBorrowingmFiatToken <= TokenLendingPlace.liquidationLimit, message: "Liquidate amount must less than liquidationLimit")
                self.myBorrowingmFiatToken = self.myBorrowingmFiatToken - (from.balance / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice())
                TokenLendingPlace.mFiatTokenBorrowingAmountToken = TokenLendingPlace.mFiatTokenBorrowingAmountToken - (from.balance / TokenLendingPlace.getmFiatTokenBorrowingTokenPrice())
                let repaymoney = from.balance

                liquidatorVault.depositemFiatToken(from:(repaymoney * TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.getmFiatTokenTokenPrice() / (1.0 - TokenLendingPlace.liquidationPenalty)))
                
                // event
                emit LiquidateBorrow(
                    liquidator: from.owner?.address,
                    borrower: self.ownerAddress,
                    kindRepay: FiatToken.getType(),
                    kindSeize: FiatToken.getType(),
                    repayAmount: from.balance,
                    seizeTokens:(repaymoney * TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.getmFiatTokenTokenPrice())
                )

                TokenLendingPlace.TokenVaultFiatToken.deposit(from: <- from)

                self.mFiatToken = self.mFiatToken - (repaymoney * TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.FiatTokenRealPrice / TokenLendingPlace.getmFiatTokenTokenPrice() / (1.0 - TokenLendingPlace.liquidationPenalty))
            }
            TokenLendingPlace.updatePriceAndInterest()
        }

        access(self) fun depositemFlow(from: UFix64) {
            self.mFlow = self.mFlow + from
        }

        access(self) fun depositemFiatToken(from: UFix64) {
            self.mFiatToken = self.mFiatToken + from
        }
        
    }

    pub resource UserCertificate {};
    pub fun createCertificate(): @UserCertificate {
        return <- create UserCertificate()
    };

    // createCollection returns a new collection resource to the caller
    pub fun createTokenLendingCollection(_cer: Capability<&UserCertificate>) {
        TokenLendingPlace.lendingCollection[_cer.borrow()!.owner!.address] <-! create TokenLendingCollection( _owner : _cer.borrow()!.owner!.address)
    }

    pub resource Administrator {

        pub fun createNewSetter(): @Setter {
            return <- create Setter()
        }

    }

    pub resource Setter {

        pub fun updatePricefromOracle(_FlowPrice: UFix64, _FiatTokenPrice: UFix64){
            TokenLendingPlace.FlowTokenRealPrice = _FlowPrice
            TokenLendingPlace.FiatTokenRealPrice = _FiatTokenPrice
        }
        pub fun updateDepositLimit(_FlowLimit: UFix64, _FiatTokenLimit: UFix64){
            TokenLendingPlace.depositeLimitFLOWToken = _FlowLimit
            TokenLendingPlace.depositeLimitFiatToken = _FiatTokenLimit
        }

    }
    pub resource interface SetterProxyPublic {
        pub fun setSetterCapability(cap: Capability<&Setter>)
    }

    pub resource SetterProxy: SetterProxyPublic {

        // access(self) so nobody else can copy the capability and use it.
        access(self) var SetterCapability: Capability<&Setter>?

        // Anyone can call this, but only the admin can create Setter capabilities,
        // so the type system constrains this to being called by the admin.
        pub fun setSetterCapability(cap: Capability<&Setter>) {
            self.SetterCapability = cap
        }

        pub fun updatePricefromOracle(_FlowPrice: UFix64, _FiatTokenPrice: UFix64){
            self.SetterCapability!
            .borrow()!.updatePricefromOracle(_FlowPrice: _FlowPrice, _FiatTokenPrice:_FiatTokenPrice)
        }
        pub fun updateDepositLimit(_FlowLimit: UFix64, _FiatTokenLimit: UFix64){
            self.SetterCapability!
            .borrow()!.updateDepositLimit(_FlowLimit: _FlowLimit, _FiatTokenLimit: _FiatTokenLimit)
        }

        init() {
            self.SetterCapability = nil
        }

    }

    pub fun createSetterProxy(): @SetterProxy {
        return <- create SetterProxy()
    }

    init() {
        self.TokenVaultFlow <- FlowToken.createEmptyVault() as! @FlowToken.Vault
        self.TokenVaultFiatToken <- FiatToken.createEmptyVault()

        self.lendingCollection <- {}

        self.mFlowInterestRate = 0.0
        self.mFiatTokenInterestRate = 0.0
        self.mFlowBorrowingInterestRate = 0.0
        self.mFiatTokenBorrowingInterestRate = 0.0
        self.mFlowtokenPrice = 1.0
        self.mFiatTokentokenPrice = 1.0
        self.mFlowBorrowingtokenPrice = 1.0
        self.mFiatTokenBorrowingtokenPrice = 1.0

        self.FlowTokenRealPrice = 10.0
        self.FiatTokenRealPrice = 1.0
        self.finalTimestamp = 0.0 // getCurrentBlock().height

        self.mFlowBorrowingAmountToken = 0.0
        self.mFiatTokenBorrowingAmountToken = 0.0

        self.depositeLimitFLOWToken = 10000.0
        self.depositeLimitFiatToken = 50000.0

        self.liquidationPenalty  = 0.05

        self.liquidationLimit = 0.5

        self.optimalUtilizationRate = 0.8
        self.optimalBorrowApy = 0.08
        self.loanToValueRatio = 0.7
        self.CollectionStoragePath = /storage/TokenLendingPlace001
        self.CollectionPublicPath = /public/TokenLendingPlace001

        self.AdminStoragePath = /storage/TokenLendingPlaceAdmin
        self.SetterProxyPublicPath = /public/TokenLendingPlaceMinterProxy001
        self.SetterProxyStoragePath = /storage/TokenLendingPlaceMinterProxy001

        self.CertificateStoragePath = /storage/TokenLendingUserCertificate001;
        self.CertificatePrivatePath = /private/TokenLendingUserCertificate001;

        let admin <- create Administrator()
        self.account.save(<-admin, to: self.AdminStoragePath)
  }
}