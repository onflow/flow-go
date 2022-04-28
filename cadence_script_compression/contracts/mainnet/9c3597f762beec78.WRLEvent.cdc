 /*
 * Copyright (C) Wayports, Inc.
 *
 * SPDX-License-Identifier: (MIT)
 */

// WRLEvent
// The Wayports Racing League Events contract will keep track of
// all the races in the WRL. All events will be registered in the Flow blockchain
// in order to increase transparency for the users.
// The Administrador will register a new event on chain and will give persion to
// another account to insert the result of that event.
// Once the results are updated, a set of Stewards will analyze the race and judge
// if any penalty should be applied to one or more participants due to race incidents.
// After the analysis, the Stewards will be able to validate the Event.
//
pub contract WRLEvent {

    // Interfaces

    // Validable interface is implemented by the Event resource, with it's methods
    // being used latter on by the Stewards to validate and penalize race participants
    //
    pub resource interface Validable {
        pub fun validate()

        pub fun addPenalty(participant: Address, time: UInt64)
    }

    // ResultSetter
    // Describe a function that should be exposed to an arbitrary account
    // by the Administrator resource, so this account can push the results to the chain
    //
    pub resource interface ResultSetter {
        pub fun setResults(stands: {Address:UInt64})
    }

    // GetEventInfo
    // Implemented by the Administrator resource
    // in order to allow publicly accessible information about an event
    //
    pub resource interface GetEventInfo {
        pub fun getEventInfo(): EventInfo
    }

    // ValidatorReceiver
    // Implemented by the Steward resource, allows the holder of a Steward resource
    // to receive an Event Validator from the Administrator resource
    //
    pub resource interface ValidatorReceiver {
        pub fun receiveValidator(cap: Capability<&WRLEvent.Event{WRLEvent.Validable}>)
    }

    // EventViewerReceiver
    // Implemented by Steward resource, this interface exposes methods to display
    // information about the event being validated
    //
    pub resource interface EventViewerReceiver {
        pub fun receiveEventViewer(cap: Capability<&WRLEvent.Event{WRLEvent.GetEventInfo}>)

        pub fun getEventInfo(): EventInfo
    }

    // ResultSetterReceiver
    // Implemented by the Oracle resource, exposes publicly the method that will allow the Administrator
    // resource to deposit a capability to Oracle responsible for set the event results
    //
    pub resource interface ResultSetterReceiver {
        pub fun receiveResultSetter(cap: Capability<&WRLEvent.Event{WRLEvent.ResultSetter}>)
    }

    // EventInfo
    // This struct is used to return information about an Event
    //
    pub struct EventInfo {
        pub let name: String
        pub let baseReward: UFix64
        pub let rewards: [UFix64; 3]
        pub let participants: [Address; 3]

        pub var finished: Bool
        pub var validations: Int
        pub var  resultsUpdated: Bool
        pub var finalStands: {Address:UInt64}
        pub var penalties: {Address:UInt64}

        init(
            _ name: String
            _ baseReward: UFix64
            _ rewards: [UFix64; 3]
            _ participants: [Address; 3]
            _ finished: Bool
            _ validations: Int
            _ resultsUpdated: Bool
            _ finalStands: {Address:UInt64}
            _ penalties: {Address:UInt64}
        ) {
            self.name = name
            self.participants = participants
            self.rewards = rewards
            self.baseReward = baseReward
            self.finished = finished
            self.resultsUpdated = resultsUpdated
            self.validations = validations
            self.finalStands = finalStands;
            self.penalties = penalties;
        }
    }

    // Event
    // This resource holds all the information about a given Event on the Wayport Racing League
    //
    pub resource Event: Validable, ResultSetter, GetEventInfo {
        // Name of the event
        pub let name: String
        // The base reward in Lilium that all drivers will receive
        // at the end of the race
        pub let baseReward: UFix64
        // An in-order array with the amount of Lilium that each driver
        // will receive at the end of the race according to the final stands
        pub let rewards: [UFix64; 3]
        // A list with all the participants addresses
        pub let participants: [Address; 3]

        // A flag that indicates if the event is finished
        pub var finished: Bool
        // A flag that indicates if the Oracle has updated the results
        pub var  resultsUpdated: Bool
        // A counter that indicates how many Steward had validated the event
        pub var validations: Int
        // A dictionary composed by the participant address and the amount of time
        // that he/she took to complet the event
        pub var finalStands: {Address:UInt64}
        // A dictionary containing all the penalties that were applied in the event by Stewards
        pub var penalties: {Address:UInt64}

        init(
            name: String,
            participants: [Address; 3],
            rewards: [UFix64; 3],
            baseReward: UFix64,
        ) {
            self.name = name
            self.participants = participants
            self.rewards = rewards
            self.baseReward = baseReward
            self.finished = false
            self.resultsUpdated = false
            self.validations = 0
            self.finalStands = {};
            self.penalties = {};
        }

        // setResults
        // This function updated the race stands, not allowing the update to happen
        // if it's already been updated or if the race is not finished yet
        //
        pub fun setResults(stands: {Address:UInt64}) {
            pre {
                self.finished: "Race is not finished"
                !self.resultsUpdated: "Results were alredy updated"
            }

            self.finalStands = stands;
            self.resultsUpdated = true;
        }

        // addPenalty
        // Adds a time penalty to a given participant. The penalty is applied to the finalStands dictionary
        // and also to the penalties dictionary in order to keep track of all the penalties applied on a given event
        //
        pub fun addPenalty(participant: Address, time: UInt64) {
            pre {
                // The address must be among the address of the final stands
                self.finalStands.containsKey(participant): "The address was not registered in the event"
                // Only one penalty per event
                !self.penalties.containsKey(participant): "The participant already received a penalty in this event"
            }

            let participantTime = self.finalStands[participant]!

            self.finalStands[participant] = participantTime + time
            self.penalties.insert(key: participant, time)
        }

        // validate
        // Increase the validation counter by 1 unit
        //
        pub fun validate() {
            pre {
              self.resultsUpdated: "Results were not updated"
            }

            self.validations = self.validations + 1
        }

        // end
        // Sets the finished flag to true indicating that the event is over
        //
        pub fun end() {
            pre {
                !self.finished: "Race is already finished"
            }

            self.finished = true
        }

        // sortByTime
        // Returns an array of addresses sorted by finishing time of all participants
        //
        pub fun sortByTime(): [Address] {
            pre {
                self.resultsUpdated: "Results were not updated"
            }

            let rewardOrder: [Address] = []

            var i = 0
            for participant in self.finalStands.keys {
                let currentParticipantTime = self.finalStands[participant]!

                var j = 0
                while(j < rewardOrder.length) {
                    let participantTime = self.finalStands[rewardOrder[j]]!

                    if currentParticipantTime < participantTime {
                        break
                    }

                    j = j + 1
                }

                rewardOrder.insert(at: j, participant)
            }

            return rewardOrder;
        }

        // getEventInfo
        // Returns all fields of the Event
        //
        pub fun getEventInfo(): EventInfo {
            return EventInfo(
                self.name,
                self.baseReward,
                self.rewards,
                self.participants,
                self.finished,
                self.validations,
                self.resultsUpdated,
                self.finalStands,
                self.penalties,
          )
        }
    }

    // EventViewer
    // This resource allows to the UI to easily query the current event being
    // analyzed by a Steward
    //
    pub resource EventViewer: EventViewerReceiver {
        // A capability that exposes the getEventInfo, that will return the info about an Event
        //
        pub var eventInfoCapability: Capability<&WRLEvent.Event{WRLEvent.GetEventInfo}>?

        init() {
            self.eventInfoCapability = nil
        }

        // receiveEventViewer
        // Receives the capability that will be used to return the event info
        //
        pub fun receiveEventViewer(cap: Capability<&WRLEvent.Event{WRLEvent.GetEventInfo}>) {
            pre {
                cap.borrow() != nil: "Invalid Event Info Capability"
            }

            self.eventInfoCapability = cap;
        }

        // getEventInfo
        // Uses the received capability to return the information about an Event
        //
        pub fun getEventInfo(): EventInfo {
            pre {
                self.eventInfoCapability != nil: "No event info capability"
            }

            let eventRef = self.eventInfoCapability!.borrow()!

            return eventRef.getEventInfo()
        }
    }

    // Steward
    // The Steward resource interacts with some functions in the Event resource
    // to update information about penalties and validate the results updated by the Oracle
    // 
    pub resource Steward: ValidatorReceiver {
        // The capability that allows the interaction with a given Event
        //
        pub var validateEventCapability: Capability<&WRLEvent.Event{WRLEvent.Validable}>?

        init() {
            self.validateEventCapability = nil;
        }

        // receiveValidator
        // Receives and updates the validateEventCapability
        //
        pub fun receiveValidator(cap: Capability<&WRLEvent.Event{WRLEvent.Validable}>) {
            pre {
                cap.borrow() != nil: "Invalid Validator capability";
            }

            self.validateEventCapability = cap;
        }

        // validateEvent
        // Uses the received capability to validate the Event by increasing the validations counter
        // 
        pub fun validateEvent() {
            pre {
                self.validateEventCapability != nil: "No validator capability"
            }

            let validatorRef = self.validateEventCapability!.borrow()!

            validatorRef.validate();
        }

        // addPenalty
        // Takes a participant address and an amount of time to be added to the finishing time
        // of that participant, in order to penalize for any incidents that took place in the Event
        //
        pub fun addPenalty(participant: Address, time: UInt64) {
            pre {
                self.validateEventCapability != nil: "No validator capability"
            }

            let validatorRef = self.validateEventCapability!.borrow()!

            validatorRef.addPenalty(participant: participant, time: time);
        }
    }

    // Oracle
    // The Oracle resource will belong to a offchain trusted account that will
    // have access to the final race results for a given Event and will be resposible
    // update the finalStands of the Event
    //
    pub resource Oracle: ResultSetterReceiver {
        // The capability that will allow the interaction with the setResults function from the Event resource
        pub var resultSetter: Capability<&WRLEvent.Event{WRLEvent.ResultSetter}>?

        init() {
            self.resultSetter = nil
        }

        // receiveResultSetter
        // Receives and stores the capability that allows interaction with Event resource
        pub fun receiveResultSetter(cap: Capability<&WRLEvent.Event{WRLEvent.ResultSetter}>) {
            pre {
                cap.borrow() != nil: "Invalid Validator capability";
            }

            self.resultSetter = cap;
        }


        // setResults
        // Receives a dictionary containing the participant address and the time that participant
        // took to finish the race as the value and sets it as the Event finalStands
        pub fun setResults(results: {Address: UInt64}) {
            pre {
                self.resultSetter != nil: "No capability"
            }

            let resultSetterRef = self.resultSetter!.borrow()!

            resultSetterRef.setResults(stands: results);
        }
    }

    // Administrator
    // The Administrator resource is the only resource able to create new
    // event resources, therefore the only one able to delegate Validators and ResultSetters
    pub resource Administrator {
        pub fun createEvent(
            eventName: String,
            participants: [Address; 3],
            rewards: [UFix64; 3],
            baseReward: UFix64
        ): @Event {
            return <- create Event(
                name: eventName,
                participants: participants,
                rewards: rewards,
                baseReward: baseReward
            )
        }
    }

    // createSteward
    // Creates a new instance of Steward resource returns it
    //
    pub fun createSteward(): @Steward {
        return <- create Steward()
    }

    // createEventViewer
    // Creates a new instance of EventViewer resource and returns it

    //
    pub fun createEventViewer(): @EventViewer {
        return <- create EventViewer()
    }

    // createOracle
    // Creates a new instance of Oracle resource and returns it
    //
    pub fun createOracle(): @Oracle {
        return <- create Oracle()
    }

    init() {
        let adminAccount = self.account;

        let admin <- create Administrator();

        adminAccount.save(<-admin, to: /storage/admin);
    }
}
