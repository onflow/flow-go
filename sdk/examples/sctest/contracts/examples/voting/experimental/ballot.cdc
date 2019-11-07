/*
*   WARNING: Don't completely trust that this code is correct.  It is just practice
*
*   In this example, we want to create a simple voting contract where a polling place 
*   issues ballots to addresses. 
*   
*   When the polling place is created, it is initialized with an array of proposals and 
*   every user with a ballot is allowed approve one proposal in the array
*
*   TODO: Add functionality to send a ballot to someone else so they can vote for you.
*         Might use an NFT for this
*
*/

// interface for how a polling place should be implemented
contract interface PollingPlace {
    // each polling place needs to have the concept of a ballot
    resource Ballot {}

    // And a way to cast a ballot that uses up the resource
    pub fun cast(ballot: <-Ballot)

    // other functionality could be required, but ballots could be issued individually 
    // or sent in bulk so we don't want to pigeonhole devs too much
}

// contains all the functionality associated with voting
pub contract SingleVoteBallot: PollingPlace {

    // This is the resource that is issued to users, they modify to include their vote,
    // and then resubmit to have their vote included in the polling
    pub resource Ballot {
        pub let proposals: [String]  // array of proposals
        pub var choice: Int?   // indicates the index of the proposal being voted for

        // creator must supply the array of proposals on init
        init(proposals: [String]) {
            self.proposals = proposals
            self.choice = nil
        }

        // modifies the ballot in storage to indicate which proposal it is voting for
        pub fun vote(proposal: Int) {
            pre {
                proposal <= self.proposals.length
            }
            self.choice = proposal
        }
    }

    pub let ballotIssued: {Address: bool}  //indicates if a user has already been inssued a ballot
    pub let proposals: [String]
    pub let votes: {String: Int}   // the number of votes for each proposal

    // init function, or constructor, for the contract
    init(proposals: [String]) {
        self.proposals = proposals
        self.votes = {}
    }

    // A user moves their ballot to this function where its vote is tallied and the ballot is destroyed
    pub fun cast(ballot: <-Ballot) {
        pre {
            ballot.choice != nil:
                "Ballot must have a choice"
        }
        // optional conditional.  ballot.choice is an optional, so if choice isn't nil, the block of code will executes
        if let choice = ballot.choice {

            let proposal = self.proposals[choice]
            self.votes[proposal] = (self.votes[proposal] ?? 0) + 1
            // ballot has been used, so it is no longer needed
            destroy ballot
        }
    }

    // Any account can call this function to be issued a ballot
    // can only be called once per account
    pub fun issueBallot(): <-Ballot {
        pre {
            !self.ballotIssued[MSG.SENDER]  //TODO: Need to figure out how to get the caller's address
                "caller has already been issued a ballot!"
        }
        self.ballotIssued[MSG.SENDER] == true
        return <-create Ballot(proposals: self.proposals)
    }

    // Combines the issuing, voting, and casting a ballot all into one function call
    pub fun issueAndCast(proposal: Int) {
        let ballot: <- self.issueBallot()

        // record vote for the proposal that the caller submitted
        ballot.vote(proposals[Int])
        
        // cast the whole ballot
        self.cast(<-ballot)
    }
}
