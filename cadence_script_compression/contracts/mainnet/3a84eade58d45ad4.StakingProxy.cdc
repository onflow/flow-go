// This contract defines an interface for node stakers
// to use to be able to perform common staking actions

// It also defines a resource that a node operator can
// use to store staking proxies for all of their node operation
// relationships 

pub contract StakingProxy {

    /// path to store the node operator resource
    /// in the node operators account for staking helper
    pub let NodeOperatorCapabilityStoragePath: Path

    pub let NodeOperatorCapabilityPublicPath: Path

    /// Contains the node info associated with a node operator
    pub struct NodeInfo {

        pub let id: String
        pub let role: UInt8
        pub let networkingAddress: String
        pub let networkingKey: String
        pub let stakingKey: String

        init(nodeID: String, role: UInt8, networkingAddress: String, networkingKey: String, stakingKey: String) {
            pre {
                networkingAddress.length > 0 && networkingKey.length > 0 && stakingKey.length > 0:
                        "Address and Key have to be the correct length"
            }
            self.id = nodeID
            self.role = role
            self.networkingAddress = networkingAddress
            self.networkingKey = networkingKey
            self.stakingKey = stakingKey
        }
    }

    /// The interface that limits what a node operator can access
    /// from the staker who they operate for
    pub struct interface NodeStakerProxy {

        pub fun stakeNewTokens(amount: UFix64)

        pub fun stakeUnstakedTokens(amount: UFix64)

        pub fun requestUnstaking(amount: UFix64)

        pub fun unstakeAll()

        pub fun withdrawUnstakedTokens(amount: UFix64)

        pub fun withdrawRewardedTokens(amount: UFix64)

    }

    /// The interface the describes what a delegator can do
    pub struct interface NodeDelegatorProxy {

        pub fun delegateNewTokens(amount: UFix64)

        pub fun delegateUnstakedTokens(amount: UFix64)
        
        pub fun delegateRewardedTokens(amount: UFix64)

        pub fun requestUnstaking(amount: UFix64)

        pub fun withdrawUnstakedTokens(amount: UFix64)

        pub fun withdrawRewardedTokens(amount: UFix64)
    }

    /// The interface that a node operator publishes their NodeStakerProxyHolder
    /// as in order to allow other token holders to initialize
    /// staking helper relationships with them
    pub resource interface NodeStakerProxyHolderPublic {
        
        pub fun addStakingProxy(nodeID: String, proxy: AnyStruct{NodeStakerProxy})

        pub fun getNodeInfo(nodeID: String): NodeInfo?
    }

    /// The resource that node operators store in their accounts
    /// to manage relationships with token holders who pay them off-chain
    /// instead of with tokens
    pub resource NodeStakerProxyHolder: NodeStakerProxyHolderPublic {

        /// Maps node IDs to any struct that implements the NodeStakerProxy interface
        /// allows node operators to work with users with locked tokens
        /// and with unstaked tokens
        access(self) var stakingProxies: {String: AnyStruct{NodeStakerProxy}}

        /// Maps node IDs to NodeInfo
        access(self) var nodeInfo: {String: NodeInfo}

        init() {
            self.stakingProxies = {}
            self.nodeInfo = {}
        }

        /// Node operator calls this to add info about a node they 
        /// want to accept tokens for
        pub fun addNodeInfo(nodeInfo: NodeInfo) {
            pre {
                self.nodeInfo[nodeInfo.id] == nil
            }
            self.nodeInfo[nodeInfo.id] = nodeInfo
        }

        /// Remove node info if it isn't in use any more
        pub fun removeNodeInfo(nodeID: String): NodeInfo {
            return self.nodeInfo.remove(key: nodeID)!
        }

        /// Published function to get all the info for a specific node ID
        pub fun getNodeInfo(nodeID: String): NodeInfo? {
            return self.nodeInfo[nodeID]
        }

        /// Published function for a token holder who has signed up
        /// the node operator's NodeInfo to operate a node
        /// They store their `NodeStakerProxy` here to allow the node
        /// operator to perform some staking actions also
        pub fun addStakingProxy(nodeID: String, proxy: AnyStruct{NodeStakerProxy}) {
            pre {
                self.stakingProxies[nodeID] == nil
            }
            self.stakingProxies[nodeID] = proxy
        }

        /// The node operator can call the removeStakingProxy function
        /// to remove a staking proxy if it is no longer needed
        pub fun removeStakingProxy(nodeID: String): AnyStruct{NodeStakerProxy} {
            pre {
                self.stakingProxies[nodeID] != nil
            }

            return self.stakingProxies.remove(key: nodeID)!
        }

        /// Borrow a "reference" to the staking proxy so staking operations
        /// can be performed with it
        pub fun borrowStakingProxy(nodeID: String): AnyStruct{NodeStakerProxy}? {
            return self.stakingProxies[nodeID]
        }
    }

    /// Create a new proxy holder for a node operator
    pub fun createProxyHolder(): @NodeStakerProxyHolder {
        return <- create NodeStakerProxyHolder()
    }

    init() {
        self.NodeOperatorCapabilityStoragePath = /storage/nodeOperator
        self.NodeOperatorCapabilityPublicPath = /public/nodeOperator
    }
}
