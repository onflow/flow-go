
// the smart contract for a fungible token
import TokenContract from 0x93
// the acutal token resource
import TokenContract.Tokens from 0x93


// placeholder for getting the reference to the flow token resource from
// the owning account's storage
import FlowToken from storage


// how do we call functions on a different token contract to distribute tokens?

// Should the token resource be defined in the token sale so it can have control?
// or do we have an approve function that gives the sale contract permission to send tokens?
// 
// how to store flow tokens that are used to buy ICO tokens?

contract interface DirectTokenSale {

    pub var TokenContract: ITokenContract

    pub tokensPerFLW: Int

    pub fun buyTokens(flow_token: <-FLW): <-TokenPool {
        pre {

        }
    }


}



contract JoshTokenSale: DirectTokenSale {

    pub var TokenContract: TokenContract

    pub var tokensPerFLW: Int

    constructor(tokenPrice: Int, tokencontract: TokenContract) {
        self.tokensPerFLW = tokenPrice
        self.TokenContract = tokencontract
    }

    pub fun buyTokens(flow_token: <-FLW): {
        let numJoshTokens = flow_token.balance * tokensPerFLW

        TokenContract.mint(MSG.SENDER, numJoshTokens)

    }
}