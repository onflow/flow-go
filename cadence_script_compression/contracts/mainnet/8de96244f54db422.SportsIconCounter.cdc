pub contract SportsIconCounter {
    pub var nextNFTID: UInt64
    pub var nextSetID: UInt64
    pub var nextAuctionID: UInt64
    pub var nextBuyNowID: UInt64
    
    access(account) fun incrementNFTCounter(): UInt64 {
        self.nextNFTID = self.nextNFTID + 1
        return self.nextNFTID
    }
    
    access(account) fun incrementSetCounter(): UInt64 {
        self.nextSetID = self.nextSetID + 1
        return self.nextSetID
    }
    
    access(account) fun incrementAuctionCounter(): UInt64 {
        self.nextAuctionID = self.nextAuctionID + 1
        return self.nextAuctionID
    }
    
    access(account) fun incrementBuyNowCounter(): UInt64 {
        self.nextBuyNowID = self.nextBuyNowID + 1
        return self.nextBuyNowID
    }

    init() {
        self.nextNFTID = 1
        self.nextSetID = 1
        self.nextAuctionID = 1
        self.nextBuyNowID = 1
    }
}