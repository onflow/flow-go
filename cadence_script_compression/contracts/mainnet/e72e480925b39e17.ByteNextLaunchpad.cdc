import FungibleToken from 0xf233dcee88fe0abe

pub contract ByteNextLaunchpad {
    pub let LaunchpadStoragePath: StoragePath;
    pub let LaunchpadPublicPath: PublicPath;

    pub struct LaunchpadInfo{
        pub(set) var _startTime: UFix64;
        pub(set) var _endTime: UFix64;

        // Price of token to INO
        pub(set) var _tokenPrice: UFix64;

        // Type of token to INO
        pub(set) var _tokenType: Type;

        // Type of payment vault which user will paid
        pub(set) var _paymentType: Type;

        // User Allocation in PaymentType
        pub(set) var _userAllocations: {Address: UFix64};
        pub(set) var _userBoughts: {Address: UFix64};

        // Mapping the receiver token address
        pub var _userTokenReceiver: {Address: Capability<&{FungibleToken.Receiver}>};

        // The receiver to receiver user fund when join pool
        pub var _tokenReceiver: Capability<&{FungibleToken.Receiver}>;

        pub var _totalRaise: UFix64;
        pub var _totalBought: UFix64;

        pub(set) var _claimingTimes: [UFix64];
        pub(set) var _claimingPercents: [UFix64];
        pub(set) var _claimingCounts: {Address: Int};

        init(startTime: UFix64, endTime: UFix64, tokenPrice: UFix64,
            tokenType: Type, paymentType: Type, tokenReceiver: Capability<&{FungibleToken.Receiver}>,
            claimingTimes: [UFix64], claimingPercents: [UFix64], totalRaise: UFix64)  {
            self._startTime = startTime;
            self._endTime = endTime;
            self._tokenPrice = tokenPrice;
            self._tokenType = tokenType;
            self._paymentType = paymentType;
            self._totalBought = 0.0;
            self._tokenReceiver = tokenReceiver;

            self._userAllocations = {};
            self._userBoughts = {};

            self._claimingTimes = claimingTimes;
            self._claimingPercents = claimingPercents;
            self._claimingCounts = {};
            self._userTokenReceiver = {};
            self._totalRaise = totalRaise;
        }

        pub fun setTotalBought(_ value: UFix64){
            self._totalBought = value;
        }
    }
  
    pub resource interface LaunchpadPublic {
        pub fun join(id: Int, paymentVault: @FungibleToken.Vault, tokenReceiver: Capability<&{FungibleToken.Receiver}>);
        pub fun claim(id: Int, address: Address); 
        pub fun getLaunchpadCount(): Int;
        pub fun getLaunchpadInfo(id: Int): LaunchpadInfo?;
        pub fun getUserAllocation(id: Int, _ account: Address): UFix64;
        pub fun getClaimable(id: Int, _ account: Address): UFix64;
    }

    pub resource Launchpad: LaunchpadPublic {
        //The number of launchpad
        access(self) var _launchpadCount: Int;

        //Dictionary that stores launchpad information
        access(self) var _launchpads: {Int: LaunchpadInfo};

        access(self) var _launchpadTokens: @{Int: FungibleToken.Vault};

        init() {
            self._launchpadCount = 0;
            self._launchpads = {};
            self._launchpadTokens <- {};
        }

        pub fun createNewLaunchpad(startTime: UFix64, endTime: UFix64, 
            tokenPrice: UFix64, tokenType: Type,
            paymentType: Type, tokenReceiver: Capability<&{FungibleToken.Receiver}>,
            claimingTimes: [UFix64], claimingPercents: [UFix64], totalRaise: UFix64){
            pre{
                startTime < endTime: "startTime should be less than endTime";
            }
            self._launchpadCount = self._launchpadCount + 1;
            self._launchpads[self._launchpadCount] =  LaunchpadInfo(
                startTime: startTime,
                endTime: endTime,
                tokenPrice: tokenPrice,
                tokenType: tokenType,
                paymentType: paymentType,
                tokenReceiver: tokenReceiver,
                claimingTimes: claimingTimes,
                claimingPercents: claimingPercents,
                totalRaise: totalRaise 
            );

            emit NewLauchpadCreated(id: self._launchpadCount, startTime: startTime, endTime: endTime, tokenType: tokenType, paymentType: paymentType);
        }

        pub fun setTime(id: Int, startTime: UFix64, endTime: UFix64){
            pre{
                startTime < endTime: "startTime should be less than endTime";
            }

            var launchpadInfo = self._launchpads[id]!;
            launchpadInfo._startTime = startTime;
            launchpadInfo._endTime = startTime;
            self._launchpads[id] = launchpadInfo;

            emit LaunchpadTimeUpdated(id: id, startTime: startTime, endTime: endTime);
        }

        pub fun setUserAllocation(id: Int, account: Address, allocation: UFix64){
            pre{
                self.getLaunchpadInfo(id: id) != nil: "Invalid launchpad id";
                allocation > 0.0: "Allocation should be greater than 0";
            }

            self._launchpads[id]!._userAllocations.remove(key: account);
            self._launchpads[id]!._userAllocations.insert(key: account, allocation);

            emit UserAllocationSetted(id: id, account: account, allocation: allocation);
        }

        pub fun depositLaunchpadToken(id: Int, newVault: @FungibleToken.Vault) {
            pre {
                self.getLaunchpadInfo(id: id) != nil: "Invalid launchpad id";
                newVault.isInstance(self.getLaunchpadInfo(id: id)!._tokenType): "Launchpad token mismatch"
            }

            let vault <- self._launchpadTokens.remove(key: id);
            if vault != nil {
                newVault.deposit(from: <- vault!)
            } else {
                destroy vault;
            }

            let oldVault <- self._launchpadTokens[id] <- newVault;
            destroy  oldVault
        }

        pub fun withdrawLaunchpadToken(id: Int, amount: UFix64): @FungibleToken.Vault {
            let vault <- self._launchpadTokens.remove(key: id)!;
            let withdrawVault <- vault.withdraw(amount: amount);
            let oldVault <- self._launchpadTokens[id] <- vault;
            destroy  oldVault

            return <- withdrawVault;
        }

        //PUBLIC FUNCTIONS
        pub fun getLaunchpadCount(): Int{
            return self._launchpadCount;
        }

        pub fun getLaunchpadInfo(id: Int): LaunchpadInfo?{
            return self._launchpads[id];
        }

        pub fun getUserAllocation(id: Int,_ account: Address): UFix64{
            let launchpadInfo : LaunchpadInfo? = self.getLaunchpadInfo(id: id);
            if(launchpadInfo == nil){
                return 0.0;
            }
            
            let userAllocation = launchpadInfo!._userAllocations[account];
            if(userAllocation == nil){
                return 0.0;
            }

            return userAllocation!;
        }

        pub fun getClaimable(id: Int, _ account: Address): UFix64{
            if(self.getLaunchpadInfo(id: id) == nil) {return 0.0;}
            let launchpadInfo : LaunchpadInfo = self.getLaunchpadInfo(id: id)!;
            let now: UFix64 =  getCurrentBlock().timestamp;
            if(now <= launchpadInfo._endTime){
                return 0.0;
            }

            let userBought: UFix64 = launchpadInfo._userBoughts[account]!;
            if(userBought == nil || userBought == 0.0){
                return 0.0;
            }

            let claimingTimeLength: Int = launchpadInfo._claimingTimes.length;

            if(claimingTimeLength == 0){
                return 0.0;
            }

            if(getCurrentBlock().timestamp < launchpadInfo._claimingTimes[0]){
                return 0.0;
            }

            var startIndex: Int = launchpadInfo._claimingCounts[account] ?? 0;
            if(startIndex >= claimingTimeLength){
                return 0.0;
            }

            var index: Int = startIndex;

            //  userBought / launchpadInfo._tokenPrice * percent1 / 100.0
            //  userBought / launchpadInfo._tokenPrice * percent2 / 100.0
            //  userBought / launchpadInfo._tokenPrice * percent3 / 100.0

            var totalPercent: UFix64 = 0.0;
            while(index < claimingTimeLength){
                let claimingTime: UFix64 = launchpadInfo._claimingTimes[index];
                if(now >= claimingTime){
                    totalPercent = totalPercent +  launchpadInfo._claimingPercents[index];
                } else {
                    break;
                }
                index = index + 1;
            }

            return (userBought / launchpadInfo._tokenPrice) * totalPercent / 100.0;
        }

        pub fun join(id: Int, paymentVault: @FungibleToken.Vault, tokenReceiver: Capability<&{FungibleToken.Receiver}>){
            pre{
                self.getLaunchpadInfo(id: id) != nil: "Launchpad id is invalid";
                self.getUserAllocation(id: id, tokenReceiver.address) > 0.0: "You can not join this launchpad";
                paymentVault.isInstance(self.getLaunchpadInfo(id: id)!._paymentType):
                        "Payment token is not allowed";
            }

            let launchpadInfo : LaunchpadInfo = self.getLaunchpadInfo(id: id)!;
            if(launchpadInfo._startTime > getCurrentBlock().timestamp || launchpadInfo._endTime < getCurrentBlock().timestamp){
                panic("Can not join this launchpad at this time");
            }

            let account: Address = tokenReceiver.address;
            var userBoughtInPaymentToken: UFix64 = 0.0;
            if(launchpadInfo._userBoughts[account] == nil){
                userBoughtInPaymentToken = 0.0;
            }
            else{
                userBoughtInPaymentToken = launchpadInfo._userBoughts[account]!;
            }

            let maxPaymentToBuy = launchpadInfo._userAllocations[account]! - userBoughtInPaymentToken;
            if(maxPaymentToBuy == 0.0){
                panic("You can not join this launchpad anymore");
            }

            if(maxPaymentToBuy < paymentVault.balance){
                panic("Out of allocation");
            }

            var tokenToBuy = paymentVault.balance / launchpadInfo._tokenPrice;
            if (self._launchpads[id]!._totalBought + tokenToBuy > launchpadInfo._totalRaise) {
                panic("Exceed the total token raised");
            }

            var maxPaymentToken: UFix64 = maxPaymentToBuy * launchpadInfo._tokenPrice;
            if(maxPaymentToken > paymentVault.balance){
                maxPaymentToken = paymentVault.balance;
            }

            tokenToBuy = maxPaymentToken / launchpadInfo._tokenPrice;

            // Store the token receiver
            self._launchpads[id]!._userTokenReceiver.remove(key: account);
            self._launchpads[id]!._userTokenReceiver.insert(key: account, tokenReceiver);

            self._launchpads[id]!._userBoughts.remove(key: account);
            self._launchpads[id]!._userBoughts.insert(key: account, userBoughtInPaymentToken + paymentVault.balance);
            self._launchpads[id]!.setTotalBought(self._launchpads[id]!._totalBought + paymentVault.balance);

            emit Joined(account: account, id: id, tokenQuantity: tokenToBuy, paymentAmount: paymentVault.balance);

            launchpadInfo._tokenReceiver.borrow()!.deposit(from: <- paymentVault);
        }

        pub fun claim(id: Int, address: Address){
            pre {
                self.getLaunchpadInfo(id: id) != nil: "Launchpad id is invalid";
                self.getLaunchpadInfo(id: id)!._userTokenReceiver[address] != nil: "You were not join this launchpad";
                self.getUserAllocation(id: id, address) > 0.0: "You can not join this launchpad";
            }

            let launchpadInfo : LaunchpadInfo = self.getLaunchpadInfo(id: id)!;
            let now: UFix64 =  getCurrentBlock().timestamp;
            if(now <= launchpadInfo._endTime){
                panic("Can not claim token of this launchpad at this time");
            }


            let tokenReceiver = self.getLaunchpadInfo(id: id)!._userTokenReceiver[address]!;
            let account: Address = tokenReceiver.address;

            let userBought: UFix64 = launchpadInfo._userBoughts[account]!;
            if(userBought == nil || userBought == 0.0){
                panic("You can not claim for this launchpad");
            }

            let claimingTimeLength: Int = launchpadInfo._claimingTimes.length;

            if(claimingTimeLength == 0){
                panic("Can not claim at this time");
            }

            if(getCurrentBlock().timestamp < launchpadInfo._claimingTimes[0]){
                panic("Can not claim at this time");
            }

            var startIndex: Int = launchpadInfo._claimingCounts[account] ?? 0;
            if(startIndex >= claimingTimeLength){
                panic("You have claimed all token");
            }

            var index: Int = startIndex;
            var totalPercentClaim = 0.0;
            while(index < claimingTimeLength) {
                let claimingTime: UFix64 = launchpadInfo._claimingTimes[index];

                if(now >= claimingTime){
                    totalPercentClaim = totalPercentClaim + launchpadInfo._claimingPercents[index];

                    let claimingCount: Int = self._launchpads[id]!._claimingCounts[account] ?? 0;

                    self._launchpads[id]!._claimingCounts.remove(key:account);
                    self._launchpads[id]!._claimingCounts.insert(key:account, claimingCount + 1);
                } else {
                    break;
                }
                index = index + 1;
            }

            var tokenQuantity: UFix64 = (userBought / launchpadInfo._tokenPrice) * totalPercentClaim / 100.0;
            if(tokenQuantity > 0.0){
                let token <- self._launchpadTokens.remove(key: id)!;

                let claimingToken <- token.withdraw(amount: tokenQuantity);
                tokenReceiver.borrow()!.deposit(from: <- claimingToken);

                let oldToken <- self._launchpadTokens.insert(key: id, <- token);
                destroy  oldToken;
            }

            emit Claimed(account: account, id: id, tokenQuantity: tokenQuantity);
        }
        
        destroy() {
            destroy self._launchpadTokens;
        }
    }

    init() {
        self.LaunchpadStoragePath = /storage/byteNextByteNextLaunchpad;
        self.LaunchpadPublicPath = /public/byteNextPublicByteNextLaunchpad;

        self.account.save(<- create Launchpad(), to: self.LaunchpadStoragePath);
        self.account.link<&Launchpad{LaunchpadPublic}>(self.LaunchpadPublicPath, target: self.LaunchpadStoragePath);
    }

    pub event NewLauchpadCreated(id: Int, startTime: UFix64, endTime: UFix64, tokenType: Type, paymentType: Type);
    pub event LaunchpadTimeUpdated(id: Int, startTime: UFix64, endTime: UFix64);
    pub event UserAllocationSetted(id: Int, account: Address, allocation: UFix64);
    pub event Joined(account: Address, id: Int, tokenQuantity: UFix64, paymentAmount: UFix64);
    pub event Claimed(account: Address, id: Int, tokenQuantity: UFix64);
}
