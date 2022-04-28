import FungibleToken from 0xf233dcee88fe0abe
import NyatheesOVO from 0x75e0b6de94eb05d0
import NonFungibleToken from 0x1d7e57aa55817448

pub contract OVOMarketPlace{

    pub enum orderStatues: UInt8{
        pub case onSell
        pub case sold
        pub case canceled
    }
    pub struct orderData{
        // order Id
        pub let orderId: UInt64;
        // order statue
        pub var orderStatue: orderStatues;
        // tokenId of in this order
        pub let tokenId: UInt64;
        // seller address
        pub let sellerAddr: Address;
        // buyer address
        pub var buyerAddr: Address?;
        // token name
        pub let tokenName: String;
        // total price
        pub let totalPrice: UFix64;
        // create time of this order
        pub let createTime: UFix64;
        // sold time of this order
        pub let soldTime: UFix64;

        init(orderId: UInt64, orderStatue: orderStatues, tokenId: UInt64, sellerAddr: Address, 
            buyerAddr: Address?, tokenName: String, totalPrice: UFix64, createTime: UFix64
            soldTime: UFix64){
                self.orderId = orderId;
                self.orderStatue = orderStatue;
                self.tokenId = tokenId;
                self.sellerAddr = sellerAddr;
                self.buyerAddr = buyerAddr;
                self.tokenName = tokenName;
                self.totalPrice = totalPrice;
                self.createTime = createTime;
                self.soldTime = soldTime;
        }
    }

    pub event SellNFT(sellerAddr: Address, orderId: UInt64, tokenId: UInt64, totalPrice: UFix64, 
                     buyerFee: UFix64, sellerFee: UFix64, tokenName: String, createTime: UFix64)
	pub event BuyNFT(buyerAddr: Address, orderId: UInt64, tokenId: UInt64, totalPrice: UFix64,
                    buyerFee: UFix64, sellerFee: UFix64, createTime: UFix64, 
                    soldTime: UFix64)
	pub event CancelOrder(orderId: UInt64, sellerAddr: Address, cancelTime: UFix64)
    pub event MarketControllerCreated()


    //path
    pub let MarketPublicPath: PublicPath
    pub let MarketControllerPrivatePath: PrivatePath
    pub let MarketControllerStoragePath: StoragePath

    // public functions to all users to buy and sell NFTs in this market
    pub resource interface MarketPublic {
        // @dev
        // orderId: get it from orderList
        // buyerAddr: the one who want to buy NFT
        // buyerTokenVault: the vault from the buyer to buy NFTs
        // this function will transfer NFT from onSellNFTList to the buyer
        // transfer fees to admin, deposit price to seller and set order statue to sold
        pub fun buyNFT(orderId: UInt64, buyerAddr: Address, tokenName: String, buyerTokenVault: @FungibleToken.Vault);

        // @dev
        // sellerAddr: as it means
        // tokenName: to support other tokens, you shuould pass tokenName like "FUSD"
        // totalPrice: total price of this NFT, which contains buyer fee
        // sellerNFT: the NFT seller want to sell
        // this function will create an order, move NFT resource to onSellNFTList
        // and set order statue to onSell
        pub fun sellNFT(sellerAddr: Address, tokenName: String, totalPrice: UFix64, tokenId: UInt64,
                                    sellerNFTProvider: &NyatheesOVO.Collection{NonFungibleToken.Provider, NyatheesOVO.NFTCollectionPublic});

        // @dev
        // orderId: the order to cancel
        // sellerAddr: only the seller can cancel it`s own order
        // this will set order statue to canceled and return the NFT to the seller
        pub fun cancelOrder(orderId: UInt64, sellerAddr: Address);

        // @dev
        // this function will return order list
        pub fun getMarketOrderList(): [orderData];

        // @dev
        // this function will return an order data
        pub fun getMarketOrder(orderId: UInt64): orderData?

        // @dev
        // this function will return transaction fee
        pub fun getTransactionFee(tokenName: String): UFix64?
    }

    // private functions for admin to set some args
    pub resource interface MarketController {
        pub fun setTransactionFee(tokenName: String, fee_percentage: UFix64);
        pub fun setTransactionFeeReceiver(receiverAddr: Address);
        pub fun getTransactionFeeReceiver(): Address?;
    }

    pub resource Market: MarketController, MarketPublic {

        // save on sell NFTs
        access(self) var onSellNFTList:@{UInt64: NonFungibleToken.NFT};
        // order list
        access(self) var orderList: {UInt64: orderData};
        // fee list, like "FUSD":0.05
        // 0.05 means 5%
        access(self) var transactionFeeList: {String: UFix64}
        // record current orderId
        // it will increase 1 when a new order is created
        access(self) var orderId: UInt64;
        // fees receiver
        access(self) var transactionFeeReceiverAddr: Address?;

        destroy() {
			destroy self.onSellNFTList;
		 }
        // public functions
        pub fun buyNFT(orderId: UInt64, buyerAddr: Address, tokenName: String, buyerTokenVault: @FungibleToken.Vault) {
            pre {
                buyerAddr != nil : "Buyer Address Can not be nil"
                orderId >= 0 : "Wrong Token Id"
                self.orderList[orderId] != nil : "Order not exist"
                self.onSellNFTList[orderId] != nil : "Order was canceled or sold"
                self.transactionFeeList["FUSD_SellerFee"] != nil : "Seller Fee not set"
                self.transactionFeeList["FUSD_BuyerFee"] != nil : "buyer Fee not set"
            }

            // get order data
            // and it should exist
            var orderData = self.orderList[orderId]
            if (orderData!.orderStatue != orderStatues.onSell){
                panic("Unable to buy the order which was sold or canceled")
            }
            
            // get transaction fee for seller and buyer
            var sellerFeePersentage = self.transactionFeeList[tokenName.concat("_SellerFee")]!
            var buyerFeePersentage = self.transactionFeeList[tokenName.concat("_BuyerFee")]!
            if (sellerFeePersentage == nil || buyerFeePersentage == nil){
                panic("Fees not found")
            }

            var sellerFUSDReceiver = getAccount(orderData!.sellerAddr)
                        .getCapability(/public/fusdReceiver)
                        .borrow<&{FungibleToken.Receiver}>()
                        ?? panic("Unable to borrow seller receiver reference")

            var feeReceiverFUSDReceiver = getAccount(self.transactionFeeReceiverAddr!)
                        .getCapability(/public/fusdReceiver)
                        .borrow<&{FungibleToken.Receiver}>()
                        ?? panic("Unable to borrow fee receiver reference")

            var totalPrice = orderData!.totalPrice
            if (totalPrice == nil || totalPrice <= 0.0){
                panic("Wrong total price")
            }

            // balance of buyer token vault should > total price
            // if we have buyer fee
            var buyerFee = totalPrice * (buyerFeePersentage * 100000000.0) / 100000000.0
            var sellerFee = totalPrice * (sellerFeePersentage * 100000000.0) / 100000000.0
            
            // deposit buyer fee if exist
            if (buyerFeePersentage > 0.0){
                feeReceiverFUSDReceiver.deposit(from: <-buyerTokenVault.withdraw(amount: buyerFee))
            }

            // total price should >= buyer vault balance
            // after deposit fees
            if (totalPrice > buyerTokenVault.balance){
                panic("Please provide enough money")
            }

            // deposit seller fee
            feeReceiverFUSDReceiver.deposit(from: <-buyerTokenVault.withdraw(amount: sellerFee))
            // deposit valut to seller
            sellerFUSDReceiver.deposit(from: <-buyerTokenVault)

            var buyerNFTCap = getAccount(buyerAddr).getCapability(NyatheesOVO.CollectionPublicPath).borrow<&{NyatheesOVO.NFTCollectionPublic}>() 
                                                        ?? panic("Unable to borrow NyatheesOVO Collection of the seller")
            // deposit NFT to buyer
            buyerNFTCap.deposit(token: <-self.onSellNFTList.remove(key: orderId)!)
            // update order info
            self.orderList[orderId] = OVOMarketPlace.orderData(orderId: orderData!.orderId, orderStatue: orderStatues.sold,
                                                                tokenId: orderData!.tokenId, sellerAddr: orderData!.sellerAddr, buyerAddr: buyerAddr,
                                                                tokenName: orderData!.tokenName, totalPrice: totalPrice,
                                                                createTime: orderData!.createTime, soldTime: getCurrentBlock().timestamp)
            
            emit BuyNFT(buyerAddr: buyerAddr, orderId: orderId, tokenId: orderData!.tokenId, 
                        totalPrice: totalPrice, buyerFee: buyerFee, sellerFee: sellerFee,
                        createTime: orderData!.createTime, soldTime: getCurrentBlock().timestamp)
            
        }

        pub fun sellNFT(sellerAddr: Address, tokenName: String, totalPrice: UFix64, tokenId: UInt64, 
                    sellerNFTProvider: &NyatheesOVO.Collection{NonFungibleToken.Provider, NyatheesOVO.NFTCollectionPublic}) {
            pre {
                tokenName != "" : "Token Name Can Not Be \"\" "
                totalPrice > 0.0 : "Total Price should > 0.0"
                sellerNFTProvider != nil : "NFT Provider can not be nil"
                tokenId >= 0 : "Wrong Token Id"
                self.transactionFeeList["FUSD_SellerFee"] != nil : "Seller Fee not set"
                self.transactionFeeList["FUSD_BuyerFee"] != nil : "buyer Fee not set"
            }

            self.orderList.insert(key: self.orderId, orderData(orderId: self.orderId, orderStatue: orderStatues.onSell,
                                                                tokenId: tokenId, sellerAddr: sellerAddr, buyerAddr: nil,
                                                                tokenName: tokenName, totalPrice: totalPrice,
                                                                createTime: getCurrentBlock().timestamp, soldTime: 0.0))
            if (!sellerNFTProvider.idExists(id: tokenId)){
			    panic("The NFT not belongs to you")
		    }
            
            // check metadata
            // user can not sell NFT which has sign = 1
            var metadata = sellerNFTProvider.borrowNFTItem(id: tokenId)!.getMetadata()
            if (metadata != nil && metadata["sign"] != nil && metadata["sign"] == "1"){
                panic("You can not sell this NFT")
            }

            // get transaction fee for seller and buyer
            var sellerFeePersentage = self.transactionFeeList[tokenName.concat("_SellerFee")]!
            var buyerFeePersentage = self.transactionFeeList[tokenName.concat("_BuyerFee")]!
            if (sellerFeePersentage == nil || buyerFeePersentage == nil){
                panic("Fees not found")
            }
            
            self.onSellNFTList[self.orderId] <-!sellerNFTProvider.withdraw(withdrawID: tokenId)


            emit SellNFT(sellerAddr: sellerAddr, orderId: self.orderId, tokenId: tokenId, 
                         totalPrice: totalPrice, buyerFee: buyerFeePersentage, sellerFee: sellerFeePersentage,
                          tokenName: tokenName, createTime: getCurrentBlock().timestamp)

            self.orderId = self.orderId + 1

        }

        pub fun cancelOrder(orderId: UInt64, sellerAddr: Address) {
            pre {
                sellerAddr != nil : "Seller Address Can not be nil"
                orderId >= 0 : "Wrong Token Id"
                self.orderList[orderId] != nil : "Order not exist"
                self.onSellNFTList[orderId] != nil : "Order was canceled or sold"
            }

            var orderData = self.orderList[orderId]
            if (orderData!.orderStatue != orderStatues.onSell){
                panic("Unable to cancel the order which was sold or canceled!")
            }

            if (orderData!.sellerAddr != sellerAddr){
                panic("Unable to cancel the order which not belongs to you!")
            } 

            var tokenId = orderData!.tokenId

            var sellerNFTCap = getAccount(sellerAddr).getCapability(NyatheesOVO.CollectionPublicPath).borrow<&{NyatheesOVO.NFTCollectionPublic}>() 
                                                        ?? panic("Unable to borrow NyatheesOVO Collection of the seller!")
            sellerNFTCap.deposit(token: <-self.onSellNFTList.remove(key: orderId)!)

            self.orderList[orderId] = OVOMarketPlace.orderData(orderId: orderData!.orderId, orderStatue: orderStatues.canceled,
                                                                tokenId: tokenId, sellerAddr: sellerAddr, buyerAddr: nil,
                                                                tokenName: orderData!.tokenName, totalPrice: orderData!.totalPrice,
                                                                createTime: orderData!.createTime, soldTime: getCurrentBlock().timestamp)
            emit CancelOrder(orderId: orderId, sellerAddr: sellerAddr, cancelTime: getCurrentBlock().timestamp)
        }

        pub fun getMarketOrderList(): [orderData] {
            return self.orderList.values;
        }

        pub fun getMarketOrder(orderId: UInt64): orderData?{
            return self.orderList[orderId]
        }

        pub fun getTransactionFee(tokenName: String): UFix64?{
            return self.transactionFeeList[tokenName]
        }

        // private functions
        pub fun setTransactionFee(tokenName: String, fee_percentage: UFix64) {
            self.transactionFeeList[tokenName] = fee_percentage;
        }

        pub fun setTransactionFeeReceiver(receiverAddr: Address) {
            self.transactionFeeReceiverAddr = receiverAddr;
        }

        pub fun getTransactionFeeReceiver(): Address? {
            return self.transactionFeeReceiverAddr
        }

        init(){
            self.onSellNFTList <- {};
            self.orderList = {};
            self.transactionFeeList = {};
            self.orderId = 0;
            self.transactionFeeReceiverAddr = nil;
        }
    }

    init(){
        self.MarketPublicPath = /public/MarketPublic;
        self.MarketControllerPrivatePath = /private/MarketControllerPrivate;
        self.MarketControllerStoragePath = /storage/MarketControllerStorage;
        let market <- create Market();
        self.account.save(<-market, to: self.MarketControllerStoragePath)
        self.account.link<&OVOMarketPlace.Market{MarketPublic}>(self.MarketPublicPath, target: self.MarketControllerStoragePath)
        emit MarketControllerCreated()
    }
}