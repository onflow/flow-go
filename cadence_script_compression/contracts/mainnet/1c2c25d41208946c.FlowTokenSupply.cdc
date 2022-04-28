/*
    FlowTokenSupply contract provides a function (getFlowTokenSupply) that can calculate the real FLOW token
    supply by deducting the tokens that are planned for burning.
 */

import FlowToken from 0x1654653399040a61
import FungibleToken from 0xf233dcee88fe0abe

pub contract FlowTokenSupply {    
    
    pub fun getFlowTokenSupply(): UFix64 {
        
        // list of accounts that contain tokens which will be burned
        let bonusAccounts: [Address] = [
            0x1e4141df8dd1b9e1,
            0x119b2c599e83aff4,
            0x2cb528af2b0371c4,
            0x156b73464154c7c8,
            0x033f1675d850cda9,
            0x7f57d2ac0891612f,
            0x95466e57bfafc9fc,
            0x63c3ea9f3b192653,
            0x87a453422ae3268c,
            0xd3dfd30730446b72,
            0xc34469bbf05df448,
            0x9e6c5cce732ab7d5,
            0x883839fdea2ebdb4,
            0xffb3e14e7a6aeba4,
            0xe790c694e4a1e2d1,
            0x03f77f49f55be20e,
            0xecbaf40d28249cf8,
            0xe92fed41ce65ce62,
            0x08dd4dd039de9c27,
            0x1e8928e3a0da9646,
            0xfaee913eb1209699,
            0x0d48549cdf9fcebd,
            0x2723d066e1a7f497,
            0x94652f3d55a67e77,
            0x747de035c4141f06,
            0x700296e0445c7ea8,
            0x081524ec14d5b380,
            0x046fdaddfb0f9405,
            0x1b1c31af469bc4dc,
            0xfa26f8029c2bb93e,
            0xf10cca9b50aec717,
            0xe758afa8c9aacd76,
            0xec729d31052fb35f,
            0x657610ecc8758351,
            0x9745cc02408b89ef,
            0x9ea5723dff2a336a,
            0x45a96a7a77eb20bb,
            0x2118dc0d7a357319,
            0xd32b00e3f2cb79a7,
            0xadb5f244beaa2cff,
            0xbbe1977727ae269e,
            0x3b420b8a3c74213f,
            0x53fd0f49eeef2ada,
            0xb79ab694ff152a05,
            0xc971d764b48a2b81,
            0x64da97a566569657,
            0x5f862eaa36542641,
            0x89ef336e54c298ec,
            0x88f1170e662e390b,
            0x187b7d8f3a97be6e,
            0x3177b55578a3fef6,
            0xc0ea7c9c8d8e215c,
            0x75978faca21d2c32,
            0x958f40a433af4d43,
            0x622985065d101567,
            0xea48a161b269b4d0,
            0xb0cba5eeeb2b58b7,
            0x7a0be51362d0b70a,
            0xfc1cc4522b6dbeb1,
            0x4b1eb4331db2783a,
            0x3827ed8f717026b3,
            0x04399e054862479c,
            0x1b45c3dc587973c0,
            0x444df4871d66dc12,
            0x416a4ee267e83a69,
            0xe3d2a2bf31d3be0a
        ]

        // sum the balance of above accounts
        var bonusAmount: UFix64 = 0.0
        for accountAddress in bonusAccounts {
            let account = getAccount(accountAddress)
            let vaultRef = account.getCapability(/public/flowTokenBalance)
                .borrow<&FlowToken.Vault{FungibleToken.Balance}>()
                ?? panic("Could not borrow Balance reference to the Vault")
            bonusAmount = bonusAmount + vaultRef.balance
        }

        // amount of FLOW tokens that will be burned from the Dapper Genesis account
        let dapperGenesisBurnAmount: UFix64 = 28046030.0

        return FlowToken.totalSupply - bonusAmount - dapperGenesisBurnAmount
    }

}