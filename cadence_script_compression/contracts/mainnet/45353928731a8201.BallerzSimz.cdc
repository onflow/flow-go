import Gaia from 0x8b148183c28ff88f
import FlowToken from 0x1654653399040a61
import FungibleToken from 0xf233dcee88fe0abe

pub contract BallerzSimz {

    /****************
    *  Named Paths  *
    *****************/
    // Internal paths
    pub let BallerzSimzResourcePublicPath: PublicPath

    pub let BallerzSimzResourceStoragePath: StoragePath
    pub let BallerzSimzConfigStoragePath: StoragePath

    pub let BallerzSimzResourceAdminPath: PrivatePath
    pub let BallerzSimzConfigAdminPath: PrivatePath

    // External paths
    pub let BallerzCollectionPublicPath: PublicPath
    pub let FlowTokenReceiverPublicPath: PublicPath

    /*****************
    *Conctract Fields*
    ******************/
    access(contract) var latestSimId: UInt64
    access(contract) var latestTeamId: UInt64

    // Cadence doesn't have Sets so use a dict instead for quick removal
    pub let activeBallerIdz: {UInt64: Bool}

    /****************
    *     Events    *
    *****************/
    pub event SimStarted(simId: UInt64, teams: [BallerzTeamSimEventInfo], bonuses: {String: String})
    // scores are cumulative: [1st, 2nd, 3rd, 4th]
    pub event SimCompleted(
        simId: UInt64, winner: BallerzTeamSimEventInfo, loser: BallerzTeamSimEventInfo,
        winnerScores: [UInt16], loserScores: [UInt16], winningPayout: UFix64
    )

    pub event TeamCreated(
        id: UInt64, privateTeam: Bool, teamName: String, isPrivate: Bool, entryAmount: UFix64
    )
    pub event TeamDeleted(id: UInt64)

    pub event BallerJoinedTeam(teamId: UInt64, ballerId: UInt64, entryAmount: UFix64)
    pub event BallerLeftTeam(teamId: UInt64, ballerId: UInt64)

    pub struct BallerzTeamSimEventInfo {
        pub let teamId: UInt64
        pub let ballerIdz: [UInt64]
        pub let ballerOwnerz: {UInt64 : Address}
        pub let privateTeam: Bool
        pub let teamName: String
        pub let entryFee: UFix64
        pub let feePercentage: UFix64

        init(
            teamId: UInt64, ballerIdz: [UInt64], ballerOwnerz: {UInt64: Address}, privateTeam: Bool,
            teamName: String, entryFee: UFix64, feePercentage: UFix64
        ) {
            self.teamId = teamId
            self.ballerIdz = ballerIdz
            self.ballerOwnerz = ballerOwnerz
            self.privateTeam = privateTeam
            self.teamName = teamName
            self.entryFee = entryFee
            self.feePercentage = feePercentage
        }
    }

    /****************
    *   Resources   *
    *****************/
    // Admin resource. Published only to private storage so the owner can start and complete sims.
    pub resource interface BallerzSimzAdmin {
        pub fun startSim(
            team1Id: UInt64,
            team2Id: UInt64,
            entryAmount: UFix64,
            feePercentage: UFix64,
            bonuses: {String: String}
        ): UInt64
        pub fun completeSim(
            simId: UInt64,
            winnerId: UInt64,
            loserId: UInt64,
            winnerScores: [UInt16],
            loserScores: [UInt16]
        )
        pub fun updateFeeOwner(newAddress: Address)
    }

    // Public resource
    pub resource interface BallerzSimzPublic {
        // Getters
        pub fun getActiveSimz(): [&ActiveSim]
        pub fun getActiveSimIdz(): [UInt64]
        pub fun getActiveSim(simId: UInt64): &ActiveSim
        pub fun getActiveSimzForIdz(simIdz: [UInt64]): [&ActiveSim]
        pub fun getWaitingTeamz(): [&BallerzTeam]
        pub fun getWaitingTeamzForIdz(teamIdz: [UInt64]): [&BallerzTeam]
        pub fun getWaitingTeamIdz(): [UInt64]
        pub fun getWaitingTeam(teamId: UInt64): &BallerzTeam

        // These allow an account that holds a baller to take action on teams
        pub fun createTeam(
            baller: &Gaia.NFT,
            entryTokens: @FungibleToken.Vault,
            teamSize: UInt8,
            privateTeam: Bool,
            teamName: String
        ): UInt64
        pub fun joinTeam(baller: &Gaia.NFT, teamId: UInt64, entryTokens: @FungibleToken.Vault)
        pub fun leaveTeam(baller: &Gaia.NFT, teamId: UInt64)
    }

    // Manager Admin interface. Published only to private storage so the admin can change config values.
    pub resource interface BallerzSimzConfigAdmin {
        pub fun addEntryFee(newEntryFee: UFix64, feePercentage: UFix64)
        pub fun removeEntryFee(entryFee: UFix64)
        pub fun addTeamSize(newTeamSize: UInt8)
        pub fun removeTeamSize(teamSize: UInt8)

        pub fun getCurrentEntryFees(): [UFix64]
        pub fun getEntryFeeToFeePercentageMap(): {UFix64: UFix64}
        pub fun getTeamSizes(): [UInt8]
    }

    // ActiveSim is a resource for a sim that has been started with two active teams with full rosters
    pub resource ActiveSim {

        pub let simId: UInt64
        pub let teamSize: UInt8
        pub let entryAmount: UFix64
        pub let feePercentage: UFix64
        pub let teams: @{UInt64: BallerzTeam}

        init(simId: UInt64, team1: @BallerzTeam, team2: @BallerzTeam, entryAmount: UFix64, feePercentage: UFix64) {
            pre {
                team1.getTeamSize() == team2.getTeamSize() : "Team size mismatch"
                team1.entryFee == team2.entryFee : "Entry fee mismatch"
            }

            self.simId = simId
            self.entryAmount = entryAmount
            self.feePercentage = feePercentage

            // Store teams
            self.teamSize = team1.getTeamSize()
            self.teams <- {}
            self.teams[team1.teamId] <-! team1
            self.teams[team2.teamId] <-! team2
        }

        destroy() {
            destroy self.teams
        }

    }

    // The BallerzTeam resource holds the baller ids on the team as well as their tokens and nft/flow token capabilities.
    pub resource BallerzTeam {

        pub let teamId: UInt64
        pub let privateTeam: Bool
        pub let entryFee: UFix64
        pub let feePercentage: UFix64
        pub let teamName: String
        pub var startedSim: Bool

        // This is the final team size, a sim cannot be started unless ballerIdz.length == teamSize
        access(self) let teamSize: UInt8
        access(self) let ballerIdz: [UInt64]
        // {ballerId: ownerAddress}
        access(self) let ballerOwnerz: {UInt64: Address}

        // Capabilities to let us access the baller NFTs (to access the metadata)
        access(contract) let ownerCapabilities: {UInt64: Capability<&AnyResource{Gaia.CollectionPublic}>}
        // Capabilities to payout winners
        access(contract) let ownerReceivers: {UInt64: Capability<&{FungibleToken.Receiver}>}
        // Entry fees
        access(contract) let entryTokens: @{UInt64: FungibleToken.Vault}

        // When we create a new team we start with one baller, we can't have an empty team.
        init(teamSize: UInt8, privateTeam: Bool, teamName: String, startingBaller: &Gaia.NFT, entryTokens: @FungibleToken.Vault) {
            pre {
                !BallerzSimz.activeBallerIdz.containsKey(startingBaller.id) : "Baller already active"
                BallerzSimz.getCurrentEntryFees().contains(entryTokens.balance) : "Incorrect entry fee"
                BallerzSimz.getTeamSizes().contains(teamSize) : "Invalid teamSize"
            }

            // Increase latestTeamId to be used below
            BallerzSimz.latestTeamId = BallerzSimz.latestTeamId + (1 as UInt64)

            self.startedSim = false
            self.teamSize = teamSize
            self.privateTeam = privateTeam
            self.teamId = BallerzSimz.latestTeamId
            self.entryFee = entryTokens.balance
            self.teamName = teamName
            self.feePercentage = BallerzSimz.getEntryFeeToFeePercentageMap()[entryTokens.balance]!

            self.ballerIdz = []
            self.ownerCapabilities = {}
            self.ownerReceivers = {}
            self.entryTokens <- {}
            self.ballerOwnerz = {}

            // Join the team with the starting baller
            self.joinTeam(baller: startingBaller, entryTokens: <-entryTokens)

            emit TeamCreated(
                id: self.teamId, privateTeam: privateTeam,
                teamName: teamName, isPrivate: privateTeam,
                entryAmount: self.entryFee
            )
        }

        pub fun getTeamSize(): UInt8 {
            return self.teamSize
        }

        pub fun getBallerIdz(): [UInt64] {
            return self.ballerIdz
        }

        pub fun getBallerOwnerz(): {UInt64: Address} {
            return self.ballerOwnerz
        }

        // Allow a baller to join a team. The calling account must own the baller
        access(contract) fun joinTeam(baller: &Gaia.NFT, entryTokens: @FungibleToken.Vault) {
            pre {
                self.ballerIdz.length < Int(self.teamSize) : "This team is full"
                !self.ballerIdz.contains(baller.id) : "Baller already on team"
                !BallerzSimz.activeBallerIdz.containsKey(baller.id) : "Baller already active"
                self.startedSim == false : "Sim already started with team"
                entryTokens.balance == self.entryFee : "Incorrect entry fee"
            }

            let ballerOwner = baller.owner!

            // Verify baller ownership
            let ownerGaiaCollectionCapability = ballerOwner.getCapability<&{Gaia.CollectionPublic}>(
                BallerzSimz.BallerzCollectionPublicPath
            )
            let collectionRef = ownerGaiaCollectionCapability.borrow()!
            if !collectionRef.getIDs().contains(baller.id) {
                panic("Address does not own baller")
            }

            self.ballerIdz.append(baller.id)

            // get capability to access baller metadata
            self.ownerCapabilities.insert(key: baller.id, ownerGaiaCollectionCapability)
            // get capability to deposit winnings into owners account
            self.ownerReceivers.insert(key: baller.id, ballerOwner.getCapability<&{FungibleToken.Receiver}>(
                BallerzSimz.FlowTokenReceiverPublicPath
            ))
            // store entry tokens
            self.entryTokens[baller.id] <-! entryTokens
            self.ballerOwnerz[baller.id] = ballerOwner.address

            BallerzSimz.addActiveBaller(ballerId: baller.id)

            emit BallerJoinedTeam(teamId: self.teamId, ballerId: baller.id, entryAmount: self.entryFee)
        }

        // Allows a baller to leave the team. This pays the owner back their entry fee.
        // Returns the number of ballerz on the team AFTER baller leaves team so calling function can
        // destroy the team if its empty.
        access(contract) fun leaveTeam(baller: &Gaia.NFT): Int {
            pre {
                self.startedSim == false : "Sim already started with team"
                self.ballerIdz.contains(baller.id) : "Baller not on team"
            }

            // Find baller index to remove
            var idx = 0
            for currBallerId in self.ballerIdz {
                if currBallerId == baller.id {
                    break
                }
                idx = idx + 1
            }

            // Remove baller and capabilities
            self.ballerIdz.remove(at: idx)
            self.ownerCapabilities.remove(key: baller.id)

            // Move tokens back to owner
            let ownerReceiver = self.ownerReceivers.remove(key: baller.id)!
            ownerReceiver.borrow()!.deposit(from: <- self.entryTokens.remove(key: baller.id)!)

            // Remove active baller
            BallerzSimz.removeActiveBaller(ballerId: baller.id)

            emit BallerLeftTeam(teamId: self.teamId, ballerId: baller.id)

            return self.ballerIdz.length
        }

        // Validates that the team can start a sim and sets startedSim to true if it can.
        access(contract) fun startSimForTeam() {
            pre {
                self.startedSim == false : "Sim already started with team"
                UInt8(self.ballerIdz.length) == self.teamSize : "Team not full"
            }

            self.startedSim = true
        }

        // Transfers all entry fees into the receiver. This is used to combine all tokens and then payout winners.
        access(contract) fun transferAllEntryTokens(receiver: Capability<&{FungibleToken.Receiver}>) {
            for ballerId in self.entryTokens.keys {
                let entryToken <- self.entryTokens.remove(key: ballerId)!
                receiver.borrow()!.deposit(from: <- entryToken)
            }
        }

        destroy() {
            if !self.startedSim {
                // Remove ballerz
                for ballerId in self.ballerIdz {
                    self.leaveTeam(
                        baller: self.ownerCapabilities[ballerId]!.borrow()!.borrowGaiaNFT(id: ballerId)!
                    )
                }

                // Make sure we've payed everyone back their entry fees before destroying
                assert(self.entryTokens.keys.length == 0, message: "entry tokens not empty, it should be")
            } else {
                // Remove all ballerz from activeBallerIdz
                for ballerId in self.ballerIdz {
                    BallerzSimz.removeActiveBaller(ballerId: ballerId)
                }
            }

            destroy self.entryTokens
            emit TeamDeleted(id: self.teamId)
        }

    }

    // BallerzSimzResource is the main resource in the contract. It holds all active sims, any waiting teams, as well as
    // the vaults/capabilities for the contract owner and the account that fees should be paid to.
    pub resource BallerzSimzResource: BallerzSimzAdmin, BallerzSimzPublic {

        /****************
        *Class variables*
        *****************/
        // These are sims that have been started
        // {simId : ActiveSim resource}
        access(account) let activeSimz: @{UInt64: ActiveSim}
        // A waiting team is a team that has been created but is waiting to join a sim. This could be because the team is
        // not yet full or it is full and is just waiting to be matched with a team and a sim to start.
        // {teamId: BallerzTeam}
        access(account) let waitingTeamz: @{UInt64: BallerzTeam}
        // Capability where fees are paid
        access(account) var feeOwnerCapability: Capability<&{FungibleToken.Receiver}>
        // Holds entryTokens until sim is complete
        access(account) let ownerVaultReceiver: Capability<&{FungibleToken.Receiver}>
        // Pays winnngs to winning accounts
        access(self)    let ownerVaultProvider: Capability<&{FungibleToken.Provider}>
        access(self)    let addressOwner: Capability<&{FungibleToken.Balance}>

        init(signer: AuthAccount, feeOwnerCapability: Capability<&{FungibleToken.Receiver}>) {
            self.feeOwnerCapability = feeOwnerCapability
            self.activeSimz <- {}
            self.waitingTeamz <- {}

            self.addressOwner = signer.getCapability<&{FungibleToken.Balance}>(/public/BallerzOwnerEscrowVaultBalance)
            // Store receiver and provider for entry tokens
            self.ownerVaultReceiver = signer.getCapability<&{FungibleToken.Receiver}>(/public/BallerzOwnerEscrowVaultRecevier)
            self.ownerVaultProvider = signer.getCapability<&{FungibleToken.Provider}>(/private/BallerzOwnerEscrowVaultProvider)
        }

        /****************
        * Class methods *
        *****************/

        pub fun getActiveSimz(): [&ActiveSim] {
            let activeSimzRefs: [&ActiveSim] = []
            for simId in self.activeSimz.keys {
                activeSimzRefs.append((&self.activeSimz[simId] as &ActiveSim))
            }

            return activeSimzRefs
        }

        pub fun getActiveSimzForIdz(simIdz: [UInt64]): [&ActiveSim] {
            let activeSimzRefs: [&ActiveSim] = []
            for simId in simIdz {
                if self.activeSimz[simId] != nil {
                    activeSimzRefs.append((&self.activeSimz[simId] as &ActiveSim))
                }
            }

            return activeSimzRefs
        }

        pub fun getActiveSimIdz(): [UInt64] {
            return self.activeSimz.keys
        }

        pub fun getActiveSim(simId: UInt64): &ActiveSim {
            pre {
                self.activeSimz.containsKey(simId) : "No active sim with given id"
            }

            return &self.activeSimz[simId] as &ActiveSim
        }

        pub fun getWaitingTeamz(): [&BallerzTeam] {
            let teams: [&BallerzTeam] = []
            for teamId in self.waitingTeamz.keys {
                teams.append(&self.waitingTeamz[teamId] as &BallerzTeam)
            }

            return teams
        }

        pub fun getWaitingTeamzForIdz(teamIdz: [UInt64]): [&BallerzTeam] {
            let teams: [&BallerzTeam] = []
            for teamId in teamIdz {
                if self.waitingTeamz[teamId] != nil {
                    teams.append(&self.waitingTeamz[teamId] as &BallerzTeam)
                }
            }

            return teams
        }

        pub fun getWaitingTeamIdz(): [UInt64] {
            return self.waitingTeamz.keys
        }

        pub fun getWaitingTeam(teamId: UInt64): &BallerzTeam {
            pre {
                self.waitingTeamz[teamId] != nil : "No waiting team for teamId"
            }

            return &self.waitingTeamz[teamId] as &BallerzTeam
        }

        pub fun updateFeeOwner(newAddress: Address) {
            let newAddressAccount = getAccount(newAddress)
            self.feeOwnerCapability = newAddressAccount.getCapability<&{FungibleToken.Receiver}>(
                /public/flowTokenReceiver
            )
        }

        // Creates a new team with a baller and entry tokens for that first baller.
        pub fun createTeam(baller: &Gaia.NFT, entryTokens: @FungibleToken.Vault, teamSize: UInt8, privateTeam: Bool, teamName: String): UInt64 {
            pre {
                BallerzSimz.getTeamSizes().contains(teamSize) : "Team size not allowed"
            }

            let newTeam <- create BallerzTeam(
                teamSize: teamSize,
                privateTeam: privateTeam,
                teamName: teamName,
                startingBaller: baller,
                entryTokens: <-entryTokens
            )

            let teamId = newTeam.teamId
            self.waitingTeamz[newTeam.teamId] <-! newTeam
            return teamId
        }

        // Allows a baller to join a team given they pass in their entry fee
        pub fun joinTeam(baller: &Gaia.NFT, teamId: UInt64, entryTokens: @FungibleToken.Vault) {
            pre {
                self.waitingTeamz.containsKey(teamId) : "Unknown team"
            }

            let team <- self.waitingTeamz.remove(key: teamId)!
            team.joinTeam(baller: baller, entryTokens: <- entryTokens)
            self.waitingTeamz[teamId] <-! team
        }

        // Allows a baller to leave a team. Their entry fee is returned to their account
        pub fun leaveTeam(baller: &Gaia.NFT, teamId: UInt64) {
            pre {
                self.waitingTeamz.containsKey(teamId) : "Unknown team"
            }

            let team <- self.waitingTeamz.remove(key: teamId)!
            let currentTeamSize = team.leaveTeam(baller: baller)
            self.waitingTeamz[teamId] <-! team

            if currentTeamSize == 0 {
                destroy self.waitingTeamz.remove(key: teamId)
            }
        }

        // Starts a new sim. The 2 teams must have full rosters.
        pub fun startSim(
            team1Id: UInt64,
            team2Id: UInt64,
            entryAmount: UFix64,
            feePercentage: UFix64,
            bonuses: {String : String}
        ): UInt64 {
            pre {
                !self.activeSimz.containsKey(BallerzSimz.latestSimId + 1) : "latestSimId + 1 already in use, retry"
                BallerzSimz.getCurrentEntryFees().contains(entryAmount) : "Incorrect entry fee"
                feePercentage == BallerzSimz.getEntryFeeToFeePercentageMap()[entryAmount] : "Incorrect fee percentage"
                self.waitingTeamz.containsKey(team1Id) : "team 1 not waiting"
                self.waitingTeamz.containsKey(team2Id) : "team 2 not waiting"
            }

            BallerzSimz.latestSimId = BallerzSimz.latestSimId + (1 as UInt64)

            // Assign an id
            let simId = BallerzSimz.latestSimId

            // Start sim for teams
            let team1 <- self.waitingTeamz.remove(key: team1Id)!
            let team2 <- self.waitingTeamz.remove(key: team2Id)!
            team1.startSimForTeam()
            team2.startSimForTeam()

            let team1Info = BallerzTeamSimEventInfo(
                teamId: team1Id, ballerIdz: team1.getBallerIdz(), ballerOwnerz: team1.getBallerOwnerz(), privateTeam: team1.privateTeam,
                teamName: team1.teamName, entryFee: team1.entryFee, feePercentage: team1.feePercentage
            )
            let team2Info = BallerzTeamSimEventInfo(
                teamId: team2Id, ballerIdz: team2.getBallerIdz(), ballerOwnerz: team2.getBallerOwnerz(), privateTeam: team2.privateTeam,
                teamName: team2.teamName, entryFee: team2.entryFee, feePercentage: team2.feePercentage
            )

            // Create the sim
            self.activeSimz[simId] <-! create ActiveSim(
                simId:         simId,
                team1:         <- team1,
                team2:         <- team2,
                entryAmount:   entryAmount,
                feePercentage: feePercentage
            )

            // Emit event
            emit SimStarted(simId: simId, teams: [team1Info, team2Info], bonuses: bonuses)

            return simId
        }

        // Completes a sim. This pays out the fee to the feeOwner and the winnings split evenly amonst the winning team.
        pub fun completeSim(
            simId: UInt64,
            winnerId: UInt64,
            loserId: UInt64,
            winnerScores: [UInt16],
            loserScores: [UInt16]
        ) {
            pre {
                self.activeSimz.keys.contains(simId): "No active sim for id"
                winnerScores.length == 4 : "Invalid winnerScores"
                loserScores.length == 4 : "Invalid loserScores"
                winnerScores[3] > loserScores[3] : "Invalid score, winner not winning"
            }

            // Remove (and move) active sim
            let sim <- self.activeSimz.remove(key: simId)!
            let teamIds = sim.teams.keys

            // Calculate payout
            let NUM_TEAMS = 2.0
            let totalPotAmount = sim.entryAmount * NUM_TEAMS * UFix64(sim.teamSize)
            let totalFeeAmount = totalPotAmount * sim.feePercentage
            let winningTotalPayout = totalPotAmount - totalFeeAmount
            let winningOwnerPayoutAmount = winningTotalPayout / UFix64(sim.teamSize)

            // Pull all the entry tokens into the owner vault so we can easily payout
            let winningTeam <- sim.teams.remove(key: winnerId) ?? panic("winning team not active for simId")
            let losingTeam <- sim.teams.remove(key: loserId) ?? panic("losing team not active for simId")
            winningTeam.transferAllEntryTokens(receiver: self.ownerVaultReceiver)
            losingTeam.transferAllEntryTokens(receiver: self.ownerVaultReceiver)

            // Payout winners evenly
            let ownerVaultRef = self.ownerVaultProvider.borrow()!
            for ballerId in winningTeam.getBallerIdz() {
                let ownerCapability = winningTeam.ownerReceivers[ballerId]!
                let receiver = ownerCapability.borrow()!
                let paymentCut <- ownerVaultRef.withdraw(amount: winningOwnerPayoutAmount)
                receiver.deposit(from: <- paymentCut)
            }

            // Payout fees to fee owner
            let feeCut <- ownerVaultRef.withdraw(amount: totalFeeAmount)
            self.feeOwnerCapability.borrow()!.deposit(from: <-feeCut)

            let winnerInfo = BallerzTeamSimEventInfo(
                teamId: winnerId, ballerIdz: winningTeam.getBallerIdz(), ballerOwnerz: winningTeam.getBallerOwnerz(),
                privateTeam: winningTeam.privateTeam, teamName: winningTeam.teamName, entryFee: winningTeam.entryFee,
                feePercentage: winningTeam.feePercentage
            )
            let loserInfo = BallerzTeamSimEventInfo(
                teamId: loserId, ballerIdz: losingTeam.getBallerIdz(), ballerOwnerz: losingTeam.getBallerOwnerz(),
                privateTeam: losingTeam.privateTeam, teamName: losingTeam.teamName, entryFee: losingTeam.entryFee,
                feePercentage: losingTeam.feePercentage
            )
            emit SimCompleted(
                simId: simId, winner: winnerInfo, loser: loserInfo,
                winnerScores: winnerScores, loserScores: loserScores, winningPayout: winningOwnerPayoutAmount
            )

            destroy winningTeam
            destroy losingTeam
            destroy sim
        }

        destroy() {
            destroy self.activeSimz
            destroy self.waitingTeamz
        }
    }

    pub resource BallerzSimzConfig : BallerzSimzConfigAdmin {
        access(self) var teamSizes: [UInt8]
        // {entryFee : feePercentage}
        access(self) var entryFeeToFeePercentageMap: {UFix64 : UFix64}
        access(self) var currentEntryFees: [UFix64]

        init() {
            // At 1 Flow = 11.72 USD, this is ~$5
            self.currentEntryFees = [0.4265]
            // 5% fee fow now
            self.entryFeeToFeePercentageMap = {0.4265: 0.05}
            // 2v2 for now
            self.teamSizes = [2]
        }

        pub fun getCurrentEntryFees(): [UFix64] {
            return self.currentEntryFees
        }

        pub fun getEntryFeeToFeePercentageMap(): {UFix64: UFix64} {
            return self.entryFeeToFeePercentageMap
        }

        pub fun getTeamSizes(): [UInt8] {
            return self.teamSizes
        }

        pub fun addEntryFee(newEntryFee: UFix64, feePercentage: UFix64) {
            pre {
                !self.currentEntryFees.contains(newEntryFee) : "Entry fee already exists"
                !self.entryFeeToFeePercentageMap.containsKey(newEntryFee) : "Entry fee already exists in fee % map"
                feePercentage < 1.0 : "Fee percentage too high"
            }

            self.currentEntryFees.append(newEntryFee)
            self.entryFeeToFeePercentageMap.insert(key: newEntryFee, feePercentage)
        }

        pub fun removeEntryFee(entryFee: UFix64) {
            pre {
                self.currentEntryFees.contains(entryFee)
                self.entryFeeToFeePercentageMap.containsKey(entryFee)
                self.currentEntryFees.length > 1 : "Must be at least 1 entry fee"
            }

            // Find the entry fee in currentEntryFees and remove it
            var i = 0
            for fee in self.currentEntryFees {
                if fee == entryFee {
                    self.currentEntryFees.remove(at: i)
                    return
                }
            }

            self.entryFeeToFeePercentageMap.remove(key: entryFee)
        }

        pub fun addTeamSize(newTeamSize: UInt8) {
            pre {
                !self.teamSizes.contains(newTeamSize) : "Team size already exists"
            }

            self.teamSizes.append(newTeamSize)
        }

        pub fun removeTeamSize(teamSize: UInt8) {
            pre {
                self.teamSizes.length > 1 : "Must be at least 1 team size"
            }
            // Find the teamSize in teamSizes and remove it
            var i = 0
            for size in self.teamSizes {
                if size == teamSize {
                    self.teamSizes.remove(at: i)
                    return
                }
            }
        }
    }

    access(contract) fun addActiveBaller(ballerId: UInt64) {
        self.activeBallerIdz.insert(key: ballerId, true)
    }

    access(contract) fun removeActiveBaller(ballerId: UInt64) {
        self.activeBallerIdz.remove(key: ballerId)
    }

    pub fun getCurrentEntryFees(): [UFix64] {
        let configAdmin = self.account.getCapability<&{BallerzSimzConfigAdmin}>(
            self.BallerzSimzConfigAdminPath
        )
        let configAdminRef = configAdmin.borrow()!
        return configAdminRef.getCurrentEntryFees()
    }

    pub fun getEntryFeeToFeePercentageMap(): {UFix64: UFix64} {
        let configAdmin = self.account.getCapability<&{BallerzSimzConfigAdmin}>(
            self.BallerzSimzConfigAdminPath
        )
        let configAdminRef = configAdmin.borrow()!
        return configAdminRef.getEntryFeeToFeePercentageMap()
    }

    pub fun getTeamSizes(): [UInt8] {
        let configAdmin = self.account.getCapability<&{BallerzSimzConfigAdmin}>(
            self.BallerzSimzConfigAdminPath
        )
        let configAdminRef = configAdmin.borrow()!
        return configAdminRef.getTeamSizes()
    }

    init() {
        self.latestSimId = 0
        self.latestTeamId = 0
        self.activeBallerIdz = {}

        self.BallerzSimzResourcePublicPath = /public/BallerzSimz001

        self.BallerzSimzResourceAdminPath = /private/BallerzSimzAdmin001
        self.BallerzSimzConfigAdminPath = /private/BallerzSimzConfig001

        self.BallerzSimzResourceStoragePath = /storage/BallerzSimz001
        self.BallerzSimzConfigStoragePath = /storage/BallerzSimzConfig001

        self.BallerzCollectionPublicPath = /public/GaiaCollection001
        self.FlowTokenReceiverPublicPath = /public/flowTokenReceiver

        let feeOwnerCapability = self.account.getCapability<&{FungibleToken.Receiver}>(
            /public/flowTokenReceiver
        )

        // Create an empty vault and save a receiver and provider
        self.account.save(<- FlowToken.createEmptyVault(), to: /storage/BallerzOwnerEscrowVault)
        self.account.link<&{FungibleToken.Receiver}>(
            /public/BallerzOwnerEscrowVaultRecevier,
            target: /storage/BallerzOwnerEscrowVault
        )
        self.account.link<&{FungibleToken.Provider}>(
            /private/BallerzOwnerEscrowVaultProvider,
            target: /storage/BallerzOwnerEscrowVault
        )
        self.account.link<&{FungibleToken.Balance}>(
            /public/BallerzOwnerEscrowVaultBalance,
            target: /storage/BallerzOwnerEscrowVault
        )

        // Create BallerzSimzResource
        let ballerzSimzResource <- create BallerzSimzResource(
            signer: self.account, feeOwnerCapability: feeOwnerCapability
        )

        // Save BallerzSimzResource to storage
        self.account.save<@BallerzSimzResource>(
            <- ballerzSimzResource,
            to: self.BallerzSimzResourceStoragePath
        )

        // Publish admin capability to private
        self.account.link<&{BallerzSimzAdmin}>(
            self.BallerzSimzResourceAdminPath, target: self.BallerzSimzResourceStoragePath
        )
        // Publish public capability to public
        self.account.link<&{BallerzSimzPublic}>(
            self.BallerzSimzResourcePublicPath, target: self.BallerzSimzResourceStoragePath
        )

        // Create BallerzSimzConfig
        let ballerzSimzConfig <- create BallerzSimzConfig()

        // Save BallerzSimzConfig to storage
        self.account.save<@BallerzSimzConfig>(
            <- ballerzSimzConfig,
            to: self.BallerzSimzConfigStoragePath
        )
        // Publish capability to private
        self.account.link<&{BallerzSimzConfigAdmin}>(
            self.BallerzSimzConfigAdminPath, target: self.BallerzSimzConfigStoragePath
        )
    }
}