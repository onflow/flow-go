/* 
*
*  Manages the process of generating a group key with the participation of all the consensus nodes
*  for the upcoming epoch.
*
*  When consensus nodes are first confirmed, they can request a Participant object from this contract
*  They'll use this object for every subsequent epoch that they are a staked consensus node.
*
*  At the beginning of each EpochSetup phase, the admin initializes this contract with
*  the list of consensus nodes for the upcoming epoch. Each consensus node
*  can post as many messages as they want to the DKG "whiteboard" with the `Participant.postMessage()` method,
*  but each node can only submit a final submission once per epoch via the `Participant.sendFinalSubmission() method.
*  
*  Once a >50% threshold of consensus nodes have submitted the exact same set of keys,
*  the DKG phase is technically finished.
*  Anyone can query the state of the submissions with the FlowDKG.getFinalSubmissions()
*  or FlowDKG.dkgCompleted() methods.
*  Consensus nodes can continue to submit final messages even after the required amount have been submitted though.
* 
*  This contract is a member of a series of epoch smart contracts which coordinates the 
*  process of transitioning between epochs in Flow.
*/

pub contract FlowDKG {

    // ===================================================================
    // DKG EVENTS
    // ===================================================================

    /// Emitted when the admin enables the DKG
    pub event StartDKG()

    /// Emitted when the admin ends the DKG after enough submissions have been recorded
    pub event EndDKG(finalSubmission: [String?]?)

    /// Emitted when a consensus node has posted a message to the DKG whiteboard
    pub event BroadcastMessage(nodeID: String, content: String)

    // ================================================================================
    // CONTRACT VARIABLES
    // ================================================================================

    /// The length of keys that have to be submitted as a final submission
    pub let submissionKeyLength: Int

    /// Indicates if the DKG is enabled or not
    pub var dkgEnabled: Bool

    /// Indicates if a Participant resource has already been claimed by a node ID
    /// from the identity table contract
    /// Node IDs have to claim a participant once
    /// one node will use the same specific ID and Participant resource for all time
    /// `nil` or false means that there is no voting capability for the node ID
    /// true means that the participant capability has been claimed by the node
    access(account) var nodeClaimed: {String: Bool}

    /// Record of whiteboard messages for the current epoch
    /// This is reset at the beginning of every DKG phase (once per epoch)
    access(account) var whiteboardMessages: [Message]

    /// Tracks a node's final submission for the current epoch
    /// Key: node ID
    /// Value: Set of public keys from the final submission
    /// If the value is `nil`, the node is not registered as a consensus node
    /// If the value is an empty array, the node has not submitted yet
    /// This mapping is reset at the beginning of every DKG phase (once per epoch)
    access(account) var finalSubmissionByNodeID: {String: [String?]}

    /// Array of unique final submissions from nodes
    /// if a final submission is sent that matches one that already has been submitted
    /// this array will not change at all
    access(account) var uniqueFinalSubmissions: [[String?]]

    /// Tracks how many submissions have been sent
    /// for each unique final submission
    access(account) var uniqueFinalSubmissionCount: {Int: UInt64}

    // ================================================================================
    // CONTRACT CONSTANTS
    // ================================================================================

    // Canonical paths for admin and participant resources
    pub let AdminStoragePath: StoragePath
    pub let ParticipantStoragePath: StoragePath
    pub let ParticipantPublicPath: PublicPath

    /// Struct to represent a single whiteboard message
    pub struct Message {

        /// The ID of the node who submitted the message
        pub let nodeID: String

        /// The content of the message
        /// We make no assumptions or assertions about the content of the message
        pub let content: String

        init(nodeID: String, content: String) {
            self.nodeID = nodeID
            self.content = content
        }
    }

    /// The Participant resource is generated for each consensus node when they register.
    /// Each resource instance is good for all future potential epochs, but will
    /// only be valid if the node operator has been confirmed as a consensus node for the next epoch.
    pub resource Participant {

        /// The node ID of the participant
        pub let nodeID: String

        init(nodeID: String) {
            pre {
                FlowDKG.participantIsClaimed(nodeID) == nil:
                    "Cannot create a Participant resource for a node ID that has already been claimed"
            }
            self.nodeID = nodeID
            FlowDKG.nodeClaimed[nodeID] = true
        }

        /// If the Participant resource is destroyed,
        /// It could potentially be claimed again
        destroy () {
            FlowDKG.nodeClaimed[self.nodeID] = false
        }

        /// Posts a whiteboard message to the contract
        pub fun postMessage(_ content: String) {
            pre {
                FlowDKG.participantIsRegistered(self.nodeID):
                    "Cannot send whiteboard message if not registered for the current epoch"
                content.length > 0:
                    "Cannot post an empty message to the whiteboard"
            }

            // create the message struct
            let message = Message(nodeID: self.nodeID, content: content)

            // add the message to the message record
            FlowDKG.whiteboardMessages.append(message)

            emit BroadcastMessage(nodeID: self.nodeID, content: content)

        }

        /// Sends the final key vector submission. 
        /// Can only be called by consensus nodes that are registered
        /// and can only be called once per consensus node per epoch
        pub fun sendFinalSubmission(_ submission: [String?]) {
            pre {
                FlowDKG.participantIsRegistered(self.nodeID):
                    "Cannot send final submission if not registered for the current epoch"
                !FlowDKG.nodeHasSubmitted(self.nodeID):
                    "Cannot submit a final submission twice"
                submission.length == FlowDKG.getConsensusNodeIDs().length + 1:
                    "Submission must have number of elements equal to the number of nodes participating in the DKG plus 1"
            }

            // iterate through each key in the vector
            // and make sure all of them are the correct length
            for key in submission {
                // nil keys are a valid submission
                if let keyValue = key {
                    // If a key length is incorrect, it is an invalid submission
                    if keyValue.length != FlowDKG.submissionKeyLength {
                        panic("Submission key length is not correct!")
                    }
                }
            }

            var finalSubmissionIndex = 0

            // iterate through all the existing unique submissions
            // If this participant's submission matches one of the existing ones,
            // add to the counter for that submission
            // Otherwise, track the new submission and set its counter to 1
            while finalSubmissionIndex <= FlowDKG.uniqueFinalSubmissions.length {
                // If no matches were found, add this submission as a new unique one
                // and emit an event
                if finalSubmissionIndex == FlowDKG.uniqueFinalSubmissions.length {
                    FlowDKG.uniqueFinalSubmissionCount[finalSubmissionIndex] = 1
                    FlowDKG.uniqueFinalSubmissions.append(submission)
                    break
                }

                let existingSubmission = FlowDKG.uniqueFinalSubmissions[finalSubmissionIndex]

                // If the submissions are equal,
                // update the counter for this submission and emit the event
                if FlowDKG.submissionsEqual(existingSubmission, submission) {
                    FlowDKG.uniqueFinalSubmissionCount[finalSubmissionIndex] = FlowDKG.uniqueFinalSubmissionCount[finalSubmissionIndex]! + (1 as UInt64)
                    break
                }

                // update the index counter
                finalSubmissionIndex = finalSubmissionIndex + 1
            }

            FlowDKG.finalSubmissionByNodeID[self.nodeID] = submission
        }
    }

    /// The Admin resource provides the ability to begin and end voting for an epoch
    pub resource Admin {

        /// Sets the optional safe DKG success threshold
        /// Set the threshold to nil if it isn't needed
        pub fun setSafeSuccessThreshold(newThresholdPercentage: UFix64?) {
            pre {
                !FlowDKG.dkgEnabled: "Cannot set the dkg success threshold while the DKG is enabled"
                newThresholdPercentage == nil ||  newThresholdPercentage! < 1.0: "The threshold percentage must be in [0,1)"
            }

            FlowDKG.account.load<UFix64>(from: /storage/flowDKGSafeThreshold)

            // If newThresholdPercentage is nil, we exit here. Since we loaded from
            // storage previously, this results in /storage/flowDKGSafeThreshold being empty
            if let percentage = newThresholdPercentage {
                FlowDKG.account.save<UFix64>(percentage, to: /storage/flowDKGSafeThreshold)
            }
        }

        /// Creates a new Participant resource for a consensus node
        pub fun createParticipant(nodeID: String): @Participant {
            let participant <-create Participant(nodeID: nodeID)
            FlowDKG.nodeClaimed[nodeID] = true
            return <-participant
        }

        /// Resets all the fields for tracking the current DKG process
        /// and sets the given node IDs as registered
        pub fun startDKG(nodeIDs: [String]) {
            pre {
                FlowDKG.dkgEnabled == false: "Cannot start the DKG when it is already running"
            }

            FlowDKG.finalSubmissionByNodeID = {}
            for id in nodeIDs {
                FlowDKG.finalSubmissionByNodeID[id] = []
            }

            // Clear all of the contract fields
            FlowDKG.whiteboardMessages = []
            FlowDKG.uniqueFinalSubmissions = []
            FlowDKG.uniqueFinalSubmissionCount = {}

            FlowDKG.dkgEnabled = true

            emit StartDKG()
        }

        /// Disables the DKG and closes the opportunity for messages and submissions
        /// until the next time the DKG is enabled
        pub fun endDKG() {
            pre { 
                FlowDKG.dkgEnabled == true: "Cannot end the DKG when it is already disabled"
                FlowDKG.dkgCompleted() != nil: "Cannot end the DKG until enough final arrays have been submitted"
            }

            FlowDKG.dkgEnabled = false

            emit EndDKG(finalSubmission: FlowDKG.dkgCompleted())
        }

        /// Ends the DKG without checking if it is completed
        /// Should only be used if something goes wrong with the DKG,
        /// the protocol halts, or needs to be reset for some reason
        pub fun forceEndDKG() {
            FlowDKG.dkgEnabled = false

            emit EndDKG(finalSubmission: FlowDKG.dkgCompleted())
        }
    }

    /// Checks if two final submissions are equal by comparing each element
    /// Each element has to be exactly the same and in the same order
    access(account) fun submissionsEqual(_ existingSubmission: [String?], _ submission: [String?]): Bool {

        // If the submission length is different than the one being compared to, it is not equal
        if submission.length != existingSubmission.length {
            return false
        }

        var index = 0

        // Check each key in the submiission to make sure that it matches
        // the existing one
        for key in submission {

            // if a key is different, stop checking this submission
            // and return false
            if key != existingSubmission[index] {
                return false
            }

            index = index + 1
        }

        return true
    }

    /// Returns true if a node is registered as a consensus node for the proposed epoch
    pub fun participantIsRegistered(_ nodeID: String): Bool {
        return FlowDKG.finalSubmissionByNodeID[nodeID] != nil
    }

    /// Returns true if a consensus node has claimed their Participant resource
    /// which is valid for all future epochs where the node is registered
    pub fun participantIsClaimed(_ nodeID: String): Bool? {
        return FlowDKG.nodeClaimed[nodeID]
    }

    /// Gets an array of all the whiteboard messages
    /// that have been submitted by all nodes in the DKG
    pub fun getWhiteBoardMessages(): [Message] {
        return self.whiteboardMessages
    }

    /// Returns whether this node has successfully submitted a final submission for this epoch.
    pub fun nodeHasSubmitted(_ nodeID: String): Bool {
        if let submission = self.finalSubmissionByNodeID[nodeID] {
            return submission.length > 0
        } else {
            return false
        }
    }

    /// Gets the specific final submission for a node ID
    /// If the node hasn't submitted or registered, this returns `nil`
    pub fun getNodeFinalSubmission(_ nodeID: String): [String?]? {
        if let submission = self.finalSubmissionByNodeID[nodeID] {
            if submission.length > 0 {
                return submission
            } else {
                return nil
            }
        } else {
            return nil
        }
    }

    /// Get the list of all the consensus node IDs participating
    pub fun getConsensusNodeIDs(): [String] {
        return self.finalSubmissionByNodeID.keys
    }

    /// Get the array of all the unique final submissions
    pub fun getFinalSubmissions(): [[String?]] {
        return self.uniqueFinalSubmissions
    }

    /// Gets the native threshold that the submission count needs to exceed to be considered complete [t=floor((n-1)/2)]
    /// This function returns the NON-INCLUSIVE lower bound of honest participants.
    /// For the DKG to succeed, the number of honest participants must EXCEED this threshold value.
    /// 
    /// Example:
    /// We have 10 DKG nodes (n=10)
    /// The threshold value is t=floor(10-1)/2) (t=4)
    /// There must be AT LEAST 5 honest nodes for the DKG to succeed
    pub fun getNativeSuccessThreshold(): UInt64 {
        return UInt64((self.getConsensusNodeIDs().length-1)/2)
    }

    /// Gets the safe threshold that the submission count needs to exceed to be considered complete.
    /// (always greater than or equal to the native success threshold)
    /// 
    /// This function returns the NON-INCLUSIVE lower bound of honest participants. If this function 
    /// returns threshold t, there must be AT LEAST t+1 honest nodes for the DKG to succeed.
    pub fun getSafeSuccessThreshold(): UInt64 {
        var threshold = self.getNativeSuccessThreshold()

        // Get the safety rate percentage
        if let safetyRate = self.getSafeThresholdPercentage() {

            let safeThreshold = UInt64(safetyRate * UFix64(self.getConsensusNodeIDs().length))

            if safeThreshold > threshold {
                threshold = safeThreshold
            }
        }

        return threshold
    }

    /// Gets the safe threshold percentage. This value must be either nil (semantically: 0) or in [0, 1.0)
    /// This safe threshold is used to artificially increase the DKG participation requirements to 
    /// ensure a lower-bound number of Random Beacon Committee members (beyond the bare minimum required
    /// by the DKG protocol).
    pub fun getSafeThresholdPercentage(): UFix64? {
        let safetyRate = self.account.copy<UFix64>(from: /storage/flowDKGSafeThreshold)
        return safetyRate
    }

    /// Returns the final set of keys if any one set of keys has strictly more than (nodes-1)/2 submissions
    /// Returns nil if not found (incomplete)
    pub fun dkgCompleted(): [String?]? {
        if !self.dkgEnabled { return nil }

        var index = 0

        for submission in self.uniqueFinalSubmissions {
            if self.uniqueFinalSubmissionCount[index]! > self.getSafeSuccessThreshold() {
                var foundNil: Bool = false
                for key in submission {
                    if key == nil {
                        foundNil = true
                        break
                    }
                }
                if foundNil { continue }
                return submission
            }
            index = index + 1
        }

        return nil
    }

    init() {
        self.submissionKeyLength = 192

        self.AdminStoragePath = /storage/flowEpochsDKGAdmin
        self.ParticipantStoragePath = /storage/flowEpochsDKGParticipant
        self.ParticipantPublicPath = /public/flowEpochsDKGParticipant

        self.dkgEnabled = false

        self.finalSubmissionByNodeID = {}
        self.uniqueFinalSubmissionCount = {}
        self.uniqueFinalSubmissions = []
        
        self.nodeClaimed = {}
        self.whiteboardMessages = []

        self.account.save(<-create Admin(), to: self.AdminStoragePath)
    }
}
